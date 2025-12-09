
<?php
/**
 * Admin Dashboard - Radarr/Sonarr Style
 * Monitor services, start/stop daemons, modify settings
 */

session_start();
require_once __DIR__ . '/config.php';

// Simple auth (change this password!)
$ADMIN_PASSWORD = 'admin123';

// Handle login
if (isset($_POST['password'])) {
    if ($_POST['password'] === $ADMIN_PASSWORD) {
        $_SESSION['admin_auth'] = true;
    }
}

// Handle logout
if (isset($_GET['logout'])) {
    unset($_SESSION['admin_auth']);
    header('Location: admin.php');
    exit;
}

// Check auth
$isAuthenticated = isset($_SESSION['admin_auth']) && $_SESSION['admin_auth'] === true;

// API Actions
if (isset($_GET['api'])) {
    header('Content-Type: application/json');
    
    if (!$isAuthenticated) {
        echo json_encode(['error' => 'Not authenticated']);
        exit;
    }
    
    $action = $_GET['api'];
    
    switch ($action) {
        case 'status':
            echo json_encode(getSystemStatus());
            break;
            
        case 'daemon-start':
            // Try systemd first, fallback to manual start
            $systemdResult = shell_exec('systemctl start streaming-sync.service 2>&1');
            $isActive = trim(shell_exec('systemctl is-active streaming-sync.service 2>/dev/null') ?? '') === 'active';
            if ($isActive) {
                $pid = trim(shell_exec('systemctl show streaming-sync.service --property=MainPID --value 2>/dev/null') ?? '');
                echo json_encode(['success' => true, 'pid' => $pid, 'method' => 'systemd']);
            } else {
                // Fallback to nohup
                $result = shell_exec('nohup php ' . __DIR__ . '/daemons/background_sync_daemon.php --daemon > /dev/null 2>&1 & echo $!');
                echo json_encode(['success' => true, 'pid' => trim($result), 'method' => 'manual']);
            }
            break;
            
        case 'daemon-stop':
            // Try systemd first
            shell_exec('systemctl stop streaming-sync.service 2>/dev/null');
            // Also clean up lock file if it exists
            $lockFile = __DIR__ . '/cache/sync_daemon.lock';
            if (file_exists($lockFile)) {
                $pid = trim(file_get_contents($lockFile));
                if ($pid) {
                    posix_kill(intval($pid), SIGTERM);
                }
                unlink($lockFile);
            }
            echo json_encode(['success' => true]);
            break;
            
        case 'sync-now':
            $result = shell_exec('php ' . __DIR__ . '/daemons/background_sync_daemon.php 2>&1');
            echo json_encode(['success' => true, 'output' => $result]);
            break;
            
        case 'generate-playlist':
            $result = shell_exec('php ' . __DIR__ . '/daemons/auto_playlist_daemon.php 2>&1');
            echo json_encode(['success' => true, 'output' => $result]);
            break;
            
        case 'cache-episodes':
            $result = shell_exec('nohup php ' . __DIR__ . '/daemons/sync_github_cache.php > /dev/null 2>&1 & echo "Started"');
            echo json_encode(['success' => true, 'message' => 'Episode cache sync started in background']);
            break;
            
        case 'logs':
            $logFile = $_GET['file'] ?? 'sync_daemon';
            $logPath = __DIR__ . '/logs/' . basename($logFile) . '.log';
            $lines = isset($_GET['lines']) ? intval($_GET['lines']) : 100;
            
            if (file_exists($logPath)) {
                $content = shell_exec("tail -n $lines " . escapeshellarg($logPath));
                echo json_encode(['success' => true, 'content' => $content]);
            } else {
                echo json_encode(['success' => false, 'error' => 'Log file not found']);
            }
            break;
            
        case 'save-settings':
            $settings = json_decode(file_get_contents('php://input'), true);
            if ($settings) {
                $result = updateConfigFile($settings);
                echo json_encode(['success' => $result]);
            } else {
                echo json_encode(['success' => false, 'error' => 'Invalid settings']);
            }
            break;
            
        case 'save-filters':
            $filters = json_decode(file_get_contents('php://input'), true);
            if ($filters) {
                $result = updateFilterConfig($filters);
                echo json_encode(['success' => $result]);
            } else {
                echo json_encode(['success' => false, 'error' => 'Invalid filters']);
            }
            break;
            
        case 'test-provider':
            $provider = $_GET['provider'] ?? 'comet';
            $result = testProvider($provider);
            echo json_encode($result);
            break;
            
        case 'search-collections':
            $query = $_GET['q'] ?? '';
            $results = searchTMDBCollections($query);
            echo json_encode($results);
            break;
            
        case 'get-collection':
            $collectionId = $_GET['id'] ?? '';
            $result = getTMDBCollection($collectionId);
            echo json_encode($result);
            break;
            
        case 'add-collection':
            $data = json_decode(file_get_contents('php://input'), true);
            $result = addCollectionToPlaylist($data['movies'] ?? []);
            echo json_encode($result);
            break;
            
        case 'populate-streams-status':
            $statusFile = __DIR__ . '/cache/stream_populate_status.json';
            if (file_exists($statusFile)) {
                echo file_get_contents($statusFile);
            } else {
                echo json_encode(['status' => 'idle', 'progress' => 0]);
            }
            break;
            
        case 'clear-episode-cache':
            // Clear episode cache (JSON files AND SQLite database)
            $cacheFile = __DIR__ . '/cache/episode_lookup.json';
            $shardDir = __DIR__ . '/cache/episode_shards';
            $cacheDb = __DIR__ . '/cache/episodes.db';
            $cleared = 0;
            
            // Clear JSON file
            if (file_exists($cacheFile)) {
                unlink($cacheFile);
                $cleared++;
            }
            
            // Clear shard files
            if (is_dir($shardDir)) {
                $files = glob($shardDir . '/*.json');
                foreach ($files as $file) {
                    unlink($file);
                    $cleared++;
                }
            }
            
            // Clear SQLite database
            if (file_exists($cacheDb)) {
                $db = new SQLite3($cacheDb);
                $db->exec('DELETE FROM episodes');
                $db->exec('VACUUM');
                $db->close();
                $cleared++;
            }
            
            echo json_encode(['success' => true, 'message' => "Cleared episode cache ($cleared items)"]);
            break;
            
        case 'clear-m3u8':
            // Clear M3U8 and regenerate empty
            $m3uFile = __DIR__ . '/playlist.m3u8';
            if (file_exists($m3uFile)) {
                file_put_contents($m3uFile, "#EXTM3U\n");
                echo json_encode(['success' => true, 'message' => 'M3U8 playlist cleared']);
            } else {
                echo json_encode(['success' => true, 'message' => 'M3U8 file not found']);
            }
            break;
            
        case 'clear-all-playlists':
            // Clear all playlists (movies, series, m3u8)
            $cleared = [];
            $files = [
                'playlist.json' => __DIR__ . '/playlist.json',
                'tv_playlist.json' => __DIR__ . '/tv_playlist.json',
                'playlist.m3u8' => __DIR__ . '/playlist.m3u8'
            ];
            foreach ($files as $name => $path) {
                if (file_exists($path)) {
                    if (strpos($name, '.json') !== false) {
                        file_put_contents($path, '[]');
                    } else {
                        file_put_contents($path, "#EXTM3U\n");
                    }
                    $cleared[] = $name;
                }
            }
            echo json_encode(['success' => true, 'message' => 'Cleared: ' . implode(', ', $cleared)]);
            break;
        
        // ========== MDBLIST API HANDLERS ==========
        case 'mdblist-test':
            require_once __DIR__ . '/libs/mdblist.php';
            $mdblist = new MDBListProvider();
            $result = $mdblist->testConnection();
            echo json_encode($result);
            break;
            
        case 'mdblist-get-lists':
            require_once __DIR__ . '/libs/mdblist.php';
            $lists = MDBListProvider::getSavedLists();
            echo json_encode(['success' => true, 'lists' => $lists]);
            break;
            
        case 'mdblist-add-list':
            require_once __DIR__ . '/libs/mdblist.php';
            $data = json_decode(file_get_contents('php://input'), true);
            $url = $data['url'] ?? '';
            $name = $data['name'] ?? '';
            
            if (empty($url)) {
                echo json_encode(['success' => false, 'error' => 'URL is required']);
                break;
            }
            
            // Validate URL format
            if (!preg_match('/mdblist\.com\/lists\//', $url)) {
                echo json_encode(['success' => false, 'error' => 'Invalid MDBList URL format']);
                break;
            }
            
            // Test fetch the list first
            $mdblist = new MDBListProvider();
            $testResult = $mdblist->fetchListByUrl($url);
            
            if (!$testResult['success']) {
                echo json_encode(['success' => false, 'error' => 'Failed to fetch list: ' . ($testResult['error'] ?? 'Unknown error')]);
                break;
            }
            
            $result = MDBListProvider::addList($url, $name);
            if ($result['success']) {
                $result['preview'] = [
                    'movies' => $testResult['movie_count'] ?? 0,
                    'series' => $testResult['series_count'] ?? 0,
                    'total' => $testResult['total'] ?? 0
                ];
            }
            echo json_encode($result);
            break;
            
        case 'mdblist-remove-list':
            require_once __DIR__ . '/libs/mdblist.php';
            $data = json_decode(file_get_contents('php://input'), true);
            $url = $data['url'] ?? '';
            
            if (empty($url)) {
                echo json_encode(['success' => false, 'error' => 'URL is required']);
                break;
            }
            
            $result = MDBListProvider::removeList($url);
            echo json_encode($result);
            break;
            
        case 'mdblist-toggle-list':
            require_once __DIR__ . '/libs/mdblist.php';
            $data = json_decode(file_get_contents('php://input'), true);
            $url = $data['url'] ?? '';
            $enabled = $data['enabled'] ?? true;
            
            $result = MDBListProvider::toggleList($url, $enabled);
            echo json_encode($result);
            break;
            
        case 'mdblist-preview':
            require_once __DIR__ . '/libs/mdblist.php';
            $url = $_GET['url'] ?? '';
            
            if (empty($url)) {
                echo json_encode(['success' => false, 'error' => 'URL is required']);
                break;
            }
            
            $mdblist = new MDBListProvider();
            $result = $mdblist->fetchListByUrl($url);
            echo json_encode($result);
            break;
            
        case 'mdblist-sync':
            require_once __DIR__ . '/libs/mdblist.php';
            $mdblist = new MDBListProvider();
            $result = $mdblist->fetchAllConfiguredLists();
            
            // Store the result for playlist generation
            if ($result['success']) {
                $cacheFile = __DIR__ . '/cache/mdblist_items.json';
                file_put_contents($cacheFile, json_encode($result, JSON_PRETTY_PRINT));
                
                // Auto-merge if MDBLIST_MERGE_PLAYLIST is enabled
                if ($GLOBALS['MDBLIST_MERGE_PLAYLIST'] ?? true) {
                    $playlistFile = __DIR__ . '/playlist.json';
                    $tvPlaylistFile = __DIR__ . '/tv_playlist.json';
                    
                    $mdbMovies = $result['movies'] ?? [];
                    $mdbSeries = $result['series'] ?? [];
                    
                    // Load existing playlists
                    $existingMovies = file_exists($playlistFile) ? (json_decode(file_get_contents($playlistFile), true) ?: []) : [];
                    $existingSeries = file_exists($tvPlaylistFile) ? (json_decode(file_get_contents($tvPlaylistFile), true) ?: []) : [];
                    
                    // Create ID maps for deduplication
                    $existingMovieIds = array_column($existingMovies, 'tmdb_id');
                    $existingMovieIds = array_merge($existingMovieIds, array_column($existingMovies, 'id'));
                    $existingSeriesIds = array_column($existingSeries, 'tmdb_id');
                    $existingSeriesIds = array_merge($existingSeriesIds, array_column($existingSeries, 'id'));
                    
                    $addedMovies = 0;
                    $addedSeries = 0;
                    
                    // Merge movies
                    foreach ($mdbMovies as $movie) {
                        $id = $movie['tmdb_id'] ?? $movie['id'] ?? null;
                        if ($id && !in_array($id, $existingMovieIds)) {
                            $existingMovies[] = $movie;
                            $existingMovieIds[] = $id;
                            $addedMovies++;
                        }
                    }
                    
                    // Merge series
                    foreach ($mdbSeries as $series) {
                        $id = $series['tmdb_id'] ?? $series['id'] ?? null;
                        if ($id && !in_array($id, $existingSeriesIds)) {
                            $existingSeries[] = $series;
                            $existingSeriesIds[] = $id;
                            $addedSeries++;
                        }
                    }
                    
                    // Save merged playlists
                    file_put_contents($playlistFile, json_encode($existingMovies, JSON_PRETTY_PRINT));
                    file_put_contents($tvPlaylistFile, json_encode($existingSeries, JSON_PRETTY_PRINT));
                    
                    // Regenerate M3U8
                    regenerateM3U8Playlist($existingMovies, $existingSeries);
                    
                    $result['auto_merged'] = true;
                    $result['added_movies'] = $addedMovies;
                    $result['added_series'] = $addedSeries;
                    $result['total_movies'] = count($existingMovies);
                    $result['total_series'] = count($existingSeries);
                }
            }
            
            echo json_encode($result);
            break;
            
        case 'mdblist-search':
            require_once __DIR__ . '/libs/mdblist.php';
            $query = $_GET['q'] ?? '';
            
            if (empty($query)) {
                echo json_encode(['success' => false, 'error' => 'Search query required']);
                break;
            }
            
            $mdblist = new MDBListProvider();
            $result = $mdblist->searchLists($query);
            echo json_encode($result);
            break;
            
        case 'mdblist-top-lists':
            require_once __DIR__ . '/libs/mdblist.php';
            $type = $_GET['type'] ?? 'movie';
            
            $mdblist = new MDBListProvider();
            $result = $mdblist->getTopLists($type);
            echo json_encode($result);
            break;
            
        case 'mdblist-clear-cache':
            require_once __DIR__ . '/libs/mdblist.php';
            $mdblist = new MDBListProvider();
            $count = $mdblist->clearCache();
            echo json_encode(['success' => true, 'message' => "Cleared $count cached items"]);
            break;
            
        case 'mdblist-save-settings':
            $data = json_decode(file_get_contents('php://input'), true);
            $result = updateMDBListConfig($data);
            echo json_encode(['success' => $result]);
            break;
        
        case 'mdblist-merge':
            // Merge MDBList items into main playlists
            $mdblistFile = __DIR__ . '/cache/mdblist_items.json';
            $playlistFile = __DIR__ . '/playlist.json';
            $tvPlaylistFile = __DIR__ . '/tv_playlist.json';
            
            if (!file_exists($mdblistFile)) {
                echo json_encode(['success' => false, 'error' => 'No MDBList items to merge. Sync first.']);
                break;
            }
            
            $mdblistData = json_decode(file_get_contents($mdblistFile), true);
            $mdbMovies = $mdblistData['movies'] ?? [];
            $mdbSeries = $mdblistData['series'] ?? [];
            
            // Load existing playlists
            $existingMovies = file_exists($playlistFile) ? (json_decode(file_get_contents($playlistFile), true) ?: []) : [];
            $existingSeries = file_exists($tvPlaylistFile) ? (json_decode(file_get_contents($tvPlaylistFile), true) ?: []) : [];
            
            // Create ID maps for deduplication
            $existingMovieIds = array_column($existingMovies, 'tmdb_id');
            $existingMovieIds = array_merge($existingMovieIds, array_column($existingMovies, 'id'));
            $existingSeriesIds = array_column($existingSeries, 'tmdb_id');
            $existingSeriesIds = array_merge($existingSeriesIds, array_column($existingSeries, 'id'));
            
            $addedMovies = 0;
            $addedSeries = 0;
            
            // Merge movies
            foreach ($mdbMovies as $movie) {
                $id = $movie['tmdb_id'] ?? $movie['id'] ?? null;
                if ($id && !in_array($id, $existingMovieIds)) {
                    $existingMovies[] = $movie;
                    $existingMovieIds[] = $id;
                    $addedMovies++;
                }
            }
            
            // Merge series
            foreach ($mdbSeries as $series) {
                $id = $series['tmdb_id'] ?? $series['id'] ?? null;
                if ($id && !in_array($id, $existingSeriesIds)) {
                    $existingSeries[] = $series;
                    $existingSeriesIds[] = $id;
                    $addedSeries++;
                }
            }
            
            // Save merged playlists
            file_put_contents($playlistFile, json_encode($existingMovies, JSON_PRETTY_PRINT));
            file_put_contents($tvPlaylistFile, json_encode($existingSeries, JSON_PRETTY_PRINT));
            
            // Mark as merged
            $mdblistData['merged'] = date('Y-m-d H:i:s');
            file_put_contents($mdblistFile, json_encode($mdblistData, JSON_PRETTY_PRINT));
            
            // Auto-regenerate M3U8 playlist
            $m3u8Regenerated = regenerateM3U8Playlist($existingMovies, $existingSeries);
            
            echo json_encode([
                'success' => true, 
                'added_movies' => $addedMovies,
                'added_series' => $addedSeries,
                'total_movies' => count($existingMovies),
                'total_series' => count($existingSeries),
                'm3u8_regenerated' => $m3u8Regenerated
            ]);
            break;
        
        // ========== SERVICE MANAGEMENT API ==========
        case 'regenerate-m3u8':
            $result = regenerateM3U8Playlist();
            echo json_encode([
                'success' => $result,
                'message' => $result ? 'M3U8 playlist regenerated successfully' : 'Failed to regenerate M3U8'
            ]);
            break;
            
        case 'regenerate-livetv':
            // Run background sync daemon to refresh Live TV
            $output = [];
            $returnCode = 0;
            exec('cd ' . __DIR__ . ' && php daemons/background_sync_daemon.php --force 2>&1', $output, $returnCode);
            echo json_encode([
                'success' => $returnCode === 0,
                'output' => implode("\n", array_slice($output, -10)),
                'message' => $returnCode === 0 ? 'Live TV refreshed successfully' : 'Failed to refresh Live TV'
            ]);
            break;
            
        case 'run-episode-cache':
            $output = [];
            $returnCode = 0;
            exec('cd ' . __DIR__ . ' && php daemons/auto_cache_daemon.php 2>&1', $output, $returnCode);
            echo json_encode([
                'success' => $returnCode === 0,
                'output' => implode("\n", array_slice($output, -15)),
                'message' => $returnCode === 0 ? 'Episode cache updated' : 'Episode cache failed'
            ]);
            break;
            
        case 'mdblist-sync-and-merge':
            require_once __DIR__ . '/libs/mdblist.php';
            $mdblist = new MDBListProvider();
            $syncResult = $mdblist->fetchAllConfiguredLists();
            
            if (!$syncResult['success']) {
                echo json_encode(['success' => false, 'error' => 'Sync failed: ' . ($syncResult['error'] ?? 'Unknown')]);
                break;
            }
            
            // Save to cache
            $cacheFile = __DIR__ . '/cache/mdblist_items.json';
            file_put_contents($cacheFile, json_encode($syncResult, JSON_PRETTY_PRINT));
            
            // Merge into playlists
            $playlistFile = __DIR__ . '/playlist.json';
            $tvPlaylistFile = __DIR__ . '/tv_playlist.json';
            
            $mdbMovies = $syncResult['movies'] ?? [];
            $mdbSeries = $syncResult['series'] ?? [];
            
            $existingMovies = file_exists($playlistFile) ? (json_decode(file_get_contents($playlistFile), true) ?: []) : [];
            $existingSeries = file_exists($tvPlaylistFile) ? (json_decode(file_get_contents($tvPlaylistFile), true) ?: []) : [];
            
            $existingMovieIds = array_column($existingMovies, 'tmdb_id');
            $existingMovieIds = array_merge($existingMovieIds, array_column($existingMovies, 'id'));
            $existingSeriesIds = array_column($existingSeries, 'tmdb_id');
            $existingSeriesIds = array_merge($existingSeriesIds, array_column($existingSeries, 'id'));
            
            $addedMovies = 0;
            $addedSeries = 0;
            
            foreach ($mdbMovies as $movie) {
                $id = $movie['tmdb_id'] ?? $movie['id'] ?? null;
                if ($id && !in_array($id, $existingMovieIds)) {
                    $existingMovies[] = $movie;
                    $existingMovieIds[] = $id;
                    $addedMovies++;
                }
            }
            
            foreach ($mdbSeries as $series) {
                $id = $series['tmdb_id'] ?? $series['id'] ?? null;
                if ($id && !in_array($id, $existingSeriesIds)) {
                    $existingSeries[] = $series;
                    $existingSeriesIds[] = $id;
                    $addedSeries++;
                }
            }
            
            file_put_contents($playlistFile, json_encode($existingMovies, JSON_PRETTY_PRINT));
            file_put_contents($tvPlaylistFile, json_encode($existingSeries, JSON_PRETTY_PRINT));
            
            // Regenerate M3U8
            $m3u8Result = regenerateM3U8Playlist($existingMovies, $existingSeries);
            
            echo json_encode([
                'success' => true,
                'synced_movies' => $syncResult['movie_count'] ?? 0,
                'synced_series' => $syncResult['series_count'] ?? 0,
                'added_movies' => $addedMovies,
                'added_series' => $addedSeries,
                'total_movies' => count($existingMovies),
                'total_series' => count($existingSeries),
                'm3u8_regenerated' => $m3u8Result,
                'message' => "Synced & merged! Added $addedMovies movies, $addedSeries series"
            ]);
            break;
            
        case 'service-status':
            // Get comprehensive service status
            require_once __DIR__ . '/libs/mdblist.php';
            
            // MDBList status
            $mdblistFile = __DIR__ . '/cache/mdblist_items.json';
            $mdblistData = file_exists($mdblistFile) ? json_decode(file_get_contents($mdblistFile), true) : null;
            $savedLists = MDBListProvider::getSavedLists();
            $enabledLists = array_filter($savedLists, fn($l) => $l['enabled'] ?? true);
            
            // Playlist status
            $playlistFile = __DIR__ . '/playlist.json';
            $tvPlaylistFile = __DIR__ . '/tv_playlist.json';
            $movies = file_exists($playlistFile) ? json_decode(file_get_contents($playlistFile), true) : [];
            $series = file_exists($tvPlaylistFile) ? json_decode(file_get_contents($tvPlaylistFile), true) : [];
            
            // M3U8 status
            $m3u8File = __DIR__ . '/playlist.m3u8';
            $m3u8Exists = file_exists($m3u8File);
            $m3u8Modified = $m3u8Exists ? date('Y-m-d H:i:s', filemtime($m3u8File)) : null;
            $m3u8Content = $m3u8Exists ? file_get_contents($m3u8File) : '';
            $liveTvCount = substr_count($m3u8Content, 'pluto.tv');
            
            // Sync status
            $syncStatusFile = __DIR__ . '/cache/sync_status.json';
            $syncStatus = file_exists($syncStatusFile) ? json_decode(file_get_contents($syncStatusFile), true) : null;
            
            echo json_encode([
                'success' => true,
                'mdblist' => [
                    'movies' => $mdblistData['movie_count'] ?? 0,
                    'series' => $mdblistData['series_count'] ?? 0,
                    'lists' => count($enabledLists),
                    'last_sync' => $mdblistData['fetched_at'] ?? null
                ],
                'library' => [
                    'movies' => count($movies),
                    'series' => count($series)
                ],
                'm3u8' => [
                    'movies' => count($movies),
                    'series' => count($series),
                    'livetv' => $liveTvCount,
                    'last_generated' => $m3u8Modified
                ],
                'sync' => [
                    'last_sync' => $syncStatus['details']['last_sync'] ?? null,
                    'status' => $syncStatus['status'] ?? 'unknown'
                ]
            ]);
            break;
            
        case 'check-cron':
            $output = [];
            exec('crontab -l 2>&1', $output);
            $cronContent = implode("\n", $output);
            
            // Check for our cron jobs
            $hasSync = strpos($cronContent, 'background_sync_daemon') !== false;
            $hasCache = strpos($cronContent, 'auto_cache_daemon') !== false;
            $hasMdblist = strpos($cronContent, 'mdblist') !== false;
            
            echo json_encode([
                'success' => true,
                'crontab' => $cronContent,
                'has_sync' => $hasSync,
                'has_cache' => $hasCache,
                'has_mdblist' => $hasMdblist
            ]);
            break;
            
        default:
            echo json_encode(['error' => 'Unknown action']);
    }
    exit;
}

