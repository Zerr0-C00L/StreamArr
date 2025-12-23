package streams

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// StreamQuality contains parsed quality attributes from a torrent name
type StreamQuality struct {
	Resolution   string
	HDRType      string
	AudioFormat  string
	Source       string
	Codec        string
	SizeGB       float64
	Seeders      int
}

// QualityScore represents the calculated score breakdown
type QualityScore struct {
	TotalScore       int
	ResolutionScore  int
	HDRScore         int
	AudioScore       int
	SourceScore      int
	SeedersScore     int
	SizePenalty      int
}

// CalculateScore computes quality score using pure mathematical formula (no AI)
// Formula: Resolution(40) + HDR(15) + Audio(15) + Source(20) + log(seeders)*2 - SizePenalty
// Max theoretical score: 40+15+15+20+~10 = ~100 points
func CalculateScore(quality StreamQuality) QualityScore {
	score := QualityScore{}
	
	// Resolution scoring (40 points max)
	score.ResolutionScore = getResolutionScore(quality.Resolution)
	
	// HDR scoring (15 points max)
	score.HDRScore = getHDRScore(quality.HDRType)
	
	// Audio scoring (15 points max)
	score.AudioScore = getAudioScore(quality.AudioFormat)
	
	// Source scoring (20 points max)
	score.SourceScore = getSourceScore(quality.Source)
	
	// Seeders scoring (log scale, ~10 points realistic max)
	score.SeedersScore = getSeedersScore(quality.Seeders)
	
	// Size penalty (deduct points for bloated files)
	score.SizePenalty = getSizePenalty(quality.SizeGB, quality.Resolution)
	
	// Total = sum of all - penalty
	score.TotalScore = score.ResolutionScore + score.HDRScore + score.AudioScore + 
	                   score.SourceScore + score.SeedersScore - score.SizePenalty
	
	// Floor at 0 (no negative scores)
	if score.TotalScore < 0 {
		score.TotalScore = 0
	}
	
	return score
}

// ParseQualityFromTorrentName extracts quality attributes from torrent name
// Example: "Movie.Name.2024.2160p.DV.HDR10.TrueHD.Atmos.7.1.REMUX-GROUP"
func ParseQualityFromTorrentName(torrentName string) StreamQuality {
	quality := StreamQuality{}
	upperName := strings.ToUpper(torrentName)
	
	// Parse resolution
	quality.Resolution = parseResolution(upperName)
	
	// Parse HDR type
	quality.HDRType = parseHDRType(upperName)
	
	// Parse audio format
	quality.AudioFormat = parseAudioFormat(upperName)
	
	// Parse source
	quality.Source = parseSource(upperName)
	
	// Parse codec
	quality.Codec = parseCodec(upperName)
	
	return quality
}

// getResolutionScore assigns points based on resolution
func getResolutionScore(resolution string) int {
	switch resolution {
	case "2160p", "4K", "UHD":
		return 40
	case "1080p", "FHD":
		return 30
	case "720p", "HD":
		return 15
	case "576p", "480p", "SD":
		return 5
	default:
		return 0
	}
}

// getHDRScore assigns points based on HDR technology
func getHDRScore(hdrType string) int {
	switch hdrType {
	case "DV", "Dolby Vision":
		return 15
	case "HDR10+", "HDR10PLUS":
		return 12
	case "HDR10", "HDR":
		return 10
	case "SDR", "":
		return 0
	default:
		return 0
	}
}

// getAudioScore assigns points based on audio format
func getAudioScore(audioFormat string) int {
	switch audioFormat {
	case "Atmos", "TrueHD Atmos", "TrueHD.Atmos":
		return 15
	case "TrueHD", "DTS-HD MA", "DTS-HD.MA":
		return 12
	case "DTS-HD", "DTS-X":
		return 10
	case "DD+", "EAC3", "E-AC3":
		return 7
	case "AC3", "DD", "DTS":
		return 5
	case "AAC", "MP3":
		return 2
	default:
		return 0
	}
}

// getSourceScore assigns points based on source quality
func getSourceScore(source string) int {
	switch source {
	case "REMUX", "Remux":
		return 20
	case "BluRay", "Blu-ray", "BDRip":
		return 15
	case "WEB-DL", "WEBDL":
		return 12
	case "WEBRip", "WEB":
		return 8
	case "HDTV", "DVDRip":
		return 5
	case "HDCAM", "CAM", "TS", "TC":
		return 1
	default:
		return 0
	}
}

// getSeedersScore converts seeders to logarithmic score
// Formula: log10(seeders) * 2
// Examples: 10 seeders=2pts, 100=4pts, 1000=6pts, 10000=8pts
func getSeedersScore(seeders int) int {
	if seeders <= 0 {
		return 0
	}
	return int(math.Log10(float64(seeders)) * 2)
}

// getSizePenalty penalizes unreasonably large files
// Rules:
// - 4K: >80GB = -5pts, >100GB = -10pts
// - 1080p: >40GB = -5pts, >60GB = -10pts
// - 720p: >20GB = -5pts, >30GB = -10pts
func getSizePenalty(sizeGB float64, resolution string) int {
	if sizeGB == 0 {
		return 0 // Size unknown, no penalty
	}
	
	switch resolution {
	case "2160p", "4K", "UHD":
		if sizeGB > 100 {
			return 10
		} else if sizeGB > 80 {
			return 5
		}
	case "1080p", "FHD":
		if sizeGB > 60 {
			return 10
		} else if sizeGB > 40 {
			return 5
		}
	case "720p", "HD":
		if sizeGB > 30 {
			return 10
		} else if sizeGB > 20 {
			return 5
		}
	}
	
	return 0
}

