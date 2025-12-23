package streams

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// DuplicateMatch represents a potential duplicate stream
type DuplicateMatch struct {
	MediaID1       int
	MediaID2       int
	Hash1          string
	Hash2          string
	Title1         string
	Title2         string
	Similarity     float64 // 0.0 to 1.0
	MatchType      string  // "hash", "exact_title", "fuzzy_title", "tmdb_id"
	QualityScore1  int
	QualityScore2  int
	BetterMediaID  int // Which one to keep
}

// DuplicateDetector finds duplicate streams using fuzzy matching
type DuplicateDetector struct {
	db *sql.DB
}

// NewDuplicateDetector creates a new duplicate detector
func NewDuplicateDetector(db *sql.DB) *DuplicateDetector {
	return &DuplicateDetector{db: db}
}

// FindDuplicates finds all duplicate streams in the library
func (d *DuplicateDetector) FindDuplicates(ctx context.Context, similarityThreshold float64) ([]DuplicateMatch, error) {
	if similarityThreshold < 0.0 || similarityThreshold > 1.0 {
		similarityThreshold = 0.85 // Default: 85% similarity
	}
	
	var duplicates []DuplicateMatch
	
	// Method 1: Exact hash matches (fastest, most reliable)
	hashDupes, err := d.findHashDuplicates(ctx)
	if err != nil {
		return nil, fmt.Errorf("hash duplicate detection failed: %w", err)
	}
	duplicates = append(duplicates, hashDupes...)
	
	// Method 2: Fuzzy title matching (slower, catches near-duplicates)
	titleDupes, err := d.findTitleDuplicates(ctx, similarityThreshold)
	if err != nil {
		return nil, fmt.Errorf("title duplicate detection failed: %w", err)
	}
	duplicates = append(duplicates, titleDupes...)
	
	// Remove duplicates from duplicates list (same pair found multiple ways)
	duplicates = d.deduplicateMatches(duplicates)
	
	return duplicates, nil
}

// findHashDuplicates finds streams with identical hashes
func (d *DuplicateDetector) findHashDuplicates(ctx context.Context) ([]DuplicateMatch, error) {
	query := `
		SELECT 
			ms1.media_id as media_id1,
			ms2.media_id as media_id2,
			ms1.stream_hash as hash,
			ms1.quality_score as score1,
			ms2.quality_score as score2
		FROM media_streams ms1
		JOIN media_streams ms2 ON ms1.stream_hash = ms2.stream_hash 
		WHERE ms1.media_id < ms2.media_id
		  AND ms1.stream_hash IS NOT NULL
		  AND ms1.stream_hash != ''
		ORDER BY ms1.stream_hash
	`
	
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var duplicates []DuplicateMatch
	for rows.Next() {
		var match DuplicateMatch
		err := rows.Scan(
			&match.MediaID1,
			&match.MediaID2,
			&match.Hash1,
			&match.QualityScore1,
			&match.QualityScore2,
		)
		if err != nil {
			return nil, err
		}
		
		match.Hash2 = match.Hash1
		match.Similarity = 1.0
		match.MatchType = "hash"
		
		// Determine which to keep (higher quality score)
		if match.QualityScore1 >= match.QualityScore2 {
			match.BetterMediaID = match.MediaID1
		} else {
			match.BetterMediaID = match.MediaID2
		}
		
		duplicates = append(duplicates, match)
	}
	
	return duplicates, rows.Err()
}

// findTitleDuplicates finds streams with similar titles using fuzzy matching
func (d *DuplicateDetector) findTitleDuplicates(ctx context.Context, threshold float64) ([]DuplicateMatch, error) {
	// Get all streams with their media titles
	query := `
		SELECT 
			ms.media_id,
			ms.stream_hash,
			ms.quality_score,
			COALESCE(m.title, '') as title
		FROM media_streams ms
		JOIN media m ON ms.media_id = m.id
		WHERE ms.is_available = true
		ORDER BY ms.media_id
	`
	
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	type streamInfo struct {
		mediaID      int
		hash         string
		qualityScore int
		title        string
		normalized   string
	}
	
	var streams []streamInfo
	for rows.Next() {
		var s streamInfo
		if err := rows.Scan(&s.mediaID, &s.hash, &s.qualityScore, &s.title); err != nil {
			return nil, err
		}
		s.normalized = normalizeTitle(s.title)
		streams = append(streams, s)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	// Compare all pairs (O(n²) but acceptable for typical library sizes)
	var duplicates []DuplicateMatch
	for i := 0; i < len(streams); i++ {
		for j := i + 1; j < len(streams); j++ {
			s1 := streams[i]
			s2 := streams[j]
			
			// Skip if same hash (already caught by hash detection)
			if s1.hash == s2.hash && s1.hash != "" {
				continue
			}
			
			// Calculate similarity
			similarity := calculateSimilarity(s1.normalized, s2.normalized)
			
			if similarity >= threshold {
				match := DuplicateMatch{
					MediaID1:      s1.mediaID,
					MediaID2:      s2.mediaID,
					Hash1:         s1.hash,
					Hash2:         s2.hash,
					Title1:        s1.title,
					Title2:        s2.title,
					Similarity:    similarity,
					QualityScore1: s1.qualityScore,
					QualityScore2: s2.qualityScore,
				}
				
				if similarity == 1.0 {
					match.MatchType = "exact_title"
				} else {
					match.MatchType = "fuzzy_title"
				}
				
				// Determine which to keep
				if s1.qualityScore >= s2.qualityScore {
					match.BetterMediaID = s1.mediaID
				} else {
					match.BetterMediaID = s2.mediaID
				}
				
				duplicates = append(duplicates, match)
			}
		}
	}
	
	return duplicates, nil
}

// normalizeTitle normalizes a title for comparison
func normalizeTitle(title string) string {
	// Convert to lowercase
	normalized := strings.ToLower(title)
	
	// Remove year (e.g., "(2024)" or "2024")
	yearRegex := regexp.MustCompile(`\s*\(?\d{4}\)?`)
	normalized = yearRegex.ReplaceAllString(normalized, "")
	
	// Remove common words
	commonWords := []string{"the", "a", "an"}
	for _, word := range commonWords {
		normalized = strings.ReplaceAll(normalized, " "+word+" ", " ")
	}
	
	// Remove special characters except spaces
	var builder strings.Builder
	for _, r := range normalized {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' {
			builder.WriteRune(r)
		}
	}
	normalized = builder.String()
	
	// Collapse multiple spaces
	spaceRegex := regexp.MustCompile(`\s+`)
	normalized = spaceRegex.ReplaceAllString(normalized, " ")
	
	// Trim
	normalized = strings.TrimSpace(normalized)
	
	return normalized
}