/**
 * Regenerate M3U8 playlist from movies and series JSON
 */
function regenerateM3U8Playlist($movies = null, $series = null) {
    $playlistFile = __DIR__ . '/playlist.json';
    $tvPlaylistFile = __DIR__ . '/tv_playlist.json';
    $m3u8File = __DIR__ . '/playlist.m3u8';
    
    // Load from files if not passed
    if ($movies === null) {
        $movies = file_exists($playlistFile) ? (json_decode(file_get_contents($playlistFile), true) ?: []) : [];
    }
    if ($series === null) {
        $series = file_exists($tvPlaylistFile) ? (json_decode(file_get_contents($tvPlaylistFile), true) ?: []) : [];
    }
    
    // Get base URL for playlist links
    $protocol = (!empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off') ? 'https' : 'http';
    $host = $_SERVER['HTTP_HOST'] ?? 'localhost';
    $baseUrl = $protocol . '://' . $host;
    
    $m3u = "#EXTM3U\n";
    
    // Add movies
    foreach ($movies as $movie) {
        $name = htmlspecialchars($movie['name'] ?? $movie['title'] ?? 'Unknown', ENT_QUOTES);
        $streamId = $movie['stream_id'] ?? $movie['tmdb_id'] ?? $movie['id'] ?? '';
        $logo = $movie['stream_icon'] ?? $movie['poster_path'] ?? '';
        if ($logo && strpos($logo, '/') === 0) {
            $logo = 'https://image.tmdb.org/t/p/w500' . $logo;
        }
        $year = $movie['year'] ?? '';
        $category = 'Movies';
        
        $m3u .= "#EXTINF:-1 tvg-id=\"$streamId\" tvg-logo=\"$logo\" group-title=\"$category\",$name" . ($year ? " ($year)" : "") . "\n";
        $m3u .= "{$baseUrl}/play.php?type=movie&id={$streamId}\n";
    }
    
    // Add TV series
    foreach ($series as $show) {
        $name = htmlspecialchars($show['name'] ?? $show['title'] ?? 'Unknown', ENT_QUOTES);
        $seriesId = $show['series_id'] ?? $show['tmdb_id'] ?? $show['id'] ?? '';
        $logo = $show['cover'] ?? $show['stream_icon'] ?? $show['poster_path'] ?? '';
        if ($logo && strpos($logo, '/') === 0) {
            $logo = 'https://image.tmdb.org/t/p/w500' . $logo;
        }
        $category = 'TV Shows';
        
        $m3u .= "#EXTINF:-1 tvg-id=\"$seriesId\" tvg-logo=\"$logo\" group-title=\"$category\",$name\n";
        $m3u .= "{$baseUrl}/play.php?type=series&id={$seriesId}\n";
    }
    
    // Save M3U8 file
    $result = file_put_contents($m3u8File, $m3u);
    return $result !== false;
}

function getSystemStatus() {
    // Re-read config file to get latest values (after save-settings)
    $configFile = __DIR__ . '/config.php';
    if (file_exists($configFile)) {
        // Clear any cached values and re-include
        $configContent = file_get_contents($configFile);
        
        // Parse the config values directly from file content
        $configValues = [];
        
        // Extract GLOBALS values
        if (preg_match_all("/\\\$GLOBALS\\['([^']+)'\\]\\s*=\\s*([^;]+);/", $configContent, $matches, PREG_SET_ORDER)) {
            foreach ($matches as $match) {
                $key = $match[1];
                $value = trim($match[2]);
                if ($value === 'true') $configValues[$key] = true;
                elseif ($value === 'false') $configValues[$key] = false;
                elseif (is_numeric($value)) $configValues[$key] = (int)$value;
                elseif (preg_match('/^\[.*\]$/', $value)) {
                    // Parse array values like ['comet', 'mediafusion']
                    preg_match_all("/'([^']+)'/", $value, $arrayMatches);
                    $configValues[$key] = $arrayMatches[1] ?? [];
                }
                else $configValues[$key] = trim($value, "'\"");
            }
        }
        
        // Extract regular variables
        if (preg_match_all("/\\\$([a-zA-Z_]+)\\s*=\\s*([^;]+);/", $configContent, $matches, PREG_SET_ORDER)) {
            foreach ($matches as $match) {
                $key = $match[1];
                $value = trim($match[2]);
                if ($value === 'true') $configValues[$key] = true;
                elseif ($value === 'false') $configValues[$key] = false;
                elseif (is_numeric($value)) $configValues[$key] = (int)$value;
                elseif (preg_match('/^\[.*\]$/', $value)) {
                    // Parse array values
                    preg_match_all("/'([^']+)'/", $value, $arrayMatches);
                    $configValues[$key] = $arrayMatches[1] ?? [];
                }
                else $configValues[$key] = trim($value, "'\"");
            }
        }
        
        // Update GLOBALS with fresh values
        foreach ($configValues as $key => $value) {
            $GLOBALS[$key] = $value;
        }
    }
    
    $status = [
        'timestamp' => date('Y-m-d H:i:s'),
        'daemons' => [],
        'playlists' => [],
        'cache' => [],
        'providers' => [],
        'system' => []
    ];
    
    // Check daemon status - try systemd first, then lock file
    $daemonRunning = false;
    $daemonPid = null;
    
    // Check systemd service status
    $systemdStatus = trim(shell_exec('systemctl is-active streaming-sync.service 2>/dev/null') ?? '');
    if ($systemdStatus === 'active') {
        $daemonRunning = true;
        $daemonPid = trim(shell_exec('systemctl show streaming-sync.service --property=MainPID --value 2>/dev/null') ?? '');
    } else {
        // Fallback to lock file check
        $lockFile = __DIR__ . '/cache/sync_daemon.lock';
        if (file_exists($lockFile)) {
            $pid = trim(file_get_contents($lockFile));
            if ($pid && file_exists("/proc/$pid")) {
                $daemonRunning = true;
                $daemonPid = $pid;
            } elseif ($pid && posix_kill(intval($pid), 0)) {
                $daemonRunning = true;
                $daemonPid = $pid;
            }
        }
    }
    
    $status['daemons']['background_sync'] = [
        'name' => 'Background Sync Daemon',
        'running' => $daemonRunning,
        'pid' => $daemonPid,
        'description' => 'Syncs playlists from GitHub every 6 hours'
    ];
    
    // Check sync status file
    $statusFile = __DIR__ . '/cache/sync_status.json';
    if (file_exists($statusFile)) {
        $syncStatus = json_decode(file_get_contents($statusFile), true);
        // last_sync and next_sync can be in details or at root level
        $lastSync = $syncStatus['details']['last_sync'] ?? $syncStatus['last_sync'] ?? $syncStatus['updated'] ?? 'Never';
        $nextSync = $syncStatus['details']['next_sync'] ?? $syncStatus['next_sync'] ?? 'Manual';
        $status['daemons']['background_sync']['last_sync'] = $lastSync;
        $status['daemons']['background_sync']['next_sync'] = $nextSync;
        $status['daemons']['background_sync']['movies_count'] = $syncStatus['details']['movies'] ?? $syncStatus['movies_count'] ?? 0;
        $status['daemons']['background_sync']['series_count'] = $syncStatus['details']['series'] ?? $syncStatus['series_count'] ?? 0;
    }
    
    // Playlist stats
    $playlistFile = __DIR__ . '/playlist.json';
    $tvPlaylistFile = __DIR__ . '/tv_playlist.json';
    
    if (file_exists($playlistFile)) {
        $movies = json_decode(file_get_contents($playlistFile), true);
        $status['playlists']['movies'] = [
            'count' => is_array($movies) ? count($movies) : 0,
            'updated' => date('Y-m-d H:i:s', filemtime($playlistFile)),
            'size' => formatBytes(filesize($playlistFile))
        ];
    }
    
    if (file_exists($tvPlaylistFile)) {
        $series = json_decode(file_get_contents($tvPlaylistFile), true);
        $status['playlists']['series'] = [
            'count' => is_array($series) ? count($series) : 0,
            'updated' => date('Y-m-d H:i:s', filemtime($tvPlaylistFile)),
            'size' => formatBytes(filesize($tvPlaylistFile))
        ];
    }
    
    // M3U8 playlist
    $m3uFile = __DIR__ . '/playlist.m3u8';
    if (file_exists($m3uFile)) {
        $m3uContent = file_get_contents($m3uFile);
        $entryCount = substr_count($m3uContent, '#EXTINF:');
        $status['playlists']['m3u8'] = [
            'entries' => $entryCount,
            'updated' => date('Y-m-d H:i:s', filemtime($m3uFile)),
            'size' => formatBytes(filesize($m3uFile))
        ];
    }
    
    // Cache stats
    $cacheDb = __DIR__ . '/cache/episodes.db';
    if (file_exists($cacheDb)) {
        $db = new SQLite3($cacheDb);
        $result = $db->querySingle("SELECT COUNT(*) FROM episodes");
        $status['cache']['episodes'] = [
            'count' => $result ?? 0,
            'size' => formatBytes(filesize($cacheDb))
        ];
        $db->close();
    }
    
    // MDBList stats
    $mdblistFile = __DIR__ . '/cache/mdblist_items.json';
    if (file_exists($mdblistFile)) {
        $mdblistData = json_decode(file_get_contents($mdblistFile), true);
        $status['mdblist'] = [
            'movies' => is_array($mdblistData['movies'] ?? null) ? count($mdblistData['movies']) : 0,
            'series' => is_array($mdblistData['series'] ?? null) ? count($mdblistData['series']) : 0,
            'updated' => date('Y-m-d H:i:s', filemtime($mdblistFile)),
            'merged' => $mdblistData['merged'] ?? false
        ];
    } else {
        $status['mdblist'] = [
            'movies' => 0,
            'series' => 0,
            'updated' => 'Never',
            'merged' => false
        ];
    }
    
    // Provider status
    $providers = $GLOBALS['STREAM_PROVIDERS'] ?? ['comet']; if (is_string($providers)) $providers = [$providers];
    foreach ($providers as $provider) {
        $status['providers'][$provider] = [
            'enabled' => true,
            'name' => ucfirst($provider)
        ];
    }
    
    // System info
    $status['system'] = [
        'php_version' => PHP_VERSION,
        'memory_usage' => formatBytes(memory_get_usage(true)),
        'disk_free' => formatBytes(disk_free_space(__DIR__)),
        'uptime' => trim(shell_exec('uptime -p 2>/dev/null') ?: 'Unknown')
    ];
    
    // Config settings
    $status['config'] = [
        // API Keys (masked for security)
        'apiKey' => $GLOBALS['apiKey'] ?? '',
        'rdToken' => $GLOBALS['PRIVATE_TOKEN'] ?? '',
        'premiumizeKey' => $GLOBALS['premiumizeApiKey'] ?? '',
        
        // Debrid settings
        'useRealDebrid' => $GLOBALS['useRealDebrid'] ?? false,
        'usePremiumize' => $GLOBALS['usePremiumize'] ?? false,
        
        // Playlist settings
        'totalPages' => $GLOBALS['totalPages'] ?? 5,
        'maxResolution' => $GLOBALS['maxResolution'] ?? 1080,
        'm3u8Limit' => $GLOBALS['M3U8_LIMIT'] ?? 0,
        'autoCacheInterval' => $GLOBALS['autoCacheIntervalHours'] ?? 6,
        'userCreatePlaylist' => $GLOBALS['userCreatePlaylist'] ?? true,
        
        // Content options
        'includeAdult' => $GLOBALS['INCLUDE_ADULT_VOD'] ?? false,
        'debugMode' => $GLOBALS['DEBUG'] ?? false,
        
        // Regional settings
        'language' => $GLOBALS['language'] ?? 'en-US',
        'seriesCountry' => $GLOBALS['series_with_origin_country'] ?? 'US',
        'moviesCountry' => $GLOBALS['movies_with_origin_country'] ?? 'US',
        'userSetHost' => $GLOBALS['userSetHost'] ?? '',
        
        // Stream providers
        'streamProviders' => is_array($GLOBALS['STREAM_PROVIDERS'] ?? ['comet']) ? ($GLOBALS['STREAM_PROVIDERS'] ?? ['comet']) : [$GLOBALS['STREAM_PROVIDERS']],
        'cometIndexers' => $GLOBALS['COMET_INDEXERS'] ?? ['bktorrent', 'thepiratebay', 'yts', 'eztv', 'kickasstorrents'],
        'torrentioProviders' => $GLOBALS['TORRENTIO_PROVIDERS'] ?? 'yts,eztv,rarbg,1337x,thepiratebay',
        'mediafusionEnabled' => $GLOBALS['MEDIAFUSION_ENABLED'] ?? true
    ];
    
    // Filter settings
    $status['filters'] = [
        'enabled' => $GLOBALS['ENABLE_RELEASE_FILTERS'] ?? true,
        'releaseGroups' => $GLOBALS['EXCLUDED_RELEASE_GROUPS'] ?? '',
        'languages' => $GLOBALS['EXCLUDED_LANGUAGES'] ?? '',
        'qualities' => $GLOBALS['EXCLUDED_QUALITIES'] ?? '',
        'custom' => $GLOBALS['EXCLUDED_CUSTOM'] ?? ''
    ];
    
    // MDBList settings
    $status['mdblist'] = [
        'enabled' => $GLOBALS['MDBLIST_ENABLED'] ?? false,
        'apiKey' => $GLOBALS['MDBLIST_API_KEY'] ?? '',
        'syncInterval' => $GLOBALS['MDBLIST_SYNC_INTERVAL'] ?? 6,
        'mergePlaylist' => $GLOBALS['MDBLIST_MERGE_PLAYLIST'] ?? true
    ];
    
    // Load MDBList saved lists
    require_once __DIR__ . '/libs/mdblist.php';
    $status['mdblist']['lists'] = MDBListProvider::getSavedLists();
    
    // Check for cached MDBList items
    $mdblistCache = __DIR__ . '/cache/mdblist_items.json';
    if (file_exists($mdblistCache)) {
        $mdblistData = json_decode(file_get_contents($mdblistCache), true);
        $status['mdblist']['cachedMovies'] = $mdblistData['movie_count'] ?? 0;
        $status['mdblist']['cachedSeries'] = $mdblistData['series_count'] ?? 0;
        $status['mdblist']['lastSync'] = $mdblistData['fetched_at'] ?? 'Never';
    } else {
        $status['mdblist']['cachedMovies'] = 0;
        $status['mdblist']['cachedSeries'] = 0;
        $status['mdblist']['lastSync'] = 'Never';
    }
    
    return $status;
}

function testProvider($provider) {
    global $PRIVATE_TOKEN;
    
    $testImdb = 'tt0137523'; // Fight Club
    $timeout = 10;
    
    switch ($provider) {
        case 'comet':
            $config = [
                'indexers' => $GLOBALS['COMET_INDEXERS'] ?? ['yts', 'eztv', 'thepiratebay'],
                'maxResults' => 5,
                'resolutions' => ['4k', '1080p', '720p'],
                'debridService' => 'realdebrid',
                'debridApiKey' => $PRIVATE_TOKEN
            ];
            $configB64 = rtrim(strtr(base64_encode(json_encode($config)), '+/', '-_'), '=');
            $url = "https://comet.elfhosted.com/$configB64/stream/movie/$testImdb.json";
            break;
            
        case 'mediafusion':
            $config = [
                'streaming_provider' => ['token' => $PRIVATE_TOKEN, 'service' => 'realdebrid'],
                'selected_catalogs' => ['prowlarr_streams'],
                'selected_resolutions' => ['4k', '1080p', '720p', '480p']
            ];
            $configB64 = rtrim(strtr(base64_encode(json_encode($config)), '+/', '-_'), '=');
            $url = "https://mediafusion.elfhosted.com/$configB64/stream/movie/$testImdb.json";
            break;
            
        case 'torrentio':
            $providers = $GLOBALS['TORRENTIO_PROVIDERS'] ?? 'yts,eztv,rarbg,1337x,thepiratebay';
            $url = "https://torrentio.strem.fun/realdebrid=$PRIVATE_TOKEN|providers=$providers/stream/movie/$testImdb.json";
            break;
            
        default:
            return ['success' => false, 'error' => 'Unknown provider'];
    }
    
    $ch = curl_init($url);
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_TIMEOUT => $timeout,
        CURLOPT_FOLLOWLOCATION => true,
        CURLOPT_USERAGENT => 'Mozilla/5.0'
    ]);
    
    $startTime = microtime(true);
    $response = curl_exec($ch);
    $responseTime = round((microtime(true) - $startTime) * 1000);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $error = curl_error($ch);
    curl_close($ch);
    
    if ($error) {
        return ['success' => false, 'error' => $error, 'response_time' => $responseTime];
    }
    
    if ($httpCode === 403) {
        return ['success' => false, 'error' => 'Blocked by Cloudflare (403)', 'response_time' => $responseTime];
    }
    
    $data = json_decode($response, true);
    $streamCount = isset($data['streams']) ? count($data['streams']) : 0;
    
    return [
        'success' => $httpCode === 200 && $streamCount > 0,
        'http_code' => $httpCode,
        'streams' => $streamCount,
        'response_time' => $responseTime . 'ms'
    ];
}