// parseResolution extracts resolution from torrent name
func parseResolution(upperName string) string {
	if strings.Contains(upperName, "2160P") || strings.Contains(upperName, "4K") || strings.Contains(upperName, "UHD") {
		return "2160p"
	}
	if strings.Contains(upperName, "1080P") {
		return "1080p"
	}
	if strings.Contains(upperName, "720P") {
		return "720p"
	}
	if strings.Contains(upperName, "576P") {
		return "576p"
	}
	if strings.Contains(upperName, "480P") {
		return "480p"
	}
	return "SD"
}

// parseHDRType extracts HDR technology from torrent name
func parseHDRType(upperName string) string {
	// Check for Dolby Vision first (most specific)
	if strings.Contains(upperName, "DV") || strings.Contains(upperName, "DOLBY.VISION") || strings.Contains(upperName, "DOLBYVISION") {
		return "DV"
	}
	// HDR10+ before HDR10
	if strings.Contains(upperName, "HDR10+") || strings.Contains(upperName, "HDR10PLUS") {
		return "HDR10+"
	}
	if strings.Contains(upperName, "HDR10") {
		return "HDR10"
	}
	if strings.Contains(upperName, "HDR") {
		return "HDR"
	}
	return "SDR"
}

// parseAudioFormat extracts audio format from torrent name
func parseAudioFormat(upperName string) string {
	// Check for Atmos first
	if strings.Contains(upperName, "ATMOS") {
		return "Atmos"
	}
	if strings.Contains(upperName, "TRUEHD") {
		return "TrueHD"
	}
	if strings.Contains(upperName, "DTS-HD.MA") || strings.Contains(upperName, "DTS-HD MA") {
		return "DTS-HD MA"
	}
	if strings.Contains(upperName, "DTS-HD") {
		return "DTS-HD"
	}
	if strings.Contains(upperName, "DTS-X") || strings.Contains(upperName, "DTSX") {
		return "DTS-X"
	}
	if strings.Contains(upperName, "DD+") || strings.Contains(upperName, "EAC3") || strings.Contains(upperName, "E-AC3") {
		return "DD+"
	}
	if strings.Contains(upperName, "AC3") || strings.Contains(upperName, "DD") {
		return "AC3"
	}
	if strings.Contains(upperName, "DTS") {
		return "DTS"
	}
	if strings.Contains(upperName, "AAC") {
		return "AAC"
	}
	if strings.Contains(upperName, "MP3") {
		return "MP3"
	}
	return ""
}

// parseSource extracts source type from torrent name
func parseSource(upperName string) string {
	if strings.Contains(upperName, "REMUX") {
		return "REMUX"
	}
	if strings.Contains(upperName, "BLURAY") || strings.Contains(upperName, "BLU-RAY") || strings.Contains(upperName, "BDRIP") {
		return "BluRay"
	}
	if strings.Contains(upperName, "WEB-DL") || strings.Contains(upperName, "WEBDL") {
		return "WEB-DL"
	}
	if strings.Contains(upperName, "WEBRIP") || strings.Contains(upperName, "WEB") {
		return "WEBRip"
	}
	if strings.Contains(upperName, "HDTV") {
		return "HDTV"
	}
	if strings.Contains(upperName, "DVDRIP") {
		return "DVDRip"
	}
	if strings.Contains(upperName, "CAM") || strings.Contains(upperName, "HDCAM") {
		return "CAM"
	}
	if strings.Contains(upperName, "TS") || strings.Contains(upperName, "TELESYNC") {
		return "TS"
	}
	if strings.Contains(upperName, "TC") || strings.Contains(upperName, "TELECINE") {
		return "TC"
	}
	return ""
}

// parseCodec extracts video codec from torrent name
func parseCodec(upperName string) string {
	if strings.Contains(upperName, "H.265") || strings.Contains(upperName, "H265") || strings.Contains(upperName, "HEVC") {
		return "HEVC"
	}
	if strings.Contains(upperName, "H.264") || strings.Contains(upperName, "H264") || strings.Contains(upperName, "AVC") {
		return "AVC"
	}
	if strings.Contains(upperName, "AV1") {
		return "AV1"
	}
	if strings.Contains(upperName, "VP9") {
		return "VP9"
	}
	if strings.Contains(upperName, "XVID") {
		return "XviD"
	}
	return ""
}

// ExtractSizeFromTorrentName attempts to parse file size from torrent name
// Example: "Movie.2024.2160p.50GB.REMUX" -> 50.0
func ExtractSizeFromTorrentName(torrentName string) float64 {
	// Regex to match size patterns like "50GB", "12.5GB", "1.2TB"
	sizeRegex := regexp.MustCompile(`(\d+(?:\.\d+)?)\s?(GB|TB|MB)`)
	matches := sizeRegex.FindStringSubmatch(strings.ToUpper(torrentName))
	
	if len(matches) >= 3 {
		var size float64
		size, _ = parseFloat(matches[1])
		
		unit := matches[2]
		switch unit {
		case "TB":
			return size * 1024
		case "GB":
			return size
		case "MB":
			return size / 1024
		}
	}
	
	return 0 // Size not found
}

// parseFloat parses float from string
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
