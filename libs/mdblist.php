<?php
/**
 * MDBList Integration Library
 * Fetch movie/TV lists from MDBList.com for playlist automation
 * 
 * MDBList provides curated lists with TMDB IDs, IMDB IDs, and metadata
 * which can be directly integrated into the playlist generation pipeline.
 */

class MDBListProvider {
    private $apiKey;
    private $cacheDir;
    private $cacheExpiry = 3600; // 1 hour cache
    
    public function __construct($apiKey = null) {
        $this->apiKey = $apiKey ?: ($GLOBALS['MDBLIST_API_KEY'] ?? '');
        $this->cacheDir = __DIR__ . '/../cache/mdblist/';
        
        if (!is_dir($this->cacheDir)) {
            @mkdir($this->cacheDir, 0755, true);
        }
    }
    
    /**
     * Set the API key
     */
    public function setApiKey($apiKey) {
        $this->apiKey = $apiKey;
    }
    
    /**
     * Check if API key is configured
     */
    public function hasApiKey() {
        return !empty($this->apiKey);
    }
    
    /**
     * Test API connection
     */
    public function testConnection() {
        if (!$this->hasApiKey()) {
            return ['success' => false, 'error' => 'API key not configured'];
        }
        
        $url = "https://mdblist.com/api/user/?apikey=" . urlencode($this->apiKey);
        $response = $this->fetchUrl($url);
        
        if ($response === false) {
            return ['success' => false, 'error' => 'Failed to connect to MDBList API'];
        }
        
        $data = json_decode($response, true);
        if (isset($data['error'])) {
            return ['success' => false, 'error' => $data['error']];
        }
        
        return [
            'success' => true,
            'user' => $data['name'] ?? 'Unknown',
            'api_requests_remaining' => $data['api_requests'] ?? 'Unknown'
        ];
    }
    
    /**
     * Get user's MDBList lists
     */
    public function getUserLists() {
        if (!$this->hasApiKey()) {
            return ['success' => false, 'error' => 'API key not configured'];
        }
        
        $url = "https://mdblist.com/api/lists/user/?apikey=" . urlencode($this->apiKey);
        $response = $this->fetchUrl($url);
        
        if ($response === false) {
            return ['success' => false, 'error' => 'Failed to fetch user lists'];
        }
        
        $data = json_decode($response, true);
        if (isset($data['error'])) {
            return ['success' => false, 'error' => $data['error']];
        }
        
        return ['success' => true, 'lists' => $data];
    }
    
    /**
     * Fetch items from a public MDBList URL
     * Supports URLs like: https://mdblist.com/lists/username/listname
     */
    public function fetchListByUrl($url) {
        // Parse the URL to extract username and list name
        if (preg_match('/mdblist\.com\/lists\/([^\/]+)\/([^\/\?]+)/', $url, $matches)) {
            $username = $matches[1];
            $listSlug = $matches[2];
            return $this->fetchPublicList($username, $listSlug);
        }
        
        // Try as a list ID
        if (preg_match('/mdblist\.com\/lists\/(\d+)/', $url, $matches)) {
            return $this->fetchListById($matches[1]);
        }
        
        return ['success' => false, 'error' => 'Invalid MDBList URL format'];
    }
    
    /**
     * Fetch a public list by username and slug
     */
    public function fetchPublicList($username, $listSlug) {
        // Check cache first
        $cacheKey = "list_{$username}_{$listSlug}";
        $cached = $this->getCache($cacheKey);
        if ($cached !== null) {
            return $cached;
        }
        
        // MDBList public list JSON endpoint
        $url = "https://mdblist.com/lists/{$username}/{$listSlug}/json";
        $response = $this->fetchUrl($url);
        
        if ($response === false) {
            return ['success' => false, 'error' => 'Failed to fetch list'];
        }
        
        $data = json_decode($response, true);
        if ($data === null) {
            return ['success' => false, 'error' => 'Invalid JSON response'];
        }
        
        // Process the list items
        $result = $this->processListItems($data, "{$username}/{$listSlug}");
        $this->setCache($cacheKey, $result);
        
        return $result;
    }
    
    /**
     * Fetch list by ID (requires API key)
     */
    public function fetchListById($listId) {
        if (!$this->hasApiKey()) {
            return ['success' => false, 'error' => 'API key required for list ID lookup'];
        }
        
        // Check cache
        $cacheKey = "list_id_{$listId}";
        $cached = $this->getCache($cacheKey);
        if ($cached !== null) {
            return $cached;
        }
        
        $url = "https://mdblist.com/api/lists/{$listId}/items/?apikey=" . urlencode($this->apiKey);
        $response = $this->fetchUrl($url);
        
        if ($response === false) {
            return ['success' => false, 'error' => 'Failed to fetch list'];
        }
        
        $data = json_decode($response, true);
        if (isset($data['error'])) {
            return ['success' => false, 'error' => $data['error']];
        }
        
        $result = $this->processListItems($data, "list_$listId");
        $this->setCache($cacheKey, $result);
        
        return $result;
    }
    