function updateConfigFile($settings) {
    $configFile = __DIR__ . '/config.php';
    $content = file_get_contents($configFile);
    
    // Simple variable mappings (no quotes needed)
    $simpleMappings = [
        'totalPages' => '/\$totalPages\s*=\s*\d+/',
        'maxResolution' => '/\$maxResolution\s*=\s*\d+/',
        'autoCacheInterval' => '/\$autoCacheIntervalHours\s*=\s*\d+/'
    ];
    
    // Boolean mappings
    $boolMappings = [
        'useRealDebrid' => '/\$useRealDebrid\s*=\s*(true|false)/',
        'usePremiumize' => '/\$usePremiumize\s*=\s*(true|false)/',
        'userCreatePlaylist' => '/\$userCreatePlaylist\s*=\s*(true|false)/',
        'includeAdult' => '/\$INCLUDE_ADULT_VOD\s*=\s*(true|false)/',
        'debugMode' => "/\\\$GLOBALS\\['DEBUG'\\]\\s*=\\s*(true|false)/"
    ];
    
    // String mappings (need quotes)
    $stringMappings = [
        'apiKey' => '/\$apiKey\s*=\s*\'[^\']*\'/',
        'rdToken' => '/\$PRIVATE_TOKEN\s*=\s*\'[^\']*\'/',
        'premiumizeKey' => '/\$premiumizeApiKey\s*=\s*\'[^\']*\'/',
        'language' => '/\$language\s*=\s*\'[^\']*\'/',
        'seriesCountry' => '/\$series_with_origin_country\s*=\s*\'[^\']*\'/',
        'moviesCountry' => '/\$movies_with_origin_country\s*=\s*\'[^\']*\'/',
        'userSetHost' => '/\$userSetHost\s*=\s*\'[^\']*\'/',
        'torrentioProviders' => "/\\\$GLOBALS\\['TORRENTIO_PROVIDERS'\\]\\s*=\\s*'[^']*'/"
    ];
    
    // Process simple numeric mappings
    foreach ($simpleMappings as $key => $pattern) {
        if (isset($settings[$key])) {
            $value = intval($settings[$key]);
            $varName = $key;
            if ($key === 'autoCacheInterval') $varName = 'autoCacheIntervalHours';
            $content = preg_replace($pattern, "\$$varName = $value", $content);
        }
    }
    
    // Process boolean mappings
    foreach ($boolMappings as $key => $pattern) {
        if (isset($settings[$key])) {
            $value = $settings[$key] ? 'true' : 'false';
            $varName = $key;
            if ($key === 'includeAdult') $varName = 'INCLUDE_ADULT_VOD';
            
            if ($key === 'debugMode') {
                $content = preg_replace($pattern, "\$GLOBALS['DEBUG'] = $value", $content);
            } else {
                $content = preg_replace($pattern, "\$$varName = $value", $content);
            }
        }
    }
    
    // Process string mappings
    foreach ($stringMappings as $key => $pattern) {
        if (isset($settings[$key])) {
            $value = str_replace("'", "\\'", $settings[$key]);
            $varName = $key;
            if ($key === 'rdToken') $varName = 'PRIVATE_TOKEN';
            if ($key === 'premiumizeKey') $varName = 'premiumizeApiKey';
            if ($key === 'seriesCountry') $varName = 'series_with_origin_country';
            if ($key === 'moviesCountry') $varName = 'movies_with_origin_country';
            
            if ($key === 'torrentioProviders') {
                $content = preg_replace($pattern, "\$GLOBALS['TORRENTIO_PROVIDERS'] = '$value'", $content);
            } else {
                $content = preg_replace($pattern, "\$$varName = '$value'", $content);
            }
        }
    }
    
    // Handle M3U8 limit (GLOBALS)
    if (isset($settings['m3u8Limit'])) {
        $limit = intval($settings['m3u8Limit']);
        $content = preg_replace(
            "/\\\$GLOBALS\['M3U8_LIMIT'\]\s*=\s*\d+/",
            "\$GLOBALS['M3U8_LIMIT'] = $limit",
            $content
        );
    }
    
    // Handle Stream Providers array
    if (isset($settings['streamProviders']) && is_array($settings['streamProviders'])) {
        $providers = array_map(function($p) { return "'$p'"; }, $settings['streamProviders']);
        $providersStr = implode(', ', $providers);
        $content = preg_replace(
            "/\\\$GLOBALS\['STREAM_PROVIDERS'\]\s*=\s*\[[^\]]*\]/",
            "\$GLOBALS['STREAM_PROVIDERS'] = [$providersStr]",
            $content
        );
    }
    
    // Handle Comet Indexers array
    if (isset($settings['cometIndexers']) && is_array($settings['cometIndexers'])) {
        $indexers = array_map(function($i) { return "'$i'"; }, $settings['cometIndexers']);
        $indexersStr = implode(', ', $indexers);
        $content = preg_replace(
            "/\\\$GLOBALS\['COMET_INDEXERS'\]\s*=\s*\[[^\]]*\]/",
            "\$GLOBALS['COMET_INDEXERS'] = [$indexersStr]",
            $content
        );
    }
    
    // Handle MediaFusion enabled (GLOBALS)
    if (isset($settings['mediafusionEnabled'])) {
        $value = $settings['mediafusionEnabled'] ? 'true' : 'false';
        $content = preg_replace(
            "/\\\$GLOBALS\['MEDIAFUSION_ENABLED'\]\s*=\s*(true|false)/",
            "\$GLOBALS['MEDIAFUSION_ENABLED'] = $value",
            $content
        );
    }
    
    return file_put_contents($configFile, $content) !== false;
}

function updateFilterConfig($filters) {
    $configFile = __DIR__ . '/config.php';
    $content = file_get_contents($configFile);
    
    // Escape single quotes in the values
    $releaseGroups = str_replace("'", "\\'", $filters['releaseGroups'] ?? '');
    $languages = str_replace("'", "\\'", $filters['languages'] ?? '');
    $qualities = str_replace("'", "\\'", $filters['qualities'] ?? '');
    $custom = str_replace("'", "\\'", $filters['custom'] ?? '');
    $enabled = ($filters['enabled'] ?? true) ? 'true' : 'false';
    
    // Update each filter setting
    $content = preg_replace(
        "/\\\$GLOBALS\['EXCLUDED_RELEASE_GROUPS'\]\s*=\s*'[^']*';/",
        "\$GLOBALS['EXCLUDED_RELEASE_GROUPS'] = '$releaseGroups';",
        $content
    );
    
    $content = preg_replace(
        "/\\\$GLOBALS\['EXCLUDED_LANGUAGES'\]\s*=\s*'[^']*';/",
        "\$GLOBALS['EXCLUDED_LANGUAGES'] = '$languages';",
        $content
    );
    
    $content = preg_replace(
        "/\\\$GLOBALS\['EXCLUDED_QUALITIES'\]\s*=\s*'[^']*';/",
        "\$GLOBALS['EXCLUDED_QUALITIES'] = '$qualities';",
        $content
    );
    
    $content = preg_replace(
        "/\\\$GLOBALS\['EXCLUDED_CUSTOM'\]\s*=\s*'[^']*';/",
        "\$GLOBALS['EXCLUDED_CUSTOM'] = '$custom';",
        $content
    );
    
    $content = preg_replace(
        "/\\\$GLOBALS\['ENABLE_RELEASE_FILTERS'\]\s*=\s*(true|false)/",
        "\$GLOBALS['ENABLE_RELEASE_FILTERS'] = $enabled",
        $content
    );
    
    return file_put_contents($configFile, $content) !== false;
}

/**
 * Update MDBList configuration in config.php
 */
function updateMDBListConfig($settings) {
    $configFile = __DIR__ . '/config.php';
    $content = file_get_contents($configFile);
    
    // Update API key
    if (isset($settings['apiKey'])) {
        $apiKey = str_replace("'", "\\'", $settings['apiKey']);
        $content = preg_replace(
            "/\\\$GLOBALS\['MDBLIST_API_KEY'\]\s*=\s*'[^']*';/",
            "\$GLOBALS['MDBLIST_API_KEY'] = '$apiKey';",
            $content
        );
    }
    
    // Update enabled status
    if (isset($settings['enabled'])) {
        $value = $settings['enabled'] ? 'true' : 'false';
        $content = preg_replace(
            "/\\\$GLOBALS\['MDBLIST_ENABLED'\]\s*=\s*(true|false);/",
            "\$GLOBALS['MDBLIST_ENABLED'] = $value;",
            $content
        );
    }
    
    // Update sync interval
    if (isset($settings['syncInterval'])) {
        $interval = intval($settings['syncInterval']);
        $content = preg_replace(
            "/\\\$GLOBALS\['MDBLIST_SYNC_INTERVAL'\]\s*=\s*\d+;/",
            "\$GLOBALS['MDBLIST_SYNC_INTERVAL'] = $interval;",
            $content
        );
    }
    
    // Update merge playlist setting
    if (isset($settings['mergePlaylist'])) {
        $value = $settings['mergePlaylist'] ? 'true' : 'false';
        $content = preg_replace(
            "/\\\$GLOBALS\['MDBLIST_MERGE_PLAYLIST'\]\s*=\s*(true|false);/",
            "\$GLOBALS['MDBLIST_MERGE_PLAYLIST'] = $value;",
            $content
        );
    }
    
    return file_put_contents($configFile, $content) !== false;
}

function formatBytes($bytes) {
    $units = ['B', 'KB', 'MB', 'GB', 'TB'];
    $bytes = max($bytes, 0);
    $pow = floor(($bytes ? log($bytes) : 0) / log(1024));
    $pow = min($pow, count($units) - 1);
    return round($bytes / (1024 ** $pow), 2) . ' ' . $units[$pow];
}

