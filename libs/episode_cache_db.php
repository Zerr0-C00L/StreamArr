<?php
/**
 * SQLite-based Media Cache (Episodes + Movies)
 * Solves memory issues by using database lookups instead of loading JSON into memory
 */

class EpisodeCacheDB {
    private $db;
    private $dbFile;
    
    public function __construct($dbFile = null) {
        $this->dbFile = $dbFile ?? __DIR__ . '/../cache/episodes.db';
        $this->connect();
    }
    
    private function connect() {
        $isNew = !file_exists($this->dbFile);
        
        $this->db = new SQLite3($this->dbFile);
        $this->db->busyTimeout(5000);
        $this->db->exec('PRAGMA journal_mode = WAL');
        $this->db->exec('PRAGMA synchronous = NORMAL');
        
        if ($isNew) {
            $this->createTables();
        } else {
            // Ensure movies table exists (for upgrades)
            $this->createMoviesTable();
            // Ensure streams table exists (for upgrades)
            $this->createStreamsTable();
        }
    }
    
    private function createTables() {
        // Episodes table - indexed for fast lookups
        $this->db->exec('
            CREATE TABLE IF NOT EXISTS episodes (
                episode_id INTEGER PRIMARY KEY,
                series_id INTEGER NOT NULL,
                season INTEGER NOT NULL,
                episode INTEGER NOT NULL,
                imdb_id TEXT
            )
        ');
        
        // Index on series_id for reverse lookups
        $this->db->exec('CREATE INDEX IF NOT EXISTS idx_series ON episodes(series_id)');
        
        // Series table to track what's been cached
        $this->db->exec('
            CREATE TABLE IF NOT EXISTS series (
                series_id INTEGER PRIMARY KEY,
                name TEXT,
                imdb_id TEXT,
                episode_count INTEGER DEFAULT 0,
                cached_at INTEGER
            )
        ');
        
        // Movies table
        $this->createMoviesTable();
        
        // Metadata table
        $this->db->exec('
            CREATE TABLE IF NOT EXISTS metadata (
                key TEXT PRIMARY KEY,
                value TEXT
            )
        ');
        
        // Available streams table (cached from Torrentio)
        $this->createStreamsTable();
    }
    
    /**
     * Create streams table for caching available Torrentio streams
     */
    private function createStreamsTable() {
        $this->db->exec('
            CREATE TABLE IF NOT EXISTS streams (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                imdb_id TEXT NOT NULL,
                type TEXT NOT NULL,
                season INTEGER,
                episode INTEGER,
                quality TEXT,
                size TEXT,
                title TEXT,
                hash TEXT,
                file_idx INTEGER DEFAULT 0,
                resolve_url TEXT,
                cached_at INTEGER,
                UNIQUE(imdb_id, type, season, episode, hash)
            )
        ');
        $this->db->exec('CREATE INDEX IF NOT EXISTS idx_streams_imdb ON streams(imdb_id, type, season, episode)');
    }
    
    /**
     * Create movies table (for upgrades from older versions)
     */
    private function createMoviesTable() {
        $this->db->exec('
            CREATE TABLE IF NOT EXISTS movies (
                movie_id INTEGER PRIMARY KEY,
                title TEXT,
                imdb_id TEXT,
                year INTEGER,
                cached_at INTEGER
            )
        ');
        $this->db->exec('CREATE INDEX IF NOT EXISTS idx_movies_imdb ON movies(imdb_id)');
    }
    
    // ==================== MOVIE METHODS ====================
    
    /**
     * Get movie by TMDB ID
     * @return array|null Movie info or null if not found
     */
    public function getMovie($movieId) {
        $stmt = $this->db->prepare('SELECT movie_id, title, imdb_id, year, cached_at FROM movies WHERE movie_id = :id');
        $stmt->bindValue(':id', (int)$movieId, SQLITE3_INTEGER);
        $result = $stmt->execute();
        
        if ($row = $result->fetchArray(SQLITE3_ASSOC)) {
            return [
                'movie_id' => (string)$row['movie_id'],
                'title' => $row['title'],
                'imdb_id' => $row['imdb_id'] ?? '',
                'year' => (int)$row['year'],
                'cached_at' => (int)$row['cached_at']
            ];
        }
        
        return null;
    }
    
    /**
     * Add or update a movie
     */
    public function setMovie($movieId, $title = '', $imdbId = '', $year = 0) {
        $stmt = $this->db->prepare('
            INSERT OR REPLACE INTO movies (movie_id, title, imdb_id, year, cached_at)
            VALUES (:id, :title, :imdb_id, :year, :time)
        ');
        $stmt->bindValue(':id', (int)$movieId, SQLITE3_INTEGER);
        $stmt->bindValue(':title', $title, SQLITE3_TEXT);
        $stmt->bindValue(':imdb_id', $imdbId, SQLITE3_TEXT);
        $stmt->bindValue(':year', (int)$year, SQLITE3_INTEGER);
        $stmt->bindValue(':time', time(), SQLITE3_INTEGER);
        return $stmt->execute() !== false;
    }
    
    /**
     * Check if a movie is cached
     */
    public function isMovieCached($movieId) {
        $stmt = $this->db->prepare('SELECT 1 FROM movies WHERE movie_id = :id');
        $stmt->bindValue(':id', (int)$movieId, SQLITE3_INTEGER);
        $result = $stmt->execute();
        return $result->fetchArray() !== false;
    }
    
    /**
     * Bulk insert movies (for imports)
     */
    public function bulkInsertMovies($movies) {
        $this->db->exec('BEGIN TRANSACTION');
        
        $stmt = $this->db->prepare('
            INSERT OR REPLACE INTO movies (movie_id, title, imdb_id, year, cached_at)
            VALUES (:id, :title, :imdb_id, :year, :time)
        ');
        
        $count = 0;
        $time = time();
        foreach ($movies as $id => $data) {
            $stmt->bindValue(':id', (int)$id, SQLITE3_INTEGER);
            $stmt->bindValue(':title', $data['title'] ?? '', SQLITE3_TEXT);
            $stmt->bindValue(':imdb_id', $data['imdb_id'] ?? '', SQLITE3_TEXT);
            $stmt->bindValue(':year', (int)($data['year'] ?? 0), SQLITE3_INTEGER);
            $stmt->bindValue(':time', $time, SQLITE3_INTEGER);
            $stmt->execute();
            $stmt->reset();
            $count++;
        }
        
        $this->db->exec('COMMIT');
        return $count;
    }
    
    /**
     * Get movie count
     */
    public function getMovieCount() {
        return $this->db->querySingle('SELECT COUNT(*) FROM movies');
    }
    
    // ==================== STREAM METHODS ====================
    
    /**
     * Save available streams for a movie/episode
     */
    public function saveStreams($imdbId, $type, $streams, $season = null, $episode = null) {
        // Delete old streams for this content
        $stmt = $this->db->prepare('DELETE FROM streams WHERE imdb_id = :imdb AND type = :type AND season IS :season AND episode IS :episode');
        $stmt->bindValue(':imdb', $imdbId, SQLITE3_TEXT);
        $stmt->bindValue(':type', $type, SQLITE3_TEXT);
        $stmt->bindValue(':season', $season, $season ? SQLITE3_INTEGER : SQLITE3_NULL);
        $stmt->bindValue(':episode', $episode, $episode ? SQLITE3_INTEGER : SQLITE3_NULL);
        $stmt->execute();
        
        // Insert new streams
        $insertStmt = $this->db->prepare('
            INSERT OR REPLACE INTO streams (imdb_id, type, season, episode, quality, size, title, hash, file_idx, resolve_url, cached_at)
            VALUES (:imdb, :type, :season, :episode, :quality, :size, :title, :hash, :file_idx, :resolve_url, :cached_at)
        ');
        
        $count = 0;
        $time = time();
        
        foreach ($streams as $stream) {
            $insertStmt->bindValue(':imdb', $imdbId, SQLITE3_TEXT);
            $insertStmt->bindValue(':type', $type, SQLITE3_TEXT);
            $insertStmt->bindValue(':season', $season, $season ? SQLITE3_INTEGER : SQLITE3_NULL);
            $insertStmt->bindValue(':episode', $episode, $episode ? SQLITE3_INTEGER : SQLITE3_NULL);
            $insertStmt->bindValue(':quality', $stream['quality'] ?? '', SQLITE3_TEXT);
            $insertStmt->bindValue(':size', $stream['size'] ?? '', SQLITE3_TEXT);
            $insertStmt->bindValue(':title', $stream['title'] ?? '', SQLITE3_TEXT);
            $insertStmt->bindValue(':hash', $stream['hash'] ?? '', SQLITE3_TEXT);
            $insertStmt->bindValue(':file_idx', (int)($stream['file_idx'] ?? 0), SQLITE3_INTEGER);
            $insertStmt->bindValue(':resolve_url', $stream['resolve_url'] ?? '', SQLITE3_TEXT);
            $insertStmt->bindValue(':cached_at', $time, SQLITE3_INTEGER);
            $insertStmt->execute();
            $insertStmt->reset();
            $count++;
        }
        
        return $count;
    }
    
    /**
     * Get available streams for a movie/episode
     * Sorted by quality (best first) then by size (largest first)
     */
    public function getStreams($imdbId, $type, $season = null, $episode = null) {
        $sql = 'SELECT * FROM streams WHERE imdb_id = :imdb AND type = :type';
        if ($season !== null) {
            $sql .= ' AND season = :season AND episode = :episode';
        } else {
            $sql .= ' AND season IS NULL';
        }
        $sql .= ' ORDER BY 
            CASE quality 
                WHEN "2160P" THEN 1 
                WHEN "4K" THEN 1 
                WHEN "1080P" THEN 2 
                WHEN "720P" THEN 3 
                WHEN "480P" THEN 4 
                ELSE 5 
            END';
        
        $stmt = $this->db->prepare($sql);
        $stmt->bindValue(':imdb', $imdbId, SQLITE3_TEXT);
        $stmt->bindValue(':type', $type, SQLITE3_TEXT);
        if ($season !== null) {
            $stmt->bindValue(':season', (int)$season, SQLITE3_INTEGER);
            $stmt->bindValue(':episode', (int)$episode, SQLITE3_INTEGER);
        }
        
        $result = $stmt->execute();
        $streams = [];
        
        while ($row = $result->fetchArray(SQLITE3_ASSOC)) {
            // Filter out collection packs and multi-movie torrents
            $title = $row['title'] ?? '';
            if ($this->isCollectionPack($title)) {
                continue;
            }
            
            // Extract size in bytes for sorting - prefer size column, fallback to title
            $sizeText = $row['size'] ?? $title;
            $row['_size_bytes'] = $this->parseSizeToBytes($sizeText);
            $streams[] = $row;
        }
        
        // Sort by quality (already done by SQL) then by size within each quality
        usort($streams, function($a, $b) {
            // Quality order
            $qualityOrder = ['2160P' => 1, '4K' => 1, '1080P' => 2, '720P' => 3, '480P' => 4];
            $qa = $qualityOrder[$a['quality'] ?? ''] ?? 5;
            $qb = $qualityOrder[$b['quality'] ?? ''] ?? 5;
            
            if ($qa !== $qb) {
                return $qa - $qb;
            }
            
            // Within same quality, sort by size (largest first)
            return ($b['_size_bytes'] ?? 0) - ($a['_size_bytes'] ?? 0);
        });
        
        // Limit to 5 streams per quality
        $limitedStreams = [];
        $qualityCounts = [];
        $maxPerQuality = 5;
        
        foreach ($streams as $s) {
            $quality = $s['quality'] ?? 'unknown';
            $qualityCounts[$quality] = ($qualityCounts[$quality] ?? 0) + 1;
            
            if ($qualityCounts[$quality] <= $maxPerQuality) {
                unset($s['_size_bytes']); // Remove internal field
                $limitedStreams[] = $s;
            }
        }
        
        return $limitedStreams;
    }
    
    /**
     * Check if a stream title indicates a collection/pack (multi-movie torrent)
     */
    private function isCollectionPack($title) {
        // Patterns that indicate collection packs
        $packPatterns = [
            '/\d+\s*Movies?\s*(Collection|Pack|Part)/i',  // "500 Movies Collection", "67 Movies Part"
            '/Collection.*\d+.*Movies?/i',                 // "Collection of 250 Movies"
            '/IMDB\s*Top\s*\d+/i',                         // "IMDB Top 250"
            '/Top\s*\d+\s*Movies?/i',                      // "Top 100 Movies"
            '/\bPack\b.*\d+\s*(Movies?|Films?)/i',         // "Pack of 50 Movies"
            '/\d+\s*(Classic|Best|Greatest)\s*(Movies?|Films?)/i',  // "500 Classic Movies"
            '/Fanedit\s*Collection/i',                     // "Fanedit Collection"
            '/Movies?\s*Collection\s*\d+/i',               // "Movies Collection 2024"
        ];
        
        foreach ($packPatterns as $pattern) {
            if (preg_match($pattern, $title)) {
                return true;
            }
        }
        
        return false;
    }
    
    /**
     * Parse size string (e.g., "37.82 GB", "996.71 MB") to bytes
     */
    private function parseSizeToBytes($text) {
        // Match patterns like "💾 37.82 GB" or "37.82 GB"
        if (preg_match('/([\d.]+)\s*(TB|GB|MB|KB)/i', $text, $matches)) {
            $size = floatval($matches[1]);
            $unit = strtoupper($matches[2]);
            
            switch ($unit) {
                case 'TB': return (int)($size * 1024 * 1024 * 1024 * 1024);
                case 'GB': return (int)($size * 1024 * 1024 * 1024);
                case 'MB': return (int)($size * 1024 * 1024);
                case 'KB': return (int)($size * 1024);
            }
        }
        return 0;
    }
    
    /**
     * Check if streams are cached and fresh (within TTL)
     */
    public function hasValidStreams($imdbId, $type, $season = null, $episode = null, $ttlHours = 24) {
        $sql = 'SELECT cached_at FROM streams WHERE imdb_id = :imdb AND type = :type';
        if ($season !== null) {
            $sql .= ' AND season = :season AND episode = :episode';
        } else {
            $sql .= ' AND season IS NULL';
        }
        $sql .= ' LIMIT 1';
        
        $stmt = $this->db->prepare($sql);
        $stmt->bindValue(':imdb', $imdbId, SQLITE3_TEXT);
        $stmt->bindValue(':type', $type, SQLITE3_TEXT);
        if ($season !== null) {
            $stmt->bindValue(':season', (int)$season, SQLITE3_INTEGER);
            $stmt->bindValue(':episode', (int)$episode, SQLITE3_INTEGER);
        }
        
        $result = $stmt->execute();
        if ($row = $result->fetchArray(SQLITE3_ASSOC)) {
            $age = time() - $row['cached_at'];
            return $age < ($ttlHours * 3600);
        }
        
        return false;
    }
    
    /**
     * Get stream count
     */
    public function getStreamCount() {
        return $this->db->querySingle('SELECT COUNT(*) FROM streams');
    }
    
    // ==================== EPISODE METHODS ====================
    
    /**
     * Look up an episode by its TMDB episode ID
     * @return array|null Episode info or null if not found
     */
    public function getEpisode($episodeId) {
        $stmt = $this->db->prepare('SELECT series_id, season, episode, imdb_id FROM episodes WHERE episode_id = :id');
        $stmt->bindValue(':id', (int)$episodeId, SQLITE3_INTEGER);
        $result = $stmt->execute();
        
        if ($row = $result->fetchArray(SQLITE3_ASSOC)) {
            return [
                'series_id' => (string)$row['series_id'],
                'season' => (int)$row['season'],
                'episode' => (int)$row['episode'],
                'imdb_id' => $row['imdb_id'] ?? ''
            ];
        }
        
        return null;
    }
    
    /**
     * Add or update an episode
     */
    public function setEpisode($episodeId, $seriesId, $season, $episode, $imdbId = '') {
        $stmt = $this->db->prepare('
            INSERT OR REPLACE INTO episodes (episode_id, series_id, season, episode, imdb_id)
            VALUES (:ep_id, :series_id, :season, :episode, :imdb_id)
        ');
        $stmt->bindValue(':ep_id', (int)$episodeId, SQLITE3_INTEGER);
        $stmt->bindValue(':series_id', (int)$seriesId, SQLITE3_INTEGER);
        $stmt->bindValue(':season', (int)$season, SQLITE3_INTEGER);
        $stmt->bindValue(':episode', (int)$episode, SQLITE3_INTEGER);
        $stmt->bindValue(':imdb_id', $imdbId, SQLITE3_TEXT);
        return $stmt->execute() !== false;
    }
    
    /**
     * Bulk insert episodes (much faster for large imports)
     */
    public function bulkInsertEpisodes($episodes) {
        $this->db->exec('BEGIN TRANSACTION');
        
        $stmt = $this->db->prepare('
            INSERT OR REPLACE INTO episodes (episode_id, series_id, season, episode, imdb_id)
            VALUES (:ep_id, :series_id, :season, :episode, :imdb_id)
        ');
        
        $count = 0;
        foreach ($episodes as $epId => $data) {
            $stmt->bindValue(':ep_id', (int)$epId, SQLITE3_INTEGER);
            $stmt->bindValue(':series_id', (int)$data['series_id'], SQLITE3_INTEGER);
            $stmt->bindValue(':season', (int)$data['season'], SQLITE3_INTEGER);
            $stmt->bindValue(':episode', (int)$data['episode'], SQLITE3_INTEGER);
            $stmt->bindValue(':imdb_id', $data['imdb_id'] ?? '', SQLITE3_TEXT);
            $stmt->execute();
            $stmt->reset();
            $count++;
        }
        
        $this->db->exec('COMMIT');
        return $count;
    }
    
    /**
     * Check if a series has been cached
     */
    public function isSeriesCached($seriesId) {
        $stmt = $this->db->prepare('SELECT 1 FROM series WHERE series_id = :id');
        $stmt->bindValue(':id', (int)$seriesId, SQLITE3_INTEGER);
        $result = $stmt->execute();
        return $result->fetchArray() !== false;
    }
    
    /**
     * Mark a series as cached
     */
    public function markSeriesCached($seriesId, $name = '', $imdbId = '', $episodeCount = 0) {
        $stmt = $this->db->prepare('
            INSERT OR REPLACE INTO series (series_id, name, imdb_id, episode_count, cached_at)
            VALUES (:id, :name, :imdb_id, :count, :time)
        ');
        $stmt->bindValue(':id', (int)$seriesId, SQLITE3_INTEGER);
        $stmt->bindValue(':name', $name, SQLITE3_TEXT);
        $stmt->bindValue(':imdb_id', $imdbId, SQLITE3_TEXT);
        $stmt->bindValue(':count', (int)$episodeCount, SQLITE3_INTEGER);
        $stmt->bindValue(':time', time(), SQLITE3_INTEGER);
        return $stmt->execute() !== false;
    }
    
    /**
     * Get all cached series IDs
     */
    public function getCachedSeriesIds() {
        $result = $this->db->query('SELECT series_id FROM series');
        $ids = [];
        while ($row = $result->fetchArray(SQLITE3_NUM)) {
            $ids[] = $row[0];
        }
        return $ids;
    }
    
    /**
     * Get statistics
     */
    public function getStats() {
        $episodes = $this->db->querySingle('SELECT COUNT(*) FROM episodes');
        $series = $this->db->querySingle('SELECT COUNT(*) FROM series');
        $movies = $this->db->querySingle('SELECT COUNT(*) FROM movies');
        return [
            'episodes' => $episodes,
            'series' => $series,
            'movies' => $movies
        ];
    }
    
    /**
     * Import from JSON file (for migration)
     */
    public function importFromJson($jsonFile) {
        if (!file_exists($jsonFile)) {
            return ['error' => 'File not found'];
        }
        
        echo "Reading JSON file...\n";
        $content = file_get_contents($jsonFile);
        $fileSize = strlen($content);
        echo "File size: " . round($fileSize / 1024 / 1024, 2) . " MB\n";
        
        // Use regex to extract episodes (memory efficient)
        echo "Extracting episodes...\n";
        preg_match_all('/"(\d+)":\{"series_id":"(\d+)","season":(\d+),"episode":(\d+),"imdb_id":"([^"]*)"\}/', $content, $matches, PREG_SET_ORDER);
        
        unset($content); // Free memory
        
        $total = count($matches);
        echo "Found $total episodes\n";
        
        if ($total == 0) {
            return ['error' => 'No episodes found in JSON'];
        }
        
        echo "Importing to database...\n";
        $this->db->exec('BEGIN TRANSACTION');
        
        $stmt = $this->db->prepare('
            INSERT OR REPLACE INTO episodes (episode_id, series_id, season, episode, imdb_id)
            VALUES (:ep_id, :series_id, :season, :episode, :imdb_id)
        ');
        
        $seriesIds = [];
        $count = 0;
        
        foreach ($matches as $match) {
            $stmt->bindValue(':ep_id', (int)$match[1], SQLITE3_INTEGER);
            $stmt->bindValue(':series_id', (int)$match[2], SQLITE3_INTEGER);
            $stmt->bindValue(':season', (int)$match[3], SQLITE3_INTEGER);
            $stmt->bindValue(':episode', (int)$match[4], SQLITE3_INTEGER);
            $stmt->bindValue(':imdb_id', $match[5], SQLITE3_TEXT);
            $stmt->execute();
            $stmt->reset();
            
            $seriesIds[$match[2]] = true;
            $count++;
            
            if ($count % 50000 === 0) {
                echo "  Imported $count / $total...\n";
            }
        }
        
        // Mark all series as cached
        $seriesStmt = $this->db->prepare('
            INSERT OR IGNORE INTO series (series_id, cached_at) VALUES (:id, :time)
        ');
        foreach (array_keys($seriesIds) as $sid) {
            $seriesStmt->bindValue(':id', (int)$sid, SQLITE3_INTEGER);
            $seriesStmt->bindValue(':time', time(), SQLITE3_INTEGER);
            $seriesStmt->execute();
            $seriesStmt->reset();
        }
        
        $this->db->exec('COMMIT');
        
        echo "Import complete!\n";
        
        return [
            'episodes' => $count,
            'series' => count($seriesIds)
        ];
    }
    
    public function close() {
        $this->db->close();
    }
}