    /**
     * Search MDBList for lists
     */
    public function searchLists($query) {
        $url = "https://mdblist.com/api/lists/search/?query=" . urlencode($query);
        if ($this->hasApiKey()) {
            $url .= "&apikey=" . urlencode($this->apiKey);
        }
        
        $response = $this->fetchUrl($url);
        if ($response === false) {
            return ['success' => false, 'error' => 'Search failed'];
        }
        
        $data = json_decode($response, true);
        return ['success' => true, 'lists' => $data ?? []];
    }
    
    /**
     * Get top/popular MDBList lists
     */
    public function getTopLists($type = 'movie') {
        $url = "https://mdblist.com/api/lists/top/?mediatype=" . urlencode($type);
        if ($this->hasApiKey()) {
            $url .= "&apikey=" . urlencode($this->apiKey);
        }
        
        $response = $this->fetchUrl($url);
        if ($response === false) {
            return ['success' => false, 'error' => 'Failed to fetch top lists'];
        }
        
        $data = json_decode($response, true);
        return ['success' => true, 'lists' => $data ?? []];
    }
    
    /**
     * Process list items into our standard format
     */
    private function processListItems($items, $sourceName) {
        $movies = [];
        $series = [];
        
        if (!is_array($items)) {
            return [
                'success' => true,
                'source' => $sourceName,
                'movies' => [],
                'series' => [],
                'total' => 0
            ];
        }
        
        foreach ($items as $item) {
            // Skip items without TMDB ID
            $tmdbId = $item['tmdb_id'] ?? $item['id'] ?? null;
            if (!$tmdbId) continue;
            
            $mediaType = $item['mediatype'] ?? $item['media_type'] ?? 'movie';
            
            $processed = [
                'id' => (int)$tmdbId,
                'tmdb_id' => (int)$tmdbId,
                'imdb_id' => $item['imdb_id'] ?? null,
                'title' => $item['title'] ?? $item['name'] ?? 'Unknown',
                'year' => $item['release_year'] ?? $item['year'] ?? null,
                'rating' => $item['score'] ?? $item['rating'] ?? null,
                'poster_path' => $item['poster'] ?? null,
                'backdrop_path' => $item['backdrop'] ?? null,
                'overview' => $item['description'] ?? $item['overview'] ?? '',
                'source' => 'mdblist:' . $sourceName
            ];
            
            if ($mediaType === 'movie') {
                $movies[] = $processed;
            } else {
                // TV show
                $processed['name'] = $processed['title'];
                $series[] = $processed;
            }
        }
        
        return [
            'success' => true,
            'source' => $sourceName,
            'movies' => $movies,
            'series' => $series,
            'total' => count($movies) + count($series),
            'movie_count' => count($movies),
            'series_count' => count($series),
            'fetched_at' => date('Y-m-d H:i:s')
        ];
    }
    
    /**
     * Fetch all configured MDBList lists and merge results
     */
    public function fetchAllConfiguredLists() {
        $lists = $GLOBALS['MDBLIST_URLS'] ?? [];
        if (empty($lists)) {
            return [
                'success' => true,
                'movies' => [],
                'series' => [],
                'total' => 0,
                'message' => 'No MDBList URLs configured'
            ];
        }
        
        $allMovies = [];
        $allSeries = [];
        $errors = [];
        $seenMovieIds = [];
        $seenSeriesIds = [];
        
        foreach ($lists as $listUrl) {
            $result = $this->fetchListByUrl($listUrl);
            
            if (!$result['success']) {
                $errors[] = "Failed to fetch $listUrl: " . ($result['error'] ?? 'Unknown error');
                continue;
            }
            
            // Deduplicate movies
            foreach ($result['movies'] as $movie) {
                if (!isset($seenMovieIds[$movie['id']])) {
                    $seenMovieIds[$movie['id']] = true;
                    $allMovies[] = $movie;
                }
            }
            
            // Deduplicate series
            foreach ($result['series'] as $show) {
                if (!isset($seenSeriesIds[$show['id']])) {
                    $seenSeriesIds[$show['id']] = true;
                    $allSeries[] = $show;
                }
            }
        }
        
        return [
            'success' => true,
            'movies' => $allMovies,
            'series' => $allSeries,
            'total' => count($allMovies) + count($allSeries),
            'movie_count' => count($allMovies),
            'series_count' => count($allSeries),
            'lists_processed' => count($lists),
            'errors' => $errors,
            'fetched_at' => date('Y-m-d H:i:s')
        ];
    }
    