// HTML Dashboard
?>
<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TMDB-VOD Admin</title>
    <style>
        :root {
            --bg-primary: #1a1d21;
            --bg-secondary: #22262b;
            --bg-tertiary: #2a2f35;
            --text-primary: #ffffff;
            --text-secondary: #8e9297;
            --accent: #3498db;
            --accent-hover: #2980b9;
            --success: #2ecc71;
            --warning: #f39c12;
            --danger: #e74c3c;
            --border: #3a3f44;
        }
        
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            min-height: 100vh;
        }
        
        .login-container {
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
        }
        
        .login-box {
            background: var(--bg-secondary);
            padding: 2rem;
            border-radius: 8px;
            width: 100%;
            max-width: 400px;
        }
        
        .login-box h1 {
            margin-bottom: 1.5rem;
            text-align: center;
        }
        
        .login-box input {
            width: 100%;
            padding: 0.75rem;
            margin-bottom: 1rem;
            border: 1px solid var(--border);
            border-radius: 4px;
            background: var(--bg-tertiary);
            color: var(--text-primary);
        }
        
        .sidebar {
            position: fixed;
            left: 0;
            top: 0;
            width: 220px;
            height: 100vh;
            background: var(--bg-secondary);
            border-right: 1px solid var(--border);
            padding: 1rem 0;
        }
        
        .sidebar-header {
            padding: 0 1rem 1rem;
            border-bottom: 1px solid var(--border);
            margin-bottom: 1rem;
        }
        
        .sidebar-header h1 {
            font-size: 1.2rem;
            color: var(--accent);
        }
        
        .sidebar-header span {
            font-size: 0.75rem;
            color: var(--text-secondary);
        }
        
        .nav-item {
            display: flex;
            align-items: center;
            padding: 0.75rem 1rem;
            color: var(--text-secondary);
            text-decoration: none;
            cursor: pointer;
            transition: all 0.2s;
        }
        
        .nav-item:hover, .nav-item.active {
            background: var(--bg-tertiary);
            color: var(--text-primary);
        }
        
        .nav-item svg {
            width: 20px;
            height: 20px;
            margin-right: 0.75rem;
        }
        
        .main-content {
            margin-left: 220px;
            padding: 1.5rem;
        }
        
        .page-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.5rem;
        }
        
        .page-header h2 {
            font-size: 1.5rem;
        }
        
        .card {
            background: var(--bg-secondary);
            border-radius: 8px;
            padding: 1.25rem;
            margin-bottom: 1rem;
        }
        
        .card-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1rem;
        }
        
        .card-title {
            font-size: 1rem;
            font-weight: 600;
        }
        
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
        }
        
        .stat-card {
            background: var(--bg-tertiary);
            border-radius: 6px;
            padding: 1rem;
        }
        
        .stat-value {
            font-size: 1.75rem;
            font-weight: 700;
            color: var(--accent);
        }
        
        .stat-label {
            font-size: 0.85rem;
            color: var(--text-secondary);
            margin-top: 0.25rem;
        }
        
        .btn {
            padding: 0.5rem 1rem;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.875rem;
            font-weight: 500;
            transition: all 0.2s;
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
        }
        
        .btn-primary {
            background: var(--accent);
            color: white;
        }
        
        .btn-primary:hover {
            background: var(--accent-hover);
        }
        
        .btn-success {
            background: var(--success);
            color: white;
        }
        
        .btn-danger {
            background: var(--danger);
            color: white;
        }
        
        .btn-secondary {
            background: var(--bg-tertiary);
            color: var(--text-primary);
            border: 1px solid var(--border);
        }
        
        .status-badge {
            display: inline-flex;
            align-items: center;
            padding: 0.25rem 0.75rem;
            border-radius: 20px;
            font-size: 0.75rem;
            font-weight: 600;
        }
        
        .status-running {
            background: rgba(46, 204, 113, 0.2);
            color: var(--success);
        }
        
        .status-stopped {
            background: rgba(231, 76, 60, 0.2);
            color: var(--danger);
        }
        
        .status-warning {
            background: rgba(243, 156, 18, 0.2);
            color: var(--warning);
        }
        
        .daemon-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 1rem;
            background: var(--bg-tertiary);
            border-radius: 6px;
            margin-bottom: 0.5rem;
        }
        
        .daemon-info h4 {
            margin-bottom: 0.25rem;
        }
        
        .daemon-info p {
            font-size: 0.85rem;
            color: var(--text-secondary);
        }
        
        .daemon-actions {
            display: flex;
            gap: 0.5rem;
        }
        
        .provider-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 0.75rem;
            background: var(--bg-tertiary);
            border-radius: 6px;
            margin-bottom: 0.5rem;
        }
        
        .provider-status {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        
        .log-viewer {
            background: #0d1117;
            border-radius: 6px;
            padding: 1rem;
            font-family: 'Monaco', 'Menlo', monospace;
            font-size: 0.8rem;
            max-height: 400px;
            overflow-y: auto;
            white-space: pre-wrap;
            word-break: break-all;
        }
        
        .log-line {
            padding: 0.1rem 0;
            border-bottom: 1px solid #21262d;
        }
        
        .settings-form {
            display: grid;
            gap: 1rem;
        }
        
        .form-group {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
        }
        
        .form-group label {
            font-size: 0.875rem;
            color: var(--text-secondary);
        }
        
        .form-group input, .form-group select {
            padding: 0.5rem;
            border: 1px solid var(--border);
            border-radius: 4px;
            background: var(--bg-tertiary);
            color: var(--text-primary);
        }
        
        .form-row {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
        }
        
        .toggle-switch {
            position: relative;
            width: 50px;
            height: 26px;
        }
        
        .toggle-switch input {
            opacity: 0;
            width: 0;
            height: 0;
        }
        
        .toggle-slider {
            position: absolute;
            cursor: pointer;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: var(--bg-tertiary);
            border-radius: 26px;
            transition: 0.3s;
        }
        
        .toggle-slider:before {
            position: absolute;
            content: "";
            height: 20px;
            width: 20px;
            left: 3px;
            bottom: 3px;
            background: white;
            border-radius: 50%;
            transition: 0.3s;
        }
        
        input:checked + .toggle-slider {
            background: var(--accent);
        }
        
        input:checked + .toggle-slider:before {
            transform: translateX(24px);
        }
        
        .hidden {
            display: none;
        }
        
        .refresh-indicator {
            animation: spin 1s linear infinite;
        }
        
        @keyframes spin {
            from { transform: rotate(0deg); }
            to { transform: rotate(360deg); }
        }
        
        .toast {
            position: fixed;
            bottom: 20px;
            right: 20px;
            padding: 1rem 1.5rem;
            border-radius: 6px;
            color: white;
            font-weight: 500;
            z-index: 1000;
            animation: slideIn 0.3s ease;
        }
        
        .toast-success { background: var(--success); }
        .toast-error { background: var(--danger); }
        
        @keyframes slideIn {
            from { transform: translateX(100%); opacity: 0; }
            to { transform: translateX(0); opacity: 1; }
        }
        
        /* Modal styles */
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.7);
            z-index: 1000;
            justify-content: center;
            align-items: center;
        }
        
        .modal.show {
            display: flex;
        }
        
        .modal-content {
            background: var(--bg-secondary);
            border-radius: 8px;
            width: 90%;
            max-width: 500px;
            max-height: 90vh;
            overflow: hidden;
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
        }
        
        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 1rem 1.5rem;
            border-bottom: 1px solid var(--border);
        }
        
        .modal-header h3 {
            margin: 0;
            font-size: 1.1rem;
        }
        
        .modal-close {
            background: none;
            border: none;
            color: var(--text-secondary);
            font-size: 1.5rem;
            cursor: pointer;
            padding: 0;
            line-height: 1;
        }
        
        .modal-close:hover {
            color: var(--text-primary);
        }
        
        .modal-body {
            padding: 1.5rem;
            overflow-y: auto;
            max-height: calc(90vh - 60px);
        }
    </style>
</head>
<body>

<?php if (!$isAuthenticated): ?>
<!-- Login Form -->
<div class="login-container">
    <div class="login-box">
        <h1>🎬 TMDB-VOD Admin</h1>
        <form method="POST">
            <input type="password" name="password" placeholder="Enter admin password" autofocus>
            <button type="submit" class="btn btn-primary" style="width: 100%">Login</button>
        </form>
        <p style="margin-top: 1rem; font-size: 0.8rem; color: var(--text-secondary); text-align: center;">
            Default password: admin123
        </p>
    </div>
</div>

<?php else: ?>
<!-- Dashboard -->
<nav class="sidebar">
    <div class="sidebar-header">
        <h1>🎬 TMDB-VOD</h1>
        <span>Admin Dashboard</span>
    </div>
    
    <a class="nav-item active" data-page="dashboard">
        <svg fill="currentColor" viewBox="0 0 20 20"><path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"></path></svg>
        Dashboard
    </a>
    
    <a class="nav-item" data-page="services">
        <svg fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd"></path></svg>
        Services
    </a>
    
    <a class="nav-item" data-page="logs">
        <svg fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clip-rule="evenodd"></path></svg>
        Logs
    </a>
    
    <a class="nav-item" data-page="filters">
        <svg fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M3 3a1 1 0 011-1h12a1 1 0 011 1v3a1 1 0 01-.293.707L12 11.414V15a1 1 0 01-.293.707l-2 2A1 1 0 018 17v-5.586L3.293 6.707A1 1 0 013 6V3z" clip-rule="evenodd"></path></svg>
        Filters
    </a>
    
    <a class="nav-item" data-page="lists">
        <svg fill="currentColor" viewBox="0 0 20 20"><path d="M3 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1z"></path></svg>
        Lists
    </a>
    
    <a class="nav-item" data-page="settings">
        <svg fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd"></path></svg>
        Settings
    </a>
    
    <a class="nav-item" href="media_browser.php">
        <svg fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M4 3a2 2 0 00-2 2v10a2 2 0 002 2h12a2 2 0 002-2V5a2 2 0 00-2-2H4zm3 2h6v4H7V5zm8 8v2h1v-2h-1zm-2-2H7v4h6v-4zm2 0h1V9h-1v2zm1-4V5h-1v2h1zM5 5v2H4V5h1zm0 4H4v2h1V9zm-1 4h1v2H4v-2z" clip-rule="evenodd"></path></svg>
        Media Browser
    </a>
    
    <a class="nav-item" href="?logout=1" style="margin-top: auto; position: absolute; bottom: 1rem; width: calc(100% - 2rem); margin: 0 1rem;">
        <svg fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M3 3a1 1 0 00-1 1v12a1 1 0 102 0V4a1 1 0 00-1-1zm10.293 9.293a1 1 0 001.414 1.414l3-3a1 1 0 000-1.414l-3-3a1 1 0 10-1.414 1.414L14.586 9H7a1 1 0 100 2h7.586l-1.293 1.293z" clip-rule="evenodd"></path></svg>
        Logout
    </a>
</nav>