// calculateSimilarity calculates similarity between two strings (0.0 to 1.0)
// Uses Levenshtein distance normalized by max length
func calculateSimilarity(s1, s2 string) float64 {
	// Exact match
	if s1 == s2 {
		return 1.0
	}
	
	// Empty strings
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}
	
	// Calculate Levenshtein distance
	distance := levenshteinDistance(s1, s2)
	
	// Normalize by max length
	maxLen := max(len(s1), len(s2))
	similarity := 1.0 - (float64(distance) / float64(maxLen))
	
	return similarity
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)
	len1 := len(r1)
	len2 := len(r2)
	
	// Create matrix
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}
	
	// Initialize first row and column
	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}
	
	// Fill matrix
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}
			
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}
	
	return matrix[len1][len2]
}

// deduplicateMatches removes duplicate pairs from match list
func (d *DuplicateDetector) deduplicateMatches(matches []DuplicateMatch) []DuplicateMatch {
	seen := make(map[string]bool)
	var unique []DuplicateMatch
	
	for _, match := range matches {
		// Create key (always smaller ID first)
		id1, id2 := match.MediaID1, match.MediaID2
		if id1 > id2 {
			id1, id2 = id2, id1
		}
		key := fmt.Sprintf("%d-%d", id1, id2)
		
		if !seen[key] {
			seen[key] = true
			unique = append(unique, match)
		}
	}
	
	return unique
}

// GetDuplicateStats returns duplicate statistics
func (d *DuplicateDetector) GetDuplicateStats(ctx context.Context, threshold float64) (map[string]interface{}, error) {
	duplicates, err := d.FindDuplicates(ctx, threshold)
	if err != nil {
		return nil, err
	}
	
	stats := map[string]interface{}{
		"total_duplicates": len(duplicates),
		"by_type":          make(map[string]int),
	}
	
	byType := stats["by_type"].(map[string]int)
	for _, dup := range duplicates {
		byType[dup.MatchType]++
	}
	
	return stats, nil
}

// ResolveDuplicate removes the lower quality stream from a duplicate pair
func (d *DuplicateDetector) ResolveDuplicate(ctx context.Context, match DuplicateMatch) error {
	// Determine which to delete (keep better quality)
	deleteMediaID := match.MediaID1
	if match.BetterMediaID == match.MediaID1 {
		deleteMediaID = match.MediaID2
	}
	
	// Delete the lower quality cached stream
	query := `DELETE FROM media_streams WHERE media_id = $1`
	_, err := d.db.ExecContext(ctx, query, deleteMediaID)
	if err != nil {
		return fmt.Errorf("failed to delete duplicate: %w", err)
	}
	
	return nil
}

// AutoResolveDuplicates automatically resolves all duplicates by keeping best quality
func (d *DuplicateDetector) AutoResolveDuplicates(ctx context.Context, threshold float64, dryRun bool) ([]DuplicateMatch, error) {
	duplicates, err := d.FindDuplicates(ctx, threshold)
	if err != nil {
		return nil, err
	}
	
	if dryRun {
		return duplicates, nil // Just report, don't delete
	}
	
	var resolved []DuplicateMatch
	for _, match := range duplicates {
		if err := d.ResolveDuplicate(ctx, match); err != nil {
			return resolved, fmt.Errorf("failed to resolve duplicate: %w", err)
		}
		resolved = append(resolved, match)
	}
	
	return resolved, nil
}

// FindHashCollisions finds different media items with same hash (true duplicates)
func (d *DuplicateDetector) FindHashCollisions(ctx context.Context) ([]string, error) {
	query := `
		SELECT stream_hash, COUNT(DISTINCT media_id) as media_count
		FROM media_streams
		WHERE stream_hash IS NOT NULL 
		  AND stream_hash != ''
		GROUP BY stream_hash
		HAVING COUNT(DISTINCT media_id) > 1
	`
	
	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var hashes []string
	for rows.Next() {
		var hash string
		var count int
		if err := rows.Scan(&hash, &count); err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	
	return hashes, rows.Err()
}

// Helper functions
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