    /**
     * Convert MDBList items to playlist format
     */
    public function toPlaylistFormat($items, $type = 'movie', $playVodUrl = '') {
        $playlist = [];
        $num = 1;
        
        foreach ($items as $item) {
            $entry = [
                'num' => $num++,
                'name' => $item['title'] ?? $item['name'],
                'stream_type' => $type === 'movie' ? 'movie' : 'series',
                'stream_id' => $item['id'],
                'stream_icon' => $item['poster_path'] ?? '',
                'rating' => $item['rating'] ?? 0,
                'added' => time(),
                'category_id' => 'mdblist',
                'container_extension' => 'mp4',
                'custom_sid' => '',
                'direct_source' => ''
            ];
            
            if ($type === 'movie') {
                $entry['year'] = $item['year'] ?? '';
                $entry['plot'] = $item['overview'] ?? '';
            } else {
                $entry['series_id'] = $item['id'];
                $entry['cover'] = $item['poster_path'] ?? '';
            }
            
            $playlist[] = $entry;
        }
        
        return $playlist;
    }
    
    /**
     * HTTP fetch helper
     */
    private function fetchUrl($url, $timeout = 30) {
        $context = stream_context_create([
            'http' => [
                'timeout' => $timeout,
                'header' => [
                    'Accept: application/json',
                    'User-Agent: TMDB-VOD-Playlist/1.0'
                ]
            ],
            'ssl' => [
                'verify_peer' => true,
                'verify_peer_name' => true
            ]
        ]);
        
        $response = @file_get_contents($url, false, $context);
        return $response;
    }
    
    /**
     * Cache helpers
     */
    private function getCacheFile($key) {
        return $this->cacheDir . md5($key) . '.json';
    }
    
    private function getCache($key) {
        $file = $this->getCacheFile($key);
        if (!file_exists($file)) return null;
        
        $data = json_decode(file_get_contents($file), true);
        if (!$data || !isset($data['expires']) || $data['expires'] < time()) {
            @unlink($file);
            return null;
        }
        
        return $data['data'];
    }
    
    private function setCache($key, $data) {
        $file = $this->getCacheFile($key);
        $cacheData = [
            'expires' => time() + $this->cacheExpiry,
            'data' => $data
        ];
        file_put_contents($file, json_encode($cacheData));
    }
    
    /**
     * Clear all MDBList cache
     */
    public function clearCache() {
        $files = glob($this->cacheDir . '*.json');
        $count = 0;
        foreach ($files as $file) {
            if (unlink($file)) $count++;
        }
        return $count;
    }
    
    /**
     * Get saved MDBList URLs from JSON config
     */
    public static function getSavedLists() {
        $configFile = __DIR__ . '/../cache/mdblist_config.json';
        if (!file_exists($configFile)) {
            return [];
        }
        $data = json_decode(file_get_contents($configFile), true);
        return $data['lists'] ?? [];
    }
    
    /**
     * Save MDBList URLs to JSON config
     */
    public static function saveLists($lists) {
        $configFile = __DIR__ . '/../cache/mdblist_config.json';
        $data = [
            'lists' => $lists,
            'updated' => date('Y-m-d H:i:s')
        ];
        return file_put_contents($configFile, json_encode($data, JSON_PRETTY_PRINT)) !== false;
    }
    
    /**
     * Add a list URL
     */
    public static function addList($url, $name = '', $enabled = true) {
        $lists = self::getSavedLists();
        
        // Check if already exists
        foreach ($lists as $list) {
            if ($list['url'] === $url) {
                return ['success' => false, 'error' => 'List already exists'];
            }
        }
        
        $lists[] = [
            'url' => $url,
            'name' => $name ?: self::extractListName($url),
            'enabled' => $enabled,
            'added' => date('Y-m-d H:i:s')
        ];
        
        self::saveLists($lists);
        return ['success' => true];
    }
    
    /**
     * Remove a list URL
     */
    public static function removeList($url) {
        $lists = self::getSavedLists();
        $lists = array_filter($lists, function($list) use ($url) {
            return $list['url'] !== $url;
        });
        self::saveLists(array_values($lists));
        return ['success' => true];
    }
    
    /**
     * Toggle list enabled status
     */
    public static function toggleList($url, $enabled) {
        $lists = self::getSavedLists();
        foreach ($lists as &$list) {
            if ($list['url'] === $url) {
                $list['enabled'] = $enabled;
                break;
            }
        }
        self::saveLists($lists);
        return ['success' => true];
    }
    
    /**
     * Extract list name from URL
     */
    private static function extractListName($url) {
        if (preg_match('/mdblist\.com\/lists\/([^\/]+)\/([^\/\?]+)/', $url, $matches)) {
            return ucwords(str_replace('-', ' ', $matches[2]));
        }
        return 'MDBList';
    }
}