<main class="main-content">
    <!-- Dashboard Page -->
    <div id="page-dashboard" class="page">
        <div class="page-header">
            <h2>Dashboard</h2>
            <button class="btn btn-secondary" onclick="refreshStatus()">
                <svg id="refresh-icon" width="16" height="16" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path></svg>
                Refresh
            </button>
        </div>
        
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value" id="stat-movies">-</div>
                <div class="stat-label">Movies</div>
                <div class="stat-sub" id="stat-mdb-movies" style="font-size: 0.7rem; color: var(--accent-primary); margin-top: 0.25rem;"></div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="stat-series">-</div>
                <div class="stat-label">TV Series</div>
                <div class="stat-sub" id="stat-mdb-series" style="font-size: 0.7rem; color: var(--accent-primary); margin-top: 0.25rem;"></div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="stat-episodes">-</div>
                <div class="stat-label">Cached Episodes</div>
                <button class="btn btn-danger" style="margin-top: 0.5rem; padding: 0.25rem 0.5rem; font-size: 0.75rem;" onclick="clearEpisodeCache()">Clear</button>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="stat-m3u">-</div>
                <div class="stat-label">M3U8 Entries</div>
                <button class="btn btn-danger" style="margin-top: 0.5rem; padding: 0.25rem 0.5rem; font-size: 0.75rem;" onclick="clearM3U8()">Clear</button>
            </div>
        </div>
        
        <div class="card" style="margin-top: 1rem;">
            <div class="card-header">
                <span class="card-title">Quick Actions</span>
            </div>
            <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
                <button class="btn btn-primary" onclick="runMDBListSyncAndMerge()">
                    <svg width="16" height="16" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1z" clip-rule="evenodd"></path></svg>
                    Sync MDBList
                </button>
                <button class="btn btn-secondary" onclick="regenerateM3U8()">
                    📺 Regenerate M3U8
                </button>
                <button class="btn btn-secondary" onclick="regenerateLiveTV()">
                    📡 Refresh Live TV
                </button>
            </div>
        </div>
        
        <div class="card">
            <div class="card-header">
                <span class="card-title">System Info</span>
            </div>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-label">Last Sync</div>
                    <div id="stat-last-sync" style="font-weight: 600;">-</div>
                </div>
                <div class="stat-card">
                    <div class="stat-label">Next Sync</div>
                    <div id="stat-next-sync" style="font-weight: 600;">-</div>
                </div>
                <div class="stat-card">
                    <div class="stat-label">Disk Free</div>
                    <div id="stat-disk" style="font-weight: 600;">-</div>
                </div>
                <div class="stat-card">
                    <div class="stat-label">PHP Version</div>
                    <div id="stat-php" style="font-weight: 600;">-</div>
                </div>
            </div>
        </div>
    </div>
    
    <!-- Services Page -->
    <div id="page-services" class="page hidden">
        <div class="page-header">
            <h2>Services</h2>
            <button class="btn btn-secondary" onclick="refreshServiceStatus()">🔄 Refresh Status</button>
        </div>
        
        <!-- MDBList Sync Service -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">📋 MDBList Sync Service</span>
            </div>
            
            <div class="daemon-row">
                <div class="daemon-info">
                    <h4>MDBList Content Sync</h4>
                    <p>Fetches movies & TV series from your configured MDBList sources and merges them into your library</p>
                    <div style="margin-top: 0.75rem; display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 0.5rem;">
                        <div style="background: var(--bg-tertiary); padding: 0.5rem; border-radius: 4px; text-align: center;">
                            <div style="font-size: 1.25rem; font-weight: bold; color: var(--primary);" id="svc-mdblist-movies">-</div>
                            <div style="font-size: 0.75rem; color: var(--text-secondary);">Movies</div>
                        </div>
                        <div style="background: var(--bg-tertiary); padding: 0.5rem; border-radius: 4px; text-align: center;">
                            <div style="font-size: 1.25rem; font-weight: bold; color: var(--success);" id="svc-mdblist-series">-</div>
                            <div style="font-size: 0.75rem; color: var(--text-secondary);">TV Series</div>
                        </div>
                        <div style="background: var(--bg-tertiary); padding: 0.5rem; border-radius: 4px; text-align: center;">
                            <div style="font-size: 1.25rem; font-weight: bold; color: var(--warning);" id="svc-mdblist-lists">-</div>
                            <div style="font-size: 0.75rem; color: var(--text-secondary);">Active Lists</div>
                        </div>
                        <div style="background: var(--bg-tertiary); padding: 0.5rem; border-radius: 4px; text-align: center;">
                            <div style="font-size: 0.85rem; color: var(--text-primary);" id="svc-mdblist-lastsync">Never</div>
                            <div style="font-size: 0.75rem; color: var(--text-secondary);">Last Sync</div>
                        </div>
                    </div>
                </div>
                <div class="daemon-actions" style="display: flex; flex-direction: column; gap: 0.5rem;">
                    <button class="btn btn-primary" onclick="runMDBListSync()">🔄 Sync Now</button>
                    <button class="btn btn-success" onclick="runMDBListMerge()">📥 Merge to Library</button>
                    <button class="btn btn-secondary" onclick="runMDBListSyncAndMerge()">⚡ Sync + Merge</button>
                </div>
            </div>
        </div>
        
        <!-- Playlist Regeneration -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">📺 Playlist Regeneration</span>
            </div>
            
            <div class="daemon-row">
                <div class="daemon-info">
                    <h4>M3U8 Playlist Generator</h4>
                    <p>Regenerates the M3U8 playlist file from your movie/TV library. Run after adding new content.</p>
                    <div style="margin-top: 0.75rem; display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 0.5rem;">
                        <div style="background: var(--bg-tertiary); padding: 0.5rem; border-radius: 4px; text-align: center;">
                            <div style="font-size: 1.25rem; font-weight: bold; color: var(--primary);" id="svc-m3u8-movies">-</div>
                            <div style="font-size: 0.75rem; color: var(--text-secondary);">Movies</div>
                        </div>
                        <div style="background: var(--bg-tertiary); padding: 0.5rem; border-radius: 4px; text-align: center;">
                            <div style="font-size: 1.25rem; font-weight: bold; color: var(--success);" id="svc-m3u8-series">-</div>
                            <div style="font-size: 0.75rem; color: var(--text-secondary);">TV Series</div>
                        </div>
                        <div style="background: var(--bg-tertiary); padding: 0.5rem; border-radius: 4px; text-align: center;">
                            <div style="font-size: 1.25rem; font-weight: bold; color: var(--warning);" id="svc-m3u8-livetv">-</div>
                            <div style="font-size: 0.75rem; color: var(--text-secondary);">Live TV</div>
                        </div>
                        <div style="background: var(--bg-tertiary); padding: 0.5rem; border-radius: 4px; text-align: center;">
                            <div style="font-size: 0.85rem; color: var(--text-primary);" id="svc-m3u8-lastgen">Unknown</div>
                            <div style="font-size: 0.75rem; color: var(--text-secondary);">Last Generated</div>
                        </div>
                    </div>
                </div>
                <div class="daemon-actions" style="display: flex; flex-direction: column; gap: 0.5rem;">
                    <button class="btn btn-primary" onclick="regenerateM3U8()">🔄 Regenerate M3U8</button>
                    <button class="btn btn-secondary" onclick="regenerateLiveTV()">📡 Refresh Live TV</button>
                </div>
            </div>
        </div>
        
        <!-- Background Daemons -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">⚙️ Background Daemons</span>
            </div>
            
            <div class="daemon-row">
                <div class="daemon-info">
                    <h4>Background Sync Daemon</h4>
                    <p>Syncs Live TV and regenerates M3U8 playlist periodically</p>
                    <p style="margin-top: 0.5rem;">
                        Status: <span id="daemon-sync-status" class="status-badge status-stopped">Stopped</span>
                        <span id="daemon-sync-pid" style="margin-left: 0.5rem; font-size: 0.8rem; color: var(--text-secondary);"></span>
                    </p>
                </div>
                <div class="daemon-actions">
                    <button class="btn btn-success" id="btn-start-daemon" onclick="controlDaemon('start')">Start</button>
                    <button class="btn btn-danger" id="btn-stop-daemon" onclick="controlDaemon('stop')">Stop</button>
                </div>
            </div>
            
            <div class="daemon-row">
                <div class="daemon-info">
                    <h4>Episode Cache Daemon</h4>
                    <p>Pre-caches episode information for TV series</p>
                    <p style="margin-top: 0.5rem;">
                        Status: <span id="daemon-cache-status" class="status-badge status-stopped">Stopped</span>
                    </p>
                </div>
                <div class="daemon-actions">
                    <button class="btn btn-primary" onclick="runEpisodeCache()">▶️ Run Now</button>
                </div>
            </div>
        </div>
        
        <!-- Cron Jobs Setup -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">⏰ Automated Tasks (Cron Jobs)</span>
            </div>
            
            <div style="background: var(--bg-tertiary); padding: 1rem; border-radius: 6px; margin-bottom: 1rem;">
                <p style="margin-bottom: 0.75rem; color: var(--text-secondary); font-size: 0.9rem;">
                    Set up these cron jobs on your server for automatic syncing. SSH into your server and run <code>crontab -e</code> to add them:
                </p>
                
                <div style="margin-bottom: 1rem;">
                    <label style="font-weight: 600; display: block; margin-bottom: 0.5rem;">📋 MDBList Sync (Every 6 hours)</label>
                    <div style="background: var(--bg-secondary); padding: 0.75rem; border-radius: 4px; font-family: monospace; font-size: 0.8rem; overflow-x: auto; position: relative;">
                        <code id="cron-mdblist">0 */6 * * * cd /var/www/streaming && php -r "require 'config.php'; require 'libs/mdblist.php'; \$m = new MDBListProvider(); \$r = \$m->fetchAllConfiguredLists(); if(\$r['success']) { file_put_contents('cache/mdblist_items.json', json_encode(\$r)); }" >> logs/mdblist_sync.log 2>&1</code>
                        <button onclick="copyCronCommand('cron-mdblist')" style="position: absolute; right: 8px; top: 8px; background: var(--primary); color: white; border: none; padding: 0.25rem 0.5rem; border-radius: 4px; cursor: pointer; font-size: 0.7rem;">Copy</button>
                    </div>
                </div>
                
                <div style="margin-bottom: 1rem;">
                    <label style="font-weight: 600; display: block; margin-bottom: 0.5rem;">🔄 Background Sync Daemon (Every 6 hours)</label>
                    <div style="background: var(--bg-secondary); padding: 0.75rem; border-radius: 4px; font-family: monospace; font-size: 0.8rem; overflow-x: auto; position: relative;">
                        <code id="cron-sync">0 */6 * * * cd /var/www/streaming && php daemons/background_sync_daemon.php >> logs/sync_daemon.log 2>&1</code>
                        <button onclick="copyCronCommand('cron-sync')" style="position: absolute; right: 8px; top: 8px; background: var(--primary); color: white; border: none; padding: 0.25rem 0.5rem; border-radius: 4px; cursor: pointer; font-size: 0.7rem;">Copy</button>
                    </div>
                </div>
                
                <div style="margin-bottom: 1rem;">
                    <label style="font-weight: 600; display: block; margin-bottom: 0.5rem;">📺 Episode Cache (Daily at 4 AM)</label>
                    <div style="background: var(--bg-secondary); padding: 0.75rem; border-radius: 4px; font-family: monospace; font-size: 0.8rem; overflow-x: auto; position: relative;">
                        <code id="cron-cache">0 4 * * * cd /var/www/streaming && php daemons/auto_cache_daemon.php >> logs/auto_cache.log 2>&1</code>
                        <button onclick="copyCronCommand('cron-cache')" style="position: absolute; right: 8px; top: 8px; background: var(--primary); color: white; border: none; padding: 0.25rem 0.5rem; border-radius: 4px; cursor: pointer; font-size: 0.7rem;">Copy</button>
                    </div>
                </div>
                
                <div>
                    <label style="font-weight: 600; display: block; margin-bottom: 0.5rem;">⚡ Full Auto-Sync (MDBList + Merge + M3U8 - Every 6 hours)</label>
                    <div style="background: var(--bg-secondary); padding: 0.75rem; border-radius: 4px; font-family: monospace; font-size: 0.8rem; overflow-x: auto; position: relative;">
                        <code id="cron-full">0 */6 * * * cd /var/www/streaming && php -r "require 'admin.php';" "api=mdblist-sync-and-merge" >> logs/auto_sync.log 2>&1</code>
                        <button onclick="copyCronCommand('cron-full')" style="position: absolute; right: 8px; top: 8px; background: var(--primary); color: white; border: none; padding: 0.25rem 0.5rem; border-radius: 4px; cursor: pointer; font-size: 0.7rem;">Copy</button>
                    </div>
                </div>
            </div>
            
            <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
                <button class="btn btn-primary" onclick="checkCronStatus()">🔍 Check Cron Status</button>
                <button class="btn btn-secondary" onclick="showCronInstructions()">📖 Setup Instructions</button>
            </div>
            
            <div id="cron-status-output" style="margin-top: 1rem; display: none;">
                <pre style="background: var(--bg-tertiary); padding: 1rem; border-radius: 6px; font-size: 0.8rem; overflow-x: auto; max-height: 200px;"></pre>
            </div>
        </div>
    </div>
    
    <!-- Logs Page -->
    <div id="page-logs" class="page hidden">
        <div class="page-header">
            <h2>Logs</h2>
            <select id="log-select" onchange="loadLogs()" style="padding: 0.5rem; background: var(--bg-tertiary); color: var(--text-primary); border: 1px solid var(--border); border-radius: 4px;">
                <option value="sync_daemon">Sync Daemon</option>
                <option value="auto_cache">Auto Cache</option>
                <option value="requests">Requests</option>
            </select>
        </div>
        
        <div class="card">
            <div class="log-viewer" id="log-content">
                Select a log file to view...
            </div>
        </div>
    </div>
    
    <!-- Settings Page -->
    <div id="page-settings" class="page hidden">
        <div class="page-header">
            <h2>Settings</h2>
            <button class="btn btn-primary" onclick="saveAllSettings()">
                <svg width="16" height="16" fill="currentColor" viewBox="0 0 20 20"><path d="M7.707 10.293a1 1 0 10-1.414 1.414l3 3a1 1 0 001.414 0l3-3a1 1 0 00-1.414-1.414L11 11.586V6h5a2 2 0 012 2v7a2 2 0 01-2 2H4a2 2 0 01-2-2V8a2 2 0 012-2h5v5.586l-1.293-1.293z"></path></svg>
                Save All Settings
            </button>
        </div>
        
        <!-- API Keys Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">🔑 API Keys & Tokens</span>
            </div>
            
            <div class="settings-form">
                <div class="form-group">
                    <label>TMDB API Key</label>
                    <input type="text" id="setting-apiKey" placeholder="Enter your TMDB API key" style="font-family: monospace;">
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">
                        Get your free API key from <a href="https://www.themoviedb.org/settings/api" target="_blank" style="color: var(--primary);">themoviedb.org</a>
                    </span>
                </div>
                
                <div class="form-group">
                    <label>Real-Debrid API Token</label>
                    <div style="display: flex; gap: 0.5rem;">
                        <input type="password" id="setting-rdToken" placeholder="Enter your Real-Debrid private token" style="font-family: monospace; flex: 1;">
                        <button type="button" class="btn btn-secondary" onclick="togglePassword('setting-rdToken')" style="padding: 0.5rem;">👁</button>
                    </div>
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">
                        Get your token from <a href="https://real-debrid.com/apitoken" target="_blank" style="color: var(--primary);">real-debrid.com/apitoken</a>
                    </span>
                </div>
                
                <div class="form-group">
                    <label>Premiumize API Key (Optional)</label>
                    <div style="display: flex; gap: 0.5rem;">
                        <input type="password" id="setting-premiumizeKey" placeholder="Enter your Premiumize API key" style="font-family: monospace; flex: 1;">
                        <button type="button" class="btn btn-secondary" onclick="togglePassword('setting-premiumizeKey')" style="padding: 0.5rem;">👁</button>
                    </div>
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">
                        Get your key from <a href="https://www.premiumize.me/account" target="_blank" style="color: var(--primary);">premiumize.me/account</a>
                    </span>
                </div>
            </div>
        </div>
        
        <!-- Debrid Services Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">⚡ Debrid Services</span>
            </div>
            
            <div class="settings-form">
                <div class="form-row">
                    <div class="form-group">
                        <label>Use Real-Debrid</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-useRealDebrid">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">Enable Real-Debrid for cached torrents</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Use Premiumize</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-usePremiumize">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">Enable Premiumize as alternative</span>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Playlist Settings Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">⚙️ Playlist & Sync Settings</span>
            </div>
            
            <div style="background: var(--bg-tertiary); padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.85rem; color: var(--text-secondary);">
                💡 Use <strong>MDBList Integration</strong> (Lists page) to build your movie and TV series library from curated lists.
            </div>
            
            <div class="settings-form">
                <div class="form-row">
                    <div class="form-group">
                        <label>Max Resolution</label>
                        <select id="setting-maxResolution">
                            <option value="2160">4K (2160p)</option>
                            <option value="1080">1080p</option>
                            <option value="720">720p</option>
                            <option value="480">480p</option>
                        </select>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">Preferred stream quality</span>
                    </div>
                    
                    <div class="form-group">
                        <label>M3U8 Playlist Limit</label>
                        <select id="setting-m3u8Limit">
                            <option value="0">Unlimited (all items)</option>
                            <option value="1000">1,000 items</option>
                            <option value="2500">2,500 items</option>
                            <option value="5000">5,000 items</option>
                            <option value="10000">10,000 items</option>
                            <option value="25000">25,000 items</option>
                            <option value="50000">50,000 items</option>
                        </select>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">Limit M3U8 size for IPTV apps</span>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>Auto-Sync Interval</label>
                        <select id="setting-autoCacheInterval">
                            <option value="0">Disabled</option>
                            <option value="1">Every 1 Hour</option>
                            <option value="3">Every 3 Hours</option>
                            <option value="6" selected>Every 6 Hours</option>
                            <option value="12">Every 12 Hours</option>
                            <option value="24">Every 24 Hours</option>
                        </select>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">How often to sync episode cache</span>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Additional Content Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">📦 Additional Content</span>
            </div>
            
            <div class="settings-form">
                <div class="form-row">
                    <div class="form-group">
                        <label>Include Live TV</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeLiveTV">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">📡 427 free Pluto TV channels (US)</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Include Adult Content</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeAdult">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">🔞 ~10,000 adult VOD movies</span>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>Debug Mode</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-debugMode">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">🐛 Show detailed errors in logs</span>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Regional Settings Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">🌍 Regional Settings</span>
            </div>
            
            <div class="settings-form">
                <div class="form-row">
                    <div class="form-group">
                        <label>Language</label>
                        <select id="setting-language">
                            <option value="en-US">English (US)</option>
                            <option value="en-GB">English (UK)</option>
                            <option value="es-ES">Spanish</option>
                            <option value="fr-FR">French</option>
                            <option value="de-DE">German</option>
                            <option value="it-IT">Italian</option>
                            <option value="pt-BR">Portuguese (Brazil)</option>
                            <option value="ja-JP">Japanese</option>
                            <option value="ko-KR">Korean</option>
                            <option value="zh-CN">Chinese (Simplified)</option>
                        </select>
                    </div>
                    
                    <div class="form-group">
                        <label>Series Origin Country</label>
                        <input type="text" id="setting-seriesCountry" placeholder="US" maxlength="5">
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">Leave blank for all countries</span>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>Movies Origin Country</label>
                        <input type="text" id="setting-moviesCountry" placeholder="US" maxlength="5">
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">Leave blank for all countries</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Custom Server Host</label>
                        <input type="text" id="setting-userSetHost" placeholder="192.168.1.100">
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">For local network access</span>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Stream Providers Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">🔗 Stream Providers</span>
            </div>
            
            <div class="settings-form">
                <div class="form-group">
                    <label>Enable Providers (in priority order)</label>
                    <div style="display: flex; flex-direction: column; gap: 0.75rem; margin-top: 0.5rem;">
                        <label style="display: flex; align-items: center; gap: 0.5rem; background: var(--bg-tertiary); padding: 0.75rem 1rem; border-radius: 6px; cursor: pointer;">
                            <input type="checkbox" id="provider-comet" checked> 
                            <span style="font-weight: 500;">☄️ Comet</span>
                            <span style="font-size: 0.75rem; color: var(--text-secondary); margin-left: auto;">Best for datacenter IPs (Hetzner, etc.)</span>
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.5rem; background: var(--bg-tertiary); padding: 0.75rem 1rem; border-radius: 6px; cursor: pointer;">
                            <input type="checkbox" id="provider-mediafusion"> 
                            <span style="font-weight: 500;">🌐 MediaFusion</span>
                            <span style="font-size: 0.75rem; color: var(--text-secondary); margin-left: auto;">ElfHosted instance, datacenter-friendly</span>
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.5rem; background: var(--bg-tertiary); padding: 0.75rem 1rem; border-radius: 6px; cursor: pointer;">
                            <input type="checkbox" id="provider-torrentio"> 
                            <span style="font-weight: 500;">🔥 Torrentio</span>
                            <span style="font-size: 0.75rem; color: var(--text-secondary); margin-left: auto;">May be blocked on datacenter IPs</span>
                        </label>
                    </div>
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">First working provider is used. Drag to reorder priority.</span>
                </div>
                
                <div class="form-group" id="comet-settings" style="border-top: 1px solid var(--border-color); padding-top: 1rem; margin-top: 0.5rem;">
                    <label>☄️ Comet Indexers</label>
                    <div style="display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.5rem;">
                        <label style="display: flex; align-items: center; gap: 0.25rem; background: var(--bg-tertiary); padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer; font-size: 0.85rem;">
                            <input type="checkbox" id="comet-bktorrent" class="comet-indexer" value="bktorrent"> bktorrent
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.25rem; background: var(--bg-tertiary); padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer; font-size: 0.85rem;">
                            <input type="checkbox" id="comet-thepiratebay" class="comet-indexer" value="thepiratebay"> ThePirateBay
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.25rem; background: var(--bg-tertiary); padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer; font-size: 0.85rem;">
                            <input type="checkbox" id="comet-yts" class="comet-indexer" value="yts"> YTS
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.25rem; background: var(--bg-tertiary); padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer; font-size: 0.85rem;">
                            <input type="checkbox" id="comet-eztv" class="comet-indexer" value="eztv"> EZTV
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.25rem; background: var(--bg-tertiary); padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer; font-size: 0.85rem;">
                            <input type="checkbox" id="comet-kickasstorrents" class="comet-indexer" value="kickasstorrents"> KickassTorrents
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.25rem; background: var(--bg-tertiary); padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer; font-size: 0.85rem;">
                            <input type="checkbox" id="comet-torrentgalaxy" class="comet-indexer" value="torrentgalaxy"> TorrentGalaxy
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.25rem; background: var(--bg-tertiary); padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer; font-size: 0.85rem;">
                            <input type="checkbox" id="comet-nyaasi" class="comet-indexer" value="nyaasi"> Nyaa.si
                        </label>
                        <label style="display: flex; align-items: center; gap: 0.25rem; background: var(--bg-tertiary); padding: 0.4rem 0.75rem; border-radius: 4px; cursor: pointer; font-size: 0.85rem;">
                            <input type="checkbox" id="comet-1337x" class="comet-indexer" value="1337x"> 1337x
                        </label>
                    </div>
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">Select which torrent indexers Comet should search. All are international/English-focused.</span>
                </div>
                
                <div class="form-group" id="torrentio-settings" style="border-top: 1px solid var(--border-color); padding-top: 1rem; margin-top: 0.5rem;">
                    <label>🔥 Torrentio Providers</label>
                    <input type="text" id="setting-torrentioProviders" placeholder="yts,eztv,rarbg,1337x,thepiratebay,kickasstorrents,torrentgalaxy" style="font-family: monospace;">
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">Comma-separated list: yts, eztv, rarbg, 1337x, thepiratebay, kickasstorrents, torrentgalaxy, magnetdl, etc.</span>
                </div>
                
                <div class="form-group" style="border-top: 1px solid var(--border-color); padding-top: 1rem; margin-top: 0.5rem;">
                    <label>Test Providers</label>
                    <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
                        <button class="btn btn-secondary" onclick="testProvider('comet')">Test Comet</button>
                        <button class="btn btn-secondary" onclick="testProvider('mediafusion')">Test MediaFusion</button>
                        <button class="btn btn-secondary" onclick="testProvider('torrentio')">Test Torrentio</button>
                    </div>
                    <div id="provider-test-result" style="margin-top: 0.5rem; padding: 0.5rem; background: var(--bg-tertiary); border-radius: 4px; font-size: 0.85rem; display: none;"></div>
                </div>
            </div>
        </div>
        
        <!-- Connection Info Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">📡 Connection Info</span>
            </div>
            <div style="background: var(--bg-tertiary); padding: 1rem; border-radius: 6px; font-family: monospace;">
                <p><strong>Server URL:</strong> <?php echo (isset($_SERVER['HTTPS']) ? 'https' : 'http') . '://' . $_SERVER['HTTP_HOST']; ?></p>
                <p style="margin-top: 0.5rem;"><strong>Xtream API:</strong> <?php echo (isset($_SERVER['HTTPS']) ? 'https' : 'http') . '://' . $_SERVER['HTTP_HOST']; ?>/player_api.php</p>
                <p style="margin-top: 0.5rem;"><strong>Username:</strong> user</p>
                <p style="margin-top: 0.5rem;"><strong>Password:</strong> pass</p>
                <p style="margin-top: 0.5rem;"><strong>M3U8 Playlist:</strong> <?php echo (isset($_SERVER['HTTPS']) ? 'https' : 'http') . '://' . $_SERVER['HTTP_HOST']; ?>/playlist.m3u8</p>
            </div>
        </div>
        
        <div style="margin-top: 1rem;">
            <button class="btn btn-primary btn-lg" onclick="saveAllSettings()" style="width: 100%; padding: 1rem; font-size: 1.1rem;">
                💾 Save All Settings
            </button>
        </div>
    </div>
    
    <!-- MDBList Search Modal -->
    <div class="modal" id="mdblist-search-modal">
        <div class="modal-content" style="max-width: 700px;">
            <div class="modal-header">
                <h3>🔍 Search MDBList</h3>
                <button class="modal-close" onclick="closeMDBListSearchModal()">×</button>
            </div>
            <div class="modal-body">
                <div style="display: flex; gap: 0.5rem; margin-bottom: 1rem;">
                    <input type="text" id="mdblist-search-query" placeholder="Search for lists..." style="flex: 1;" onkeyup="if(event.key==='Enter') performMDBListSearch()">
                    <button class="btn btn-primary" onclick="performMDBListSearch()">Search</button>
                </div>
                <div id="mdblist-search-results" style="max-height: 400px; overflow-y: auto;">
                    <p style="text-align: center; color: var(--text-secondary);">Enter a search term to find MDBLists</p>
                </div>
            </div>
        </div>
    </div>
    
    <!-- Lists Page -->
    <div id="page-lists" class="page hidden">
        <div class="page-header">
            <h2>MDBList Integration</h2>
            <button class="btn btn-primary" onclick="saveMDBListSettings()">
                <svg width="16" height="16" fill="currentColor" viewBox="0 0 20 20"><path d="M7.707 10.293a1 1 0 10-1.414 1.414l3 3a1 1 0 001.414 0l3-3a1 1 0 00-1.414-1.414L11 11.586V6h5a2 2 0 012 2v7a2 2 0 01-2 2H4a2 2 0 01-2-2V8a2 2 0 012-2h5v5.586l-1.293-1.293z"></path></svg>
                Save Settings
            </button>
        </div>
        
        <!-- MDBList Content -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">📋 MDBList Integration</span>
                <label class="toggle-switch">
                    <input type="checkbox" id="mdblist-enabled">
                    <span class="toggle-slider"></span>
                </label>
            </div>
            
            <div style="background: var(--bg-tertiary); padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.85rem; color: var(--text-secondary);">
                📋 Import curated movie/TV lists from <a href="https://mdblist.com" target="_blank" style="color: var(--accent);">MDBList.com</a>. 
                Add popular lists like "Top Watched Movies of the Week" or create your own custom lists.
                <a href="https://mdblist.com/preferences/" target="_blank" style="color: var(--accent);">Get your API key here</a> (optional - only needed for private lists).
                </div>
                
                <div class="settings-form">
                    <div class="form-row">
                        <div class="form-group">
                            <label>MDBList API Key (Optional)</label>
                            <div style="position: relative;">
                                <input type="password" id="mdblist-apiKey" placeholder="Your MDBList API key">
                                <button type="button" onclick="togglePassword('mdblist-apiKey')" style="position: absolute; right: 8px; top: 50%; transform: translateY(-50%); background: none; border: none; color: var(--text-secondary); cursor: pointer;">👁</button>
                            </div>
                            <span style="font-size: 0.75rem; color: var(--text-secondary);">Only required for private lists and user-specific lists</span>
                        </div>
                        
                        <div class="form-group">
                            <label>Auto-Sync Interval</label>
                            <select id="mdblist-syncInterval">
                                <option value="0">Manual Only</option>
                                <option value="1">Every 1 Hour</option>
                                <option value="3">Every 3 Hours</option>
                                <option value="6">Every 6 Hours</option>
                                <option value="12">Every 12 Hours</option>
                                <option value="24">Every 24 Hours</option>
                            </select>
                        </div>
                    </div>
                    
                    <div class="form-row">
                        <div class="form-group">
                            <label>Merge with Main Playlist</label>
                            <label class="toggle-switch">
                                <input type="checkbox" id="mdblist-mergePlaylist" checked>
                                <span class="toggle-slider"></span>
                            </label>
                            <span style="font-size: 0.75rem; color: var(--text-secondary);">Add MDBList items to your main movie/series playlists</span>
                        </div>
                        
                        <div class="form-group">
                            <button class="btn btn-secondary" onclick="testMDBListConnection()">
                                🔌 Test Connection
                            </button>
                            <span id="mdblist-connection-status" style="margin-left: 0.5rem; font-size: 0.85rem;"></span>
                        </div>
                    </div>
                </div>
                
                <!-- Add New List -->
                <div style="border-top: 1px solid var(--border); padding-top: 1rem; margin-top: 1rem;">
                    <h4 style="margin-bottom: 0.75rem; color: var(--text-primary);">Add MDBList URL</h4>
                    <div style="display: flex; gap: 0.5rem; margin-bottom: 0.5rem;">
                        <input type="text" id="mdblist-new-url" placeholder="https://mdblist.com/lists/username/list-name" style="flex: 1;">
                        <button class="btn btn-primary" onclick="addMDBList()">
                            ➕ Add List
                        </button>
                    </div>
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">
                        Paste any public MDBList URL. Examples: 
                        <code>mdblist.com/lists/linaspuransen/top-watched-movies-of-the-week</code>
                    </span>
                    
                    <!-- Popular Lists Quick Add -->
                    <div style="margin-top: 1rem;">
                        <label style="font-size: 0.85rem; color: var(--text-secondary);">Popular Lists:</label>
                        <div style="display: flex; gap: 0.5rem; flex-wrap: wrap; margin-top: 0.5rem;">
                            <button class="btn btn-secondary" style="font-size: 0.75rem; padding: 0.4rem 0.75rem;" onclick="quickAddMDBList('https://mdblist.com/lists/linaspuransen/top-watched-movies-of-the-week', 'Top Watched Movies')">
                                🎬 Top Watched Movies
                            </button>
                            <button class="btn btn-secondary" style="font-size: 0.75rem; padding: 0.4rem 0.75rem;" onclick="quickAddMDBList('https://mdblist.com/lists/hdlists/top-ten-pirated-movies-of-the-week', 'Top Pirated Movies')">
                                🏴‍☠️ Top Pirated
                            </button>
                            <button class="btn btn-secondary" style="font-size: 0.75rem; padding: 0.4rem 0.75rem;" onclick="quickAddMDBList('https://mdblist.com/lists/linaspuransen/top-watched-tv-shows-of-the-week', 'Top Watched TV')">
                                📺 Top Watched TV
                            </button>
                            <button class="btn btn-secondary" style="font-size: 0.75rem; padding: 0.4rem 0.75rem;" onclick="searchMDBLists()">
                                🔍 Search Lists...
                            </button>
                        </div>
                    </div>
                </div>
                
                <!-- Configured Lists -->
                <div style="border-top: 1px solid var(--border); padding-top: 1rem; margin-top: 1rem;">
                    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem;">
                        <h4 style="margin: 0; color: var(--text-primary);">Your MDBList Sources</h4>
                        <div style="display: flex; gap: 0.5rem;">
                            <button class="btn btn-secondary" style="font-size: 0.75rem;" onclick="syncMDBLists()">
                                🔄 Sync Now
                            </button>
                            <button class="btn btn-success" style="font-size: 0.75rem;" onclick="mergeMDBListsToPlaylist()">
                                📥 Merge to Playlists
                            </button>
                            <button class="btn btn-secondary" style="font-size: 0.75rem;" onclick="clearMDBListCache()">
                                🗑️ Clear Cache
                            </button>
                        </div>
                    </div>
                    
                    <div id="mdblist-sources" style="max-height: 300px; overflow-y: auto;">
                        <!-- Lists will be populated by JavaScript -->
                        <div class="loading" style="text-align: center; padding: 1rem; color: var(--text-secondary);">Loading lists...</div>
                    </div>
                    
                    <!-- Sync Status -->
                    <div id="mdblist-sync-status" style="margin-top: 1rem; padding: 0.75rem; background: var(--bg-tertiary); border-radius: 6px; font-size: 0.85rem; display: none;">
                        <div style="display: flex; justify-content: space-between;">
                            <span>Last Sync: <span id="mdblist-last-sync">Never</span></span>
                            <span>Movies: <span id="mdblist-movie-count">0</span> | Series: <span id="mdblist-series-count">0</span></span>
                        </div>
                    </div>
                </div>
                
                <div style="margin-top: 1rem;">
                    <button class="btn btn-primary" onclick="saveMDBListSettings()">
                        💾 Save MDBList Settings
                    </button>
                </div>
        </div>
    </div>
    
    <!-- Filters Page -->
    <div id="page-filters" class="page hidden">
        <div class="page-header">
            <h2>Release Filters</h2>
        </div>
        
        <div class="card">
            <div class="card-header">
                <span class="card-title">Filter Settings</span>
                <label class="toggle-switch">
                    <input type="checkbox" id="filter-enabled" checked>
                    <span class="toggle-slider"></span>
                </label>
            </div>
            <p style="margin-bottom: 1rem; color: var(--text-secondary); font-size: 0.9rem;">
                Filter out unwanted releases by release group, language, or quality. Separate multiple patterns with <code>|</code> (pipe character).
            </p>
            
            <div class="settings-form">
                <div class="form-group">
                    <label>Excluded Release Groups</label>
                    <input type="text" id="filter-releaseGroups" placeholder="TVHUB|FILM|RARBG" style="font-family: monospace;">
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">
                        Block releases from specific groups. Example: <code>TVHUB|FILM</code> blocks Russian releases like "Movie.TVHUB.FILM.mkv"
                    </span>
                </div>
                
                <div class="form-group">
                    <label>Excluded Language Tags</label>
                    <input type="text" id="filter-languages" placeholder="RUSSIAN|RUS|HINDI|HIN|GERMAN|GER" style="font-family: monospace;">
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">
                        Block releases with language indicators in filename. Example: <code>RUSSIAN|RUS|HINDI|GERMAN</code>
                    </span>
                </div>
                
                <div class="form-group">
                    <label>Excluded Qualities</label>
                    <input type="text" id="filter-qualities" placeholder="REMUX|HDR|DV|3D|CAM|TS|SCR" style="font-family: monospace;">
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">
                        Block certain quality types. Example: <code>REMUX|HDR|CAM|TS</code> blocks REMUX (too large), HDR (compatibility issues), CAM/TS (low quality)
                    </span>
                </div>
                
                <div class="form-group">
                    <label>Custom Exclude Patterns (Advanced)</label>
                    <input type="text" id="filter-custom" placeholder="Sample|Trailer|\[Dual\]" style="font-family: monospace;">
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">
                        Custom regex patterns. Example: <code>Sample|Trailer</code> blocks sample files and trailers
                    </span>
                </div>
                
                <button class="btn btn-primary" onclick="saveFilters()">Save Filters</button>
            </div>
        </div>
        
        <div class="card">
            <div class="card-header">
                <span class="card-title">Filter Preview</span>
            </div>
            <div style="background: var(--bg-tertiary); padding: 1rem; border-radius: 6px;">
                <p style="margin-bottom: 0.5rem; color: var(--text-secondary);">These release names would be <strong style="color: var(--danger);">blocked</strong>:</p>
                <div id="filter-preview" style="font-family: monospace; font-size: 0.85rem; color: var(--danger);">
                    Loading...
                </div>
            </div>
        </div>
        
        <div class="card">
            <div class="card-header">
                <span class="card-title">Common Presets</span>
            </div>
            <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
                <button class="btn btn-secondary" onclick="applyPreset('english-only')">
                    🇺🇸 English Only
                </button>
                <button class="btn btn-secondary" onclick="applyPreset('no-cam')">
                    🎬 No CAM/TS
                </button>
                <button class="btn btn-secondary" onclick="applyPreset('player-friendly')">
                    📺 Player Friendly
                </button>
                <button class="btn btn-secondary" onclick="applyPreset('clear-all')">
                    ❌ Clear All
                </button>
            </div>
        </div>
    </div>
</main>

<script>
let currentStatus = {};
let settingsDirty = false; // Track if user has made unsaved changes

// Navigation
document.querySelectorAll('.nav-item[data-page]').forEach(item => {
    item.addEventListener('click', () => {
        document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
        item.classList.add('active');
        
        document.querySelectorAll('.page').forEach(p => p.classList.add('hidden'));
        document.getElementById('page-' + item.dataset.page).classList.remove('hidden');
        
        if (item.dataset.page === 'logs') loadLogs();
        if (item.dataset.page === 'lists') loadMDBLists();
    });
});

// Refresh status
async function refreshStatus() {
    const icon = document.getElementById('refresh-icon');
    icon.classList.add('refresh-indicator');
    
    try {
        const response = await fetch('?api=status');
        currentStatus = await response.json();
        updateDashboard(currentStatus);
    } catch (error) {
        showToast('Failed to refresh status', 'error');
    }
    
    icon.classList.remove('refresh-indicator');
}

function updateDashboard(status) {
    // Stats
    document.getElementById('stat-movies').textContent = status.playlists?.movies?.count?.toLocaleString() || '0';
    document.getElementById('stat-series').textContent = status.playlists?.series?.count?.toLocaleString() || '0';
    document.getElementById('stat-episodes').textContent = status.cache?.episodes?.count?.toLocaleString() || '0';
    document.getElementById('stat-m3u').textContent = status.playlists?.m3u8?.entries?.toLocaleString() || '0';
    
    // MDBList stats
    const mdbMovies = status.mdblist?.movies || 0;
    const mdbSeries = status.mdblist?.series || 0;
    const mdbMerged = status.mdblist?.merged;
    
    const mdbMoviesEl = document.getElementById('stat-mdb-movies');
    const mdbSeriesEl = document.getElementById('stat-mdb-series');
    
    if (mdbMoviesEl && mdbMovies > 0) {
        mdbMoviesEl.textContent = `+${mdbMovies.toLocaleString()} MDBList` + (mdbMerged ? ' ✓' : '');
    } else if (mdbMoviesEl) {
        mdbMoviesEl.textContent = '';
    }
    
    if (mdbSeriesEl && mdbSeries > 0) {
        mdbSeriesEl.textContent = `+${mdbSeries.toLocaleString()} MDBList` + (mdbMerged ? ' ✓' : '');
    } else if (mdbSeriesEl) {
        mdbSeriesEl.textContent = '';
    }
    
    // System info
    document.getElementById('stat-last-sync').textContent = status.daemons?.background_sync?.last_sync || 'Never';
    document.getElementById('stat-next-sync').textContent = status.daemons?.background_sync?.next_sync || 'Unknown';
    document.getElementById('stat-disk').textContent = status.system?.disk_free || '-';
    document.getElementById('stat-php').textContent = status.system?.php_version || '-';
    
    // Daemon status
    const syncDaemon = status.daemons?.background_sync;
    const statusEl = document.getElementById('daemon-sync-status');
    const pidEl = document.getElementById('daemon-sync-pid');
    
    if (syncDaemon?.running) {
        statusEl.className = 'status-badge status-running';
        statusEl.textContent = 'Running';
        pidEl.textContent = 'PID: ' + syncDaemon.pid;
    } else {
        statusEl.className = 'status-badge status-stopped';
        statusEl.textContent = 'Stopped';
        pidEl.textContent = '';
    }
    
    // Settings - populate all form fields (skip if user has unsaved changes)
    if (status.config && !settingsDirty) {
        // API Keys
        const apiKeyEl = document.getElementById('setting-apiKey');
        if (apiKeyEl) apiKeyEl.value = status.config.apiKey || '';
        
        const rdTokenEl = document.getElementById('setting-rdToken');
        if (rdTokenEl) rdTokenEl.value = status.config.rdToken || '';
        
        const premiumizeKeyEl = document.getElementById('setting-premiumizeKey');
        if (premiumizeKeyEl) premiumizeKeyEl.value = status.config.premiumizeKey || '';
        
        // Debrid settings
        const useRDEl = document.getElementById('setting-useRealDebrid');
        if (useRDEl) useRDEl.checked = status.config.useRealDebrid === true;
        
        const usePremEl = document.getElementById('setting-usePremiumize');
        if (usePremEl) usePremEl.checked = status.config.usePremiumize === true;
        
        // Playlist settings
        const totalPagesEl = document.getElementById('setting-totalPages');
        if (totalPagesEl) totalPagesEl.value = status.config.totalPages || 5;
        
        const maxResEl = document.getElementById('setting-maxResolution');
        if (maxResEl) maxResEl.value = status.config.maxResolution || 1080;
        
        const m3u8LimitEl = document.getElementById('setting-m3u8Limit');
        if (m3u8LimitEl) m3u8LimitEl.value = status.config.m3u8Limit || 0;
        
        const autoCacheEl = document.getElementById('setting-autoCacheInterval');
        if (autoCacheEl) autoCacheEl.value = status.config.autoCacheInterval || 6;
        
        const createPlaylistEl = document.getElementById('setting-userCreatePlaylist');
        if (createPlaylistEl) createPlaylistEl.checked = status.config.userCreatePlaylist === true;
        
        // Content options
        const adultEl = document.getElementById('setting-includeAdult');
        if (adultEl) adultEl.checked = status.config.includeAdult === true;
        
        const debugEl = document.getElementById('setting-debugMode');
        if (debugEl) debugEl.checked = status.config.debugMode === true;
        
        // Regional settings
        const langEl = document.getElementById('setting-language');
        if (langEl) langEl.value = status.config.language || 'en-US';
        
        const seriesCountryEl = document.getElementById('setting-seriesCountry');
        if (seriesCountryEl) seriesCountryEl.value = status.config.seriesCountry || '';
        
        const moviesCountryEl = document.getElementById('setting-moviesCountry');
        if (moviesCountryEl) moviesCountryEl.value = status.config.moviesCountry || '';
        
        const hostEl = document.getElementById('setting-userSetHost');
        if (hostEl) hostEl.value = status.config.userSetHost || '';
        
        // Stream providers
        const providers = status.config.streamProviders || ['comet', 'mediafusion'];
        const cometEl = document.getElementById('provider-comet');
        if (cometEl) cometEl.checked = providers.includes('comet');
        
        const mfEl = document.getElementById('provider-mediafusion');
        if (mfEl) mfEl.checked = providers.includes('mediafusion');
        
        const torrentioEl = document.getElementById('provider-torrentio');
        if (torrentioEl) torrentioEl.checked = providers.includes('torrentio');
        
        const torrentioProvidersEl = document.getElementById('setting-torrentioProviders');
        if (torrentioProvidersEl) torrentioProvidersEl.value = status.config.torrentioProviders || '';
        
        // Load Comet indexers
        const cometIndexers = status.config.cometIndexers || ['bktorrent', 'thepiratebay', 'yts', 'eztv', 'kickasstorrents'];
        document.querySelectorAll('.comet-indexer').forEach(el => {
            el.checked = cometIndexers.includes(el.value);
        });
        
        // Show/hide provider-specific settings based on enabled providers
        updateProviderSettingsVisibility();
    }
    
    // Filters
    if (status.filters) {
        document.getElementById('filter-enabled').checked = status.filters.enabled !== false;
        document.getElementById('filter-releaseGroups').value = status.filters.releaseGroups || '';
        document.getElementById('filter-languages').value = status.filters.languages || '';
        document.getElementById('filter-qualities').value = status.filters.qualities || '';
        document.getElementById('filter-custom').value = status.filters.custom || '';
        updateFilterPreview();
    }
}

// Daemon control
async function controlDaemon(action) {
    try {
        const response = await fetch(`?api=daemon-${action}`);
        const result = await response.json();
        
        if (result.success) {
            showToast(`Daemon ${action}ed successfully`, 'success');
            setTimeout(refreshStatus, 1000);
        } else {
            showToast(`Failed to ${action} daemon`, 'error');
        }
    } catch (error) {
        showToast('Error: ' + error.message, 'error');
    }
}

// Run action
async function runAction(action) {
    showToast('Running ' + action + '...', 'success');
    
    try {
        const response = await fetch(`?api=${action}`);
        const result = await response.json();
        
        if (result.success) {
            showToast('Action completed!', 'success');
            refreshStatus();
        } else {
            showToast('Action failed: ' + (result.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        showToast('Error: ' + error.message, 'error');
    }
}

// Clear episode cache
async function clearEpisodeCache() {
    if (!confirm('Clear all cached episodes? This will require re-caching for playback.')) return;
    
    showToast('Clearing episode cache...', 'success');
    try {
        const response = await fetch('?api=clear-episode-cache');
        const result = await response.json();
        if (result.success) {
            showToast(result.message, 'success');
            refreshStatus();
        } else {
            showToast('Failed to clear cache', 'error');
        }
    } catch (error) {
        showToast('Error: ' + error.message, 'error');
    }
}

// Clear M3U8 playlist
async function clearM3U8() {
    if (!confirm('Clear M3U8 playlist? You will need to sync again to restore content.')) return;
    
    showToast('Clearing M3U8...', 'success');
    try {
        const response = await fetch('?api=clear-m3u8');
        const result = await response.json();
        if (result.success) {
            showToast(result.message, 'success');
            refreshStatus();
        } else {
            showToast('Failed to clear M3U8', 'error');
        }
    } catch (error) {
        showToast('Error: ' + error.message, 'error');
    }
}

// Provider testing
async function testProvider(provider) {
    const statusEl = document.getElementById(`provider-${provider}-status`);
    statusEl.className = 'status-badge status-warning';
    statusEl.textContent = 'Testing...';
    
    try {
        const response = await fetch(`?api=test-provider&provider=${provider}`);
        const result = await response.json();
        
        if (result.success) {
            statusEl.className = 'status-badge status-running';
            statusEl.textContent = `OK (${result.streams} streams, ${result.response_time})`;
        } else {
            statusEl.className = 'status-badge status-stopped';
            statusEl.textContent = result.error || 'Failed';
        }
    } catch (error) {
        statusEl.className = 'status-badge status-stopped';
        statusEl.textContent = 'Error';
    }
}

async function testAllProviders() {
    await testProvider('comet');
    await testProvider('mediafusion');
    await testProvider('torrentio');
}

// Logs
async function loadLogs() {
    const logFile = document.getElementById('log-select').value;
    const logContent = document.getElementById('log-content');
    
    logContent.textContent = 'Loading...';
    
    try {
        const response = await fetch(`?api=logs&file=${logFile}&lines=200`);
        const result = await response.json();
        
        if (result.success) {
            logContent.textContent = result.content || 'Log file is empty';
        } else {
            logContent.textContent = 'Error: ' + result.error;
        }
    } catch (error) {
        logContent.textContent = 'Failed to load logs';
    }
}

// Settings
function togglePassword(inputId) {
    const input = document.getElementById(inputId);
    input.type = input.type === 'password' ? 'text' : 'password';
}

async function saveAllSettings() {
    // Collect all settings from the form
    const settings = {
        // API Keys
        apiKey: document.getElementById('setting-apiKey')?.value || '',
        rdToken: document.getElementById('setting-rdToken')?.value || '',
        premiumizeKey: document.getElementById('setting-premiumizeKey')?.value || '',
        
        // Debrid settings
        useRealDebrid: document.getElementById('setting-useRealDebrid')?.checked || false,
        usePremiumize: document.getElementById('setting-usePremiumize')?.checked || false,
        
        // Playlist settings
        totalPages: parseInt(document.getElementById('setting-totalPages')?.value) || 5,
        maxResolution: parseInt(document.getElementById('setting-maxResolution')?.value) || 1080,
        m3u8Limit: parseInt(document.getElementById('setting-m3u8Limit')?.value) || 0,
        autoCacheInterval: parseInt(document.getElementById('setting-autoCacheInterval')?.value) || 6,
        userCreatePlaylist: document.getElementById('setting-userCreatePlaylist')?.checked || false,
        
        // Content options
        includeAdult: document.getElementById('setting-includeAdult')?.checked || false,
        debugMode: document.getElementById('setting-debugMode')?.checked || false,
        
        // Regional settings
        language: document.getElementById('setting-language')?.value || 'en-US',
        seriesCountry: document.getElementById('setting-seriesCountry')?.value || '',
        moviesCountry: document.getElementById('setting-moviesCountry')?.value || '',
        userSetHost: document.getElementById('setting-userSetHost')?.value || '',
        
        // Stream providers
        streamProviders: getSelectedProviders(),
        cometIndexers: getSelectedCometIndexers(),
        torrentioProviders: document.getElementById('setting-torrentioProviders')?.value || '',
        mediafusionEnabled: document.getElementById('provider-mediafusion')?.checked || false
    };
    
    try {
        const response = await fetch('?api=save-settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(settings)
        });
        const result = await response.json();
        
        if (result.success) {
            showToast('Settings saved!', 'success');
            settingsDirty = false; // Reset dirty flag after successful save
            refreshStatus();
        } else {
            showToast('Failed to save settings: ' + (result.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        showToast('Error: ' + error.message, 'error');
    }
}

// Auto-sync after saving settings (now uses MDBList)
async function triggerAutoSync() {
    try {
        showToast('Syncing from MDBList...', 'success');
        const response = await fetch('?api=mdblist-sync-and-merge');
        const result = await response.json();
        
        if (result.success) {
            showToast(`Sync complete! Library: ${result.total_movies} movies, ${result.total_series} series`, 'success');
        } else {
            showToast('Sync issue: ' + (result.error || 'Check logs'), 'error');
        }
        refreshStatus();
    } catch (error) {
        showToast('Sync error: ' + error.message, 'error');
        refreshStatus();
    }
}

// Poll for stream population status
let streamPopulationPollInterval = null;
async function pollStreamPopulationStatus() {
    if (streamPopulationPollInterval) {
        clearInterval(streamPopulationPollInterval);
    }
    
    streamPopulationPollInterval = setInterval(async () => {
        try {
            const response = await fetch('?api=populate-streams-status');
            const status = await response.json();
            
            if (status.status === 'running') {
                showToast(`Populating streams: ${status.progress}% - ${status.details}`, 'success');
            } else if (status.status === 'complete') {
                showToast(`Stream population complete! ${status.details}`, 'success');
                clearInterval(streamPopulationPollInterval);
                streamPopulationPollInterval = null;
                refreshStatus();
            }
        } catch (error) {
            // Silent fail on status check
        }
    }, 5000); // Poll every 5 seconds
    
    // Stop polling after 30 minutes max
    setTimeout(() => {
        if (streamPopulationPollInterval) {
            clearInterval(streamPopulationPollInterval);
            streamPopulationPollInterval = null;
        }
    }, 30 * 60 * 1000);
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function getSelectedProviders() {
    const providers = [];
    
    if (document.getElementById('provider-comet')?.checked) {
        providers.push('comet');
    }
    if (document.getElementById('provider-mediafusion')?.checked) {
        providers.push('mediafusion');
    }
    if (document.getElementById('provider-torrentio')?.checked) {
        providers.push('torrentio');
    }
    
    // Ensure at least comet is selected
    if (providers.length === 0) {
        providers.push('comet');
    }
    
    return providers;
}

function getSelectedCometIndexers() {
    const indexers = [];
    document.querySelectorAll('.comet-indexer:checked').forEach(el => {
        indexers.push(el.value);
    });
    // Default to some indexers if none selected
    if (indexers.length === 0) {
        indexers.push('bktorrent', 'thepiratebay', 'yts');
    }
    return indexers;
}

function updateProviderSettingsVisibility() {
    const cometSettings = document.getElementById('comet-settings');
    const torrentioSettings = document.getElementById('torrentio-settings');
    
    if (cometSettings) {
        cometSettings.style.display = document.getElementById('provider-comet')?.checked ? 'block' : 'none';
    }
    if (torrentioSettings) {
        torrentioSettings.style.display = document.getElementById('provider-torrentio')?.checked ? 'block' : 'none';
    }
}

// Add event listeners for provider toggles
document.addEventListener('DOMContentLoaded', () => {
    // Toggle visibility of provider settings when checkboxes change
    ['provider-comet', 'provider-mediafusion', 'provider-torrentio'].forEach(id => {
        const el = document.getElementById(id);
        if (el) {
            el.addEventListener('change', updateProviderSettingsVisibility);
        }
    });
});

// Test provider function
async function testProvider(provider) {
    const resultDiv = document.getElementById('provider-test-result');
    if (resultDiv) {
        resultDiv.style.display = 'block';
        resultDiv.innerHTML = `<span style="color: var(--text-secondary);">Testing ${provider}...</span>`;
    }
    
    try {
        const response = await fetch(`?api=test-provider&provider=${provider}`);
        const result = await response.json();
        
        if (result.success) {
            resultDiv.innerHTML = `<span style="color: var(--success);">✅ ${provider}: OK - ${result.streams} streams found (${result.response_time})</span>`;
        } else {
            resultDiv.innerHTML = `<span style="color: var(--danger);">❌ ${provider}: Failed - ${result.error || 'No streams'}</span>`;
        }
    } catch (e) {
        resultDiv.innerHTML = `<span style="color: var(--danger);">❌ ${provider}: Error - ${e.message}</span>`;
    }
}

// Keep old function name for backward compatibility
async function saveSettings() {
    return saveAllSettings();
}

// Filter functions
async function saveFilters() {
    const filters = {
        enabled: document.getElementById('filter-enabled').checked,
        releaseGroups: document.getElementById('filter-releaseGroups').value,
        languages: document.getElementById('filter-languages').value,
        qualities: document.getElementById('filter-qualities').value,
        custom: document.getElementById('filter-custom').value
    };
    
    try {
        const response = await fetch('?api=save-filters', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(filters)
        });
        const result = await response.json();
        
        if (result.success) {
            showToast('Filters saved!', 'success');
            updateFilterPreview();
        } else {
            showToast('Failed to save filters', 'error');
        }
    } catch (error) {
        showToast('Error: ' + error.message, 'error');
    }
}

function updateFilterPreview() {
    const releaseGroups = document.getElementById('filter-releaseGroups').value;
    const languages = document.getElementById('filter-languages').value;
    const qualities = document.getElementById('filter-qualities').value;
    const custom = document.getElementById('filter-custom').value;
    
    // Sample releases to test against
    const sampleReleases = [
        'Frontier.Crucible.2025.TVHUB.FILM.WEB.720p.mkv',
        'Movie.2025.RUSSIAN.1080p.BluRay.mkv',
        'Film.2025.RUS.DUB.WEB-DL.720p.mkv',
        'Movie.2025.REMUX.2160p.BluRay.mkv',
        'Film.2025.HDR.DV.2160p.mkv',
        'Movie.2025.CAM.720p.mkv',
        'Film.2025.TELESYNC.720p.mkv',
        'Movie.2025.HINDI.1080p.WEB-DL.mkv',
        'Film.2025.GERMAN.DL.1080p.mkv',
        'Movie.2025.1080p.WEB-DL.x264-SPARKS.mkv'
    ];
    
    const patterns = [releaseGroups, languages, qualities, custom].filter(p => p).join('|');
    const blocked = [];
    
    if (patterns) {
        try {
            const regex = new RegExp('\\b(' + patterns + ')\\b', 'i');
            sampleReleases.forEach(release => {
                if (regex.test(release)) {
                    blocked.push(release);
                }
            });
        } catch (e) {
            // Invalid regex
        }
    }
    
    const preview = document.getElementById('filter-preview');
    if (blocked.length > 0) {
        preview.innerHTML = blocked.map(r => '❌ ' + r).join('<br>');
    } else {
        preview.innerHTML = '<span style="color: var(--success);">✓ No sample releases blocked (filters may be empty or invalid)</span>';
    }
}

function applyPreset(preset) {
    switch (preset) {
        case 'english-only':
            document.getElementById('filter-releaseGroups').value = 'TVHUB|FILM';
            document.getElementById('filter-languages').value = 'RUSSIAN|RUS|HINDI|HIN|GERMAN|GER|FRENCH|FRE|ITALIAN|ITA|SPANISH|SPA|LATINO|POLISH|POL|TURKISH|TUR|ARABIC|ARA|KOREAN|KOR|CHINESE|CHI|JAPANESE|JAP';
            break;
        case 'no-cam':
            document.getElementById('filter-qualities').value = 'CAM|TS|SCR|HDTS|HDCAM|TELESYNC|TELECINE|TC';
            break;
        case 'player-friendly':
            document.getElementById('filter-qualities').value = 'REMUX|HDR|DV|Dolby.?Vision|3D|CAM|TS|SCR|HDTS|HDCAM|TELESYNC|TELECINE|TC';
            break;
        case 'clear-all':
            document.getElementById('filter-releaseGroups').value = '';
            document.getElementById('filter-languages').value = '';
            document.getElementById('filter-qualities').value = '';
            document.getElementById('filter-custom').value = '';
            break;
    }
    updateFilterPreview();
    showToast('Preset applied - click Save to apply', 'success');
}

// Add input listeners for filter preview
['filter-releaseGroups', 'filter-languages', 'filter-qualities', 'filter-custom'].forEach(id => {
    document.getElementById(id)?.addEventListener('input', updateFilterPreview);
});

// Toast notifications
function showToast(message, type = 'success') {
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);
    
    setTimeout(() => toast.remove(), 3000);
}

// Track when user modifies settings (mark as dirty)
document.querySelectorAll('#page-settings input, #page-settings select').forEach(el => {
    el.addEventListener('change', () => { settingsDirty = true; });
    el.addEventListener('input', () => { settingsDirty = true; });
});

// ========== MDBLIST FUNCTIONS ==========

// Load MDBList settings into the UI
function loadMDBListSettings(status) {
    if (!status.mdblist) return;
    
    const enabledEl = document.getElementById('mdblist-enabled');
    if (enabledEl) enabledEl.checked = status.mdblist.enabled === true;
    
    const apiKeyEl = document.getElementById('mdblist-apiKey');
    if (apiKeyEl) apiKeyEl.value = status.mdblist.apiKey || '';
    
    const syncIntervalEl = document.getElementById('mdblist-syncInterval');
    if (syncIntervalEl) syncIntervalEl.value = status.mdblist.syncInterval || 6;
    
    const mergeEl = document.getElementById('mdblist-mergePlaylist');
    if (mergeEl) mergeEl.checked = status.mdblist.mergePlaylist !== false;
    
    // Update sync status display
    const syncStatusEl = document.getElementById('mdblist-sync-status');
    if (syncStatusEl && (status.mdblist.cachedMovies > 0 || status.mdblist.cachedSeries > 0)) {
        syncStatusEl.style.display = 'block';
        document.getElementById('mdblist-last-sync').textContent = status.mdblist.lastSync || 'Never';
        document.getElementById('mdblist-movie-count').textContent = status.mdblist.cachedMovies || 0;
        document.getElementById('mdblist-series-count').textContent = status.mdblist.cachedSeries || 0;
    }
    
    // Load configured lists
    renderMDBListSources(status.mdblist.lists || []);
}

// Render MDBList sources in the UI
function renderMDBListSources(lists) {
    const container = document.getElementById('mdblist-sources');
    if (!container) return;
    
    if (!lists || lists.length === 0) {
        container.innerHTML = `
            <div style="text-align: center; padding: 2rem; color: var(--text-secondary);">
                <p>📋 No MDBLists configured yet</p>
                <p style="font-size: 0.85rem; margin-top: 0.5rem;">Add a list URL above or click one of the popular lists to get started</p>
            </div>
        `;
        return;
    }
    
    container.innerHTML = lists.map((list, index) => `
        <div class="mdblist-item" style="display: flex; align-items: center; justify-content: space-between; padding: 0.75rem; background: var(--bg-tertiary); border-radius: 6px; margin-bottom: 0.5rem;">
            <div style="flex: 1; min-width: 0;">
                <div style="display: flex; align-items: center; gap: 0.5rem;">
                    <label class="toggle-switch" style="margin: 0;">
                        <input type="checkbox" ${list.enabled !== false ? 'checked' : ''} onchange="toggleMDBList('${escapeHtml(list.url)}', this.checked)">
                        <span class="toggle-slider"></span>
                    </label>
                    <span style="font-weight: 500; color: var(--text-primary);">${escapeHtml(list.name || 'Unnamed List')}</span>
                </div>
                <div style="font-size: 0.75rem; color: var(--text-secondary); margin-top: 0.25rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                    ${escapeHtml(list.url)}
                </div>
                ${list.added ? `<div style="font-size: 0.7rem; color: var(--text-muted); margin-top: 0.25rem;">Added: ${list.added}</div>` : ''}
            </div>
            <div style="display: flex; gap: 0.5rem; margin-left: 1rem;">
                <button class="btn btn-secondary" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;" onclick="previewMDBList('${escapeHtml(list.url)}')" title="Preview">
                    👁
                </button>
                <button class="btn btn-danger" style="padding: 0.25rem 0.5rem; font-size: 0.75rem;" onclick="removeMDBList('${escapeHtml(list.url)}')" title="Remove">
                    🗑️
                </button>
            </div>
        </div>
    `).join('');
}

// Test MDBList API connection
async function testMDBListConnection() {
    const statusEl = document.getElementById('mdblist-connection-status');
    statusEl.innerHTML = '<span style="color: var(--text-secondary);">Testing...</span>';
    
    try {
        const response = await fetch('?api=mdblist-test');
        const result = await response.json();
        
        if (result.success) {
            statusEl.innerHTML = `<span style="color: var(--success);">✅ Connected (${result.api_requests_remaining} requests remaining)</span>`;
        } else {
            statusEl.innerHTML = `<span style="color: var(--danger);">❌ ${result.error || 'Connection failed'}</span>`;
        }
    } catch (e) {
        statusEl.innerHTML = `<span style="color: var(--danger);">❌ Error: ${e.message}</span>`;
    }
}

// Add a new MDBList
async function addMDBList() {
    const urlInput = document.getElementById('mdblist-new-url');
    const url = urlInput.value.trim();
    
    if (!url) {
        showToast('Please enter a MDBList URL', 'error');
        return;
    }
    
    if (!url.includes('mdblist.com/lists/')) {
        showToast('Invalid MDBList URL format', 'error');
        return;
    }
    
    showToast('Adding list...', 'success');
    
    try {
        const response = await fetch('?api=mdblist-add-list', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url })
        });
        const result = await response.json();
        
        if (result.success) {
            showToast(`List added! (${result.preview?.total || 0} items)`, 'success');
            urlInput.value = '';
            refreshStatus();
        } else {
            showToast('Failed: ' + (result.error || 'Unknown error'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Quick add popular list
function quickAddMDBList(url, name) {
    document.getElementById('mdblist-new-url').value = url;
    addMDBList();
}

// Remove a MDBList
async function removeMDBList(url) {
    if (!confirm('Remove this list from your sources?')) return;
    
    try {
        const response = await fetch('?api=mdblist-remove-list', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url })
        });
        const result = await response.json();
        
        if (result.success) {
            showToast('List removed', 'success');
            refreshStatus();
        } else {
            showToast('Failed to remove list', 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Toggle MDBList enabled status
async function toggleMDBList(url, enabled) {
    try {
        const response = await fetch('?api=mdblist-toggle-list', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url, enabled })
        });
        const result = await response.json();
        
        if (!result.success) {
            showToast('Failed to update list', 'error');
            refreshStatus();
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Preview a MDBList
async function previewMDBList(url) {
    showToast('Fetching list preview...', 'success');
    
    try {
        const response = await fetch(`?api=mdblist-preview&url=${encodeURIComponent(url)}`);
        const result = await response.json();
        
        if (result.success) {
            alert(`List Preview:\n\nMovies: ${result.movie_count}\nSeries: ${result.series_count}\nTotal: ${result.total}\n\nFirst 5 items:\n${
                [...(result.movies || []).slice(0, 3), ...(result.series || []).slice(0, 2)]
                    .map(i => '• ' + (i.title || i.name)).join('\n')
            }`);
        } else {
            showToast('Preview failed: ' + (result.error || 'Unknown'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Sync all MDBLists now
async function syncMDBLists() {
    showToast('Syncing MDBLists...', 'success');
    
    try {
        const response = await fetch('?api=mdblist-sync');
        const result = await response.json();
        
        if (result.success) {
            let msg = `Synced! From MDBList: ${result.movie_count} movies, ${result.series_count} series`;
            if (result.auto_merged) {
                msg += `\nAuto-merged: +${result.added_movies} movies, +${result.added_series} series`;
                msg += `\nLibrary total: ${result.total_movies} movies, ${result.total_series} series`;
            }
            showToast(msg, 'success');
            refreshStatus();
        } else {
            showToast('Sync failed: ' + (result.error || 'Unknown'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Merge MDBList items to main playlists
async function mergeMDBListsToPlaylist() {
    showToast('Merging MDBList items to playlists...', 'success');
    
    try {
        const response = await fetch('?api=mdblist-merge');
        const result = await response.json();
        
        if (result.success) {
            showToast(`Merged! Added ${result.added_movies} movies, ${result.added_series} series. Total: ${result.total_movies} movies, ${result.total_series} series`, 'success');
            refreshStatus();
        } else {
            showToast('Merge failed: ' + (result.error || 'Unknown'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Clear MDBList cache
async function clearMDBListCache() {
    if (!confirm('Clear all cached MDBList data?')) return;
    
    try {
        const response = await fetch('?api=mdblist-clear-cache');
        const result = await response.json();
        
        if (result.success) {
            showToast(result.message, 'success');
            refreshStatus();
        } else {
            showToast('Failed to clear cache', 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Save MDBList settings
async function saveMDBListSettings() {
    const settings = {
        enabled: document.getElementById('mdblist-enabled')?.checked || false,
        apiKey: document.getElementById('mdblist-apiKey')?.value || '',
        syncInterval: parseInt(document.getElementById('mdblist-syncInterval')?.value) || 6,
        mergePlaylist: document.getElementById('mdblist-mergePlaylist')?.checked || false
    };
    
    try {
        const response = await fetch('?api=mdblist-save-settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(settings)
        });
        const result = await response.json();
        
        if (result.success) {
            showToast('MDBList settings saved!', 'success');
            
            // Also sync if enabled
            if (settings.enabled) {
                syncMDBLists();
            }
        } else {
            showToast('Failed to save settings', 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Search MDBLists modal
function searchMDBLists() {
    document.getElementById('mdblist-search-modal').classList.add('show');
    document.getElementById('mdblist-search-query').focus();
}

function closeMDBListSearchModal() {
    document.getElementById('mdblist-search-modal').classList.remove('show');
}

// Perform MDBList search
async function performMDBListSearch() {
    const query = document.getElementById('mdblist-search-query').value.trim();
    if (!query) return;
    
    const resultsDiv = document.getElementById('mdblist-search-results');
    resultsDiv.innerHTML = '<p style="text-align: center; color: var(--text-secondary);">Searching...</p>';
    
    try {
        const response = await fetch(`?api=mdblist-search&q=${encodeURIComponent(query)}`);
        const result = await response.json();
        
        if (result.success && result.lists && result.lists.length > 0) {
            resultsDiv.innerHTML = result.lists.map(list => `
                <div style="display: flex; align-items: center; justify-content: space-between; padding: 0.75rem; background: var(--bg-tertiary); border-radius: 6px; margin-bottom: 0.5rem;">
                    <div style="flex: 1;">
                        <div style="font-weight: 500;">${escapeHtml(list.name || 'Unnamed')}</div>
                        <div style="font-size: 0.75rem; color: var(--text-secondary);">
                            ${list.mediatype || 'mixed'} • ${list.items || 0} items • by ${escapeHtml(list.user_name || 'unknown')}
                        </div>
                    </div>
                    <button class="btn btn-primary" style="font-size: 0.8rem;" onclick="addSearchResultList('https://mdblist.com/lists/${escapeHtml(list.user_name)}/${escapeHtml(list.slug)}')">
                        ➕ Add
                    </button>
                </div>
            `).join('');
        } else {
            resultsDiv.innerHTML = '<p style="text-align: center; color: var(--text-secondary);">No lists found</p>';
        }
    } catch (e) {
        resultsDiv.innerHTML = `<p style="text-align: center; color: var(--danger);">Error: ${e.message}</p>`;
    }
}

// Add list from search results
async function addSearchResultList(url) {
    showToast('Adding list...', 'success');
    
    try {
        const response = await fetch('?api=mdblist-add-list', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url })
        });
        const result = await response.json();
        
        if (result.success) {
            showToast(`List added!`, 'success');
            closeMDBListSearchModal();
            refreshStatus();
        } else {
            showToast('Failed: ' + (result.error || 'Unknown'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// ========== SERVICES TAB FUNCTIONS ==========

// Refresh service status
async function refreshServiceStatus() {
    try {
        const response = await fetch('?api=service-status');
        const result = await response.json();
        
        if (result.success) {
            // Update MDBList stats
            document.getElementById('svc-mdblist-movies').textContent = result.mdblist.movies || '0';
            document.getElementById('svc-mdblist-series').textContent = result.mdblist.series || '0';
            document.getElementById('svc-mdblist-lists').textContent = result.mdblist.lists || '0';
            document.getElementById('svc-mdblist-lastsync').textContent = result.mdblist.last_sync || 'Never';
            
            // Update M3U8 stats
            document.getElementById('svc-m3u8-movies').textContent = result.m3u8.movies || '0';
            document.getElementById('svc-m3u8-series').textContent = result.m3u8.series || '0';
            document.getElementById('svc-m3u8-livetv').textContent = result.m3u8.livetv || '0';
            document.getElementById('svc-m3u8-lastgen').textContent = result.m3u8.last_generated || 'Unknown';
            
            showToast('Service status updated', 'success');
        }
    } catch (e) {
        showToast('Error refreshing status: ' + e.message, 'error');
    }
}

// Run MDBList sync only
async function runMDBListSync() {
    showToast('Syncing MDBLists...', 'success');
    try {
        const response = await fetch('?api=mdblist-sync');
        const result = await response.json();
        
        if (result.success) {
            showToast(`Synced! ${result.movie_count} movies, ${result.series_count} series from MDBList`, 'success');
            refreshServiceStatus();
        } else {
            showToast('Sync failed: ' + (result.error || 'Unknown'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Run MDBList merge only
async function runMDBListMerge() {
    showToast('Merging to library...', 'success');
    try {
        const response = await fetch('?api=mdblist-merge');
        const result = await response.json();
        
        if (result.success) {
            showToast(`Merged! Added ${result.added_movies} movies, ${result.added_series} series. Total: ${result.total_movies} movies, ${result.total_series} series`, 'success');
            refreshServiceStatus();
        } else {
            showToast('Merge failed: ' + (result.error || 'Unknown'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Run MDBList sync + merge + regenerate
async function runMDBListSyncAndMerge() {
    showToast('Running full sync + merge...', 'success');
    try {
        const response = await fetch('?api=mdblist-sync-and-merge');
        const result = await response.json();
        
        if (result.success) {
            let msg = `✅ ${result.message}\n`;
            msg += `From MDBList: ${result.synced_movies} movies, ${result.synced_series} series\n`;
            msg += `Library total: ${result.total_movies} movies, ${result.total_series} series`;
            showToast(msg, 'success');
            refreshServiceStatus();
            refreshStatus(); // Also refresh main dashboard
        } else {
            showToast('Sync failed: ' + (result.error || 'Unknown'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Regenerate M3U8 playlist
async function regenerateM3U8() {
    showToast('Regenerating M3U8 playlist...', 'success');
    try {
        const response = await fetch('?api=regenerate-m3u8');
        const result = await response.json();
        
        if (result.success) {
            showToast('M3U8 playlist regenerated!', 'success');
            refreshServiceStatus();
        } else {
            showToast('Failed: ' + (result.message || 'Unknown error'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Regenerate Live TV
async function regenerateLiveTV() {
    showToast('Refreshing Live TV channels...', 'success');
    try {
        const response = await fetch('?api=regenerate-livetv');
        const result = await response.json();
        
        if (result.success) {
            showToast('Live TV refreshed!', 'success');
            refreshServiceStatus();
        } else {
            showToast('Failed: ' + (result.message || 'Unknown error'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Run episode cache
async function runEpisodeCache() {
    showToast('Running episode cache...', 'success');
    try {
        const response = await fetch('?api=run-episode-cache');
        const result = await response.json();
        
        if (result.success) {
            showToast('Episode cache completed!', 'success');
        } else {
            showToast('Failed: ' + (result.message || 'Unknown error'), 'error');
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Copy cron command
function copyCronCommand(elementId) {
    const code = document.getElementById(elementId);
    if (code) {
        navigator.clipboard.writeText(code.textContent).then(() => {
            showToast('Copied to clipboard!', 'success');
        }).catch(() => {
            // Fallback for older browsers
            const range = document.createRange();
            range.selectNode(code);
            window.getSelection().removeAllRanges();
            window.getSelection().addRange(range);
            document.execCommand('copy');
            window.getSelection().removeAllRanges();
            showToast('Copied to clipboard!', 'success');
        });
    }
}

// Check cron status
async function checkCronStatus() {
    showToast('Checking cron jobs...', 'success');
    try {
        const response = await fetch('?api=check-cron');
        const result = await response.json();
        
        const outputDiv = document.getElementById('cron-status-output');
        const pre = outputDiv.querySelector('pre');
        
        if (result.success) {
            let status = '=== Current Crontab ===\n\n';
            status += result.crontab || '(empty or no access)\n';
            status += '\n\n=== Detection ===\n';
            status += `Background Sync: ${result.has_sync ? '✅ Found' : '❌ Not found'}\n`;
            status += `Episode Cache: ${result.has_cache ? '✅ Found' : '❌ Not found'}\n`;
            status += `MDBList Sync: ${result.has_mdblist ? '✅ Found' : '❌ Not found'}\n`;
            
            pre.textContent = status;
            outputDiv.style.display = 'block';
        } else {
            pre.textContent = 'Failed to check cron status';
            outputDiv.style.display = 'block';
        }
    } catch (e) {
        showToast('Error: ' + e.message, 'error');
    }
}

// Show cron setup instructions
function showCronInstructions() {
    alert(`Cron Setup Instructions:

1. SSH into your server
2. Run: crontab -e
3. Add the cron commands shown above
4. Save and exit

For this server, use:
cd /var/www/streaming

Example full crontab entry for auto-sync every 6 hours:
0 */6 * * * cd /var/www/streaming && php daemons/background_sync_daemon.php >> logs/sync_daemon.log 2>&1

To verify cron is running:
- Check logs in /var/www/streaming/logs/
- Click "Check Cron Status" button`);
}

// Update updateDashboard to include MDBList settings
const originalUpdateDashboard = updateDashboard;
updateDashboard = function(status) {
    originalUpdateDashboard(status);
    loadMDBListSettings(status);
};

// Initial load
refreshStatus();
setInterval(refreshStatus, 30000); // Auto-refresh every 30 seconds
</script>
<?php endif; ?>

</body>
</html>
