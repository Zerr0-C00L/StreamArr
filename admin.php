<?php
/**
 * Admin Dashboard - Radarr/Sonarr Style
 * Monitor services, start/stop daemons, modify settings
 */

// Handle CLI stream population mode
if (php_sapi_name() === 'cli' && in_array('--populate-streams', $argv ?? [])) {
    require_once __DIR__ . '/config.php';
    require_once __DIR__ . '/libs/episode_cache_db.php';
    populateStreamsFromLists();
    exit;
}

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
            $result = shell_exec('nohup php ' . __DIR__ . '/background_sync_daemon.php --daemon > /dev/null 2>&1 & echo $!');
            echo json_encode(['success' => true, 'pid' => trim($result)]);
            break;
            
        case 'daemon-stop':
            $lockFile = __DIR__ . '/cache/sync_daemon.lock';
            if (file_exists($lockFile)) {
                $pid = trim(file_get_contents($lockFile));
                if ($pid) {
                    posix_kill(intval($pid), SIGTERM);
                    unlink($lockFile);
                }
            }
            echo json_encode(['success' => true]);
            break;
            
        case 'sync-now':
            $result = shell_exec('php ' . __DIR__ . '/background_sync_daemon.php 2>&1');
            echo json_encode(['success' => true, 'output' => $result]);
            break;
            
        case 'generate-playlist':
            $result = shell_exec('php ' . __DIR__ . '/auto_playlist_daemon.php 2>&1');
            echo json_encode(['success' => true, 'output' => $result]);
            break;
            
        case 'cache-episodes':
            $result = shell_exec('nohup php ' . __DIR__ . '/sync_github_cache.php > /dev/null 2>&1 & echo "Started"');
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
            
        case 'sync-github':
            // First sync playlists from GitHub
            $syncResult = shell_exec('php ' . __DIR__ . '/background_sync_daemon.php 2>&1');
            // Then trigger stream population in background
            shell_exec('nohup php ' . __DIR__ . '/admin.php --populate-streams > /dev/null 2>&1 &');
            echo json_encode(['success' => true, 'message' => 'Sync started, stream population running in background']);
            break;
            
        case 'populate-streams-status':
            $statusFile = __DIR__ . '/cache/stream_populate_status.json';
            if (file_exists($statusFile)) {
                echo file_get_contents($statusFile);
            } else {
                echo json_encode(['status' => 'idle', 'progress' => 0]);
            }
            break;
            
        default:
            echo json_encode(['error' => 'Unknown action']);
    }
    exit;
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
    
    // Check daemon status
    $lockFile = __DIR__ . '/cache/sync_daemon.lock';
    $daemonRunning = false;
    $daemonPid = null;
    
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
        $result = $db->querySingle("SELECT COUNT(*) FROM episode_cache");
        $status['cache']['episodes'] = [
            'count' => $result ?? 0,
            'size' => formatBytes(filesize($cacheDb))
        ];
        $db->close();
    }
    
    // Provider status
    $providers = $GLOBALS['STREAM_PROVIDERS'] ?? ['comet', 'mediafusion', 'torrentio'];
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
        'useGithubForCache' => $GLOBALS['useGithubForCache'] ?? true,
        'userCreatePlaylist' => $GLOBALS['userCreatePlaylist'] ?? true,
        
        // Content options
        'includeLiveTV' => $GLOBALS['INCLUDE_LIVE_TV'] ?? true,
        'includeCollections' => $GLOBALS['INCLUDE_COLLECTIONS'] ?? true,
        'includeAdult' => $GLOBALS['INCLUDE_ADULT_VOD'] ?? false,
        'debugMode' => $GLOBALS['DEBUG'] ?? false,
        
        // Movie lists from TMDB
        'includeNowPlaying' => $GLOBALS['INCLUDE_NOW_PLAYING'] ?? false,
        'includePopularMovies' => $GLOBALS['INCLUDE_POPULAR_MOVIES'] ?? false,
        'includeTopRatedMovies' => $GLOBALS['INCLUDE_TOP_RATED_MOVIES'] ?? false,
        'includeUpcoming' => $GLOBALS['INCLUDE_UPCOMING'] ?? false,
        'includeLatestReleasesMovies' => $GLOBALS['INCLUDE_LATEST_RELEASES_MOVIES'] ?? false,
        
        // Series lists from TMDB
        'includeAiringToday' => $GLOBALS['INCLUDE_AIRING_TODAY'] ?? false,
        'includeOnTheAir' => $GLOBALS['INCLUDE_ON_THE_AIR'] ?? false,
        'includePopularSeries' => $GLOBALS['INCLUDE_POPULAR_SERIES'] ?? false,
        'includeTopRatedSeries' => $GLOBALS['INCLUDE_TOP_RATED_SERIES'] ?? false,
        'includeLatestReleasesSeries' => $GLOBALS['INCLUDE_LATEST_RELEASES_SERIES'] ?? false,
        
        // Regional settings
        'language' => $GLOBALS['language'] ?? 'en-US',
        'seriesCountry' => $GLOBALS['series_with_origin_country'] ?? 'US',
        'moviesCountry' => $GLOBALS['movies_with_origin_country'] ?? 'US',
        'userSetHost' => $GLOBALS['userSetHost'] ?? '',
        
        // Stream providers
        'streamProviders' => $GLOBALS['STREAM_PROVIDERS'] ?? ['comet', 'mediafusion', 'torrentio'],
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
        'useGithubForCache' => '/\$useGithubForCache\s*=\s*(true|false)/',
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
    
    // Handle Include Live TV (GLOBALS)
    if (isset($settings['includeLiveTV'])) {
        $value = $settings['includeLiveTV'] ? 'true' : 'false';
        $content = preg_replace(
            "/\\\$GLOBALS\['INCLUDE_LIVE_TV'\]\s*=\s*(true|false)/",
            "\$GLOBALS['INCLUDE_LIVE_TV'] = $value",
            $content
        );
    }
    
    // Handle Include Collections (GLOBALS)
    if (isset($settings['includeCollections'])) {
        $value = $settings['includeCollections'] ? 'true' : 'false';
        $content = preg_replace(
            "/\\\$GLOBALS\['INCLUDE_COLLECTIONS'\]\s*=\s*(true|false)/",
            "\$GLOBALS['INCLUDE_COLLECTIONS'] = $value",
            $content
        );
    }
    
    // Handle Movie Lists (GLOBALS)
    $movieListSettings = [
        'includeNowPlaying' => 'INCLUDE_NOW_PLAYING',
        'includePopularMovies' => 'INCLUDE_POPULAR_MOVIES',
        'includeTopRatedMovies' => 'INCLUDE_TOP_RATED_MOVIES',
        'includeUpcoming' => 'INCLUDE_UPCOMING',
        'includeLatestReleasesMovies' => 'INCLUDE_LATEST_RELEASES_MOVIES'
    ];
    
    foreach ($movieListSettings as $key => $globalKey) {
        if (isset($settings[$key])) {
            $value = $settings[$key] ? 'true' : 'false';
            $content = preg_replace(
                "/\\\$GLOBALS\['$globalKey'\]\s*=\s*(true|false)/",
                "\$GLOBALS['$globalKey'] = $value",
                $content
            );
        }
    }
    
    // Handle Series Lists (GLOBALS)
    $seriesListSettings = [
        'includeAiringToday' => 'INCLUDE_AIRING_TODAY',
        'includeOnTheAir' => 'INCLUDE_ON_THE_AIR',
        'includePopularSeries' => 'INCLUDE_POPULAR_SERIES',
        'includeTopRatedSeries' => 'INCLUDE_TOP_RATED_SERIES',
        'includeLatestReleasesSeries' => 'INCLUDE_LATEST_RELEASES_SERIES'
    ];
    
    foreach ($seriesListSettings as $key => $globalKey) {
        if (isset($settings[$key])) {
            $value = $settings[$key] ? 'true' : 'false';
            $content = preg_replace(
                "/\\\$GLOBALS\['$globalKey'\]\s*=\s*(true|false)/",
                "\$GLOBALS['$globalKey'] = $value",
                $content
            );
        }
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
        "/\\\$GLOBALS\['EXCLUDED_RELEASE_GROUPS'\]\s*=\s*'[^']*'/",
        "\$GLOBALS['EXCLUDED_RELEASE_GROUPS'] = '$releaseGroups'",
        $content
    );
    
    $content = preg_replace(
        "/\\\$GLOBALS\['EXCLUDED_LANGUAGES'\]\s*=\s*'[^']*'/",
        "\$GLOBALS['EXCLUDED_LANGUAGES'] = '$languages'",
        $content
    );
    
    $content = preg_replace(
        "/\\\$GLOBALS\['EXCLUDED_QUALITIES'\]\s*=\s*'[^']*'/",
        "\$GLOBALS['EXCLUDED_QUALITIES'] = '$qualities'",
        $content
    );
    
    $content = preg_replace(
        "/\\\$GLOBALS\['EXCLUDED_CUSTOM'\]\s*=\s*'[^']*'/",
        "\$GLOBALS['EXCLUDED_CUSTOM'] = '$custom'",
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
 * Populate streams from GitHub lists using Comet/MediaFusion providers
 * This runs in CLI mode after saving settings
 */
function populateStreamsFromLists() {
    global $PRIVATE_TOKEN, $apiKey;
    
    $statusFile = __DIR__ . '/cache/stream_populate_status.json';
    $logFile = __DIR__ . '/logs/stream_populate.log';
    
    $log = function($msg) use ($logFile) {
        $timestamp = date('Y-m-d H:i:s');
        echo "[$timestamp] $msg\n";
        @file_put_contents($logFile, "[$timestamp] $msg\n", FILE_APPEND);
    };
    
    $updateStatus = function($status, $progress, $details = '') use ($statusFile) {
        file_put_contents($statusFile, json_encode([
            'status' => $status,
            'progress' => $progress,
            'details' => $details,
            'updated' => date('Y-m-d H:i:s')
        ], JSON_PRETTY_PRINT));
    };
    
    $log("=== STARTING STREAM POPULATION ===");
    $updateStatus('running', 0, 'Starting stream population...');
    
    // GitHub list URLs
    $movieLists = [
        'now_playing' => 'https://raw.githubusercontent.com/Zerr0-C00L/public-files/main/movie_lists/now_playing_movies.json',
        'popular' => 'https://raw.githubusercontent.com/Zerr0-C00L/public-files/main/movie_lists/popular_movies.json',
        'top_rated' => 'https://raw.githubusercontent.com/Zerr0-C00L/public-files/main/movie_lists/top_rated_movies.json',
        'upcoming' => 'https://raw.githubusercontent.com/Zerr0-C00L/public-files/main/movie_lists/upcoming_movies.json',
        'latest_releases' => 'https://raw.githubusercontent.com/Zerr0-C00L/public-files/main/movie_lists/latest_releases_movies.json'
    ];
    
    // Check which lists are enabled in config
    $enabledLists = [];
    if ($GLOBALS['INCLUDE_NOW_PLAYING'] ?? false) $enabledLists['now_playing'] = $movieLists['now_playing'];
    if ($GLOBALS['INCLUDE_POPULAR_MOVIES'] ?? false) $enabledLists['popular'] = $movieLists['popular'];
    if ($GLOBALS['INCLUDE_TOP_RATED_MOVIES'] ?? false) $enabledLists['top_rated'] = $movieLists['top_rated'];
    if ($GLOBALS['INCLUDE_UPCOMING'] ?? false) $enabledLists['upcoming'] = $movieLists['upcoming'];
    if ($GLOBALS['INCLUDE_LATEST_RELEASES_MOVIES'] ?? false) $enabledLists['latest_releases'] = $movieLists['latest_releases'];
    
    if (empty($enabledLists)) {
        $log("No movie lists enabled, nothing to populate");
        $updateStatus('complete', 100, 'No lists enabled');
        return;
    }
    
    $log("Enabled lists: " . implode(', ', array_keys($enabledLists)));
    
    $db = new EpisodeCacheDB();
    $rdKey = $PRIVATE_TOKEN;
    $processed = 0;
    $cached = 0;
    $skipped = 0;
    $errors = 0;
    $totalMovies = 0;
    
    // First pass: count total movies
    foreach ($enabledLists as $listName => $url) {
        $log("Fetching $listName list...");
        $response = @file_get_contents($url);
        if ($response) {
            $data = json_decode($response, true) ?? [];
            // Handle both formats: direct array or {movies: [...]}
            $movies = isset($data['movies']) ? $data['movies'] : $data;
            $totalMovies += count($movies);
        }
    }
    $log("Total movies to process: $totalMovies");
    
    // Second pass: process each list
    foreach ($enabledLists as $listName => $url) {
        $log("Processing $listName list...");
        $updateStatus('running', round(($processed / max($totalMovies, 1)) * 100), "Processing $listName...");
        
        $response = @file_get_contents($url);
        if (!$response) {
            $log("Failed to fetch $listName");
            continue;
        }
        
        $data = json_decode($response, true) ?? [];
        // Handle both formats: direct array or {movies: [...]}
        $movies = isset($data['movies']) ? $data['movies'] : $data;
        $log("Found " . count($movies) . " movies in $listName");
        
        foreach ($movies as $movie) {
            $processed++;
            // Handle both formats: id (from GitHub lists) or stream_id (from playlist.json)
            $tmdbId = $movie['id'] ?? $movie['stream_id'] ?? $movie['tmdb_id'] ?? null;
            $name = $movie['title'] ?? $movie['name'] ?? 'Unknown';
            
            if (!$tmdbId) {
                $skipped++;
                continue;
            }
            
            // Get IMDB ID from cache or TMDB API
            $imdbId = null;
            $movieData = $db->getMovie($tmdbId);
            if ($movieData && !empty($movieData['imdb_id'])) {
                $imdbId = $movieData['imdb_id'];
            } else {
                // Fetch from TMDB
                $tmdbUrl = "https://api.themoviedb.org/3/movie/{$tmdbId}/external_ids?api_key={$apiKey}";
                $extIds = @file_get_contents($tmdbUrl);
                if ($extIds) {
                    $ids = json_decode($extIds, true);
                    $imdbId = $ids['imdb_id'] ?? null;
                    if ($imdbId) {
                        $db->setMovie($tmdbId, $name, $imdbId, 0);
                    }
                }
                usleep(100000); // 100ms rate limit for TMDB
            }
            
            if (!$imdbId) {
                $log("No IMDB ID for: $name (TMDB: $tmdbId)");
                $skipped++;
                continue;
            }
            
            // Check if we already have streams cached
            if ($db->hasValidStreams($imdbId, 'movie', null, null, 168)) { // 7 days TTL
                $skipped++;
                continue;
            }
            
            // Fetch streams from Comet only (MediaFusion has strict rate limits)
            $streams = fetchStreamsFromProviderForPopulate('comet', $imdbId, 'movie', null, null, $rdKey);
            
            if (empty($streams)) {
                $log("No streams found for: $name ($imdbId)");
                $errors++;
                // Add delay even on error to avoid hammering API
                usleep(1000000); // 1 second
                continue;
            }
            
            // Process and cache streams
            $streamsToCache = [];
            foreach ($streams as $s) {
                $sName = $s['name'] ?? '';
                if (strpos($sName, '[RD+]') !== false || strpos($sName, 'RD') !== false || strpos($sName, '⚡') !== false) {
                    $sTitle = $s['title'] ?? $s['description'] ?? '';
                    $sQuality = 'unknown';
                    if (preg_match('/\b(4K|2160p|UHD)\b/i', $sTitle)) $sQuality = '2160P';
                    elseif (preg_match('/\b1080p\b/i', $sTitle)) $sQuality = '1080P';
                    elseif (preg_match('/\b720p\b/i', $sTitle)) $sQuality = '720P';
                    elseif (preg_match('/\b480p\b/i', $sTitle)) $sQuality = '480P';
                    
                    $hash = $s['infoHash'] ?? '';
                    if (empty($hash) && !empty($s['url'])) {
                        if (preg_match('/\/([a-f0-9]{40})\//i', $s['url'], $hMatch)) {
                            $hash = $hMatch[1];
                        }
                    }
                    
                    $streamsToCache[] = [
                        'quality' => $sQuality,
                        'title' => $sTitle,
                        'hash' => $hash,
                        'file_idx' => $s['fileIdx'] ?? 0,
                        'resolve_url' => $s['url'] ?? ''
                    ];
                }
            }
            
            if (!empty($streamsToCache)) {
                $db->saveStreams($imdbId, 'movie', $streamsToCache);
                $cached++;
                $log("Cached " . count($streamsToCache) . " streams for: $name ($imdbId)");
            }
            
            $updateStatus('running', round(($processed / max($totalMovies, 1)) * 100), "Processed: $processed / $totalMovies");
            usleep(2000000); // 2 seconds between requests to avoid rate limiting
        }
    }
    
    $log("=== STREAM POPULATION COMPLETE ===");
    $log("Processed: $processed, Cached: $cached, Skipped: $skipped, Errors: $errors");
    $updateStatus('complete', 100, "Cached $cached movies, skipped $skipped, errors $errors");
}

/**
 * Fetch streams from a provider for population (standalone function for CLI)
 */
function fetchStreamsFromProviderForPopulate($provider, $imdbId, $type, $season, $episode, $rdKey) {
    $url = '';
    
    switch ($provider) {
        case 'comet':
            $cometConfig = [
                'indexers' => $GLOBALS['COMET_INDEXERS'] ?? ['bktorrent', 'thepiratebay', 'yts', 'eztv'],
                'debridService' => 'realdebrid',
                'debridApiKey' => $rdKey
            ];
            $configBase64 = base64_encode(json_encode($cometConfig));
            $url = "https://comet.elfhosted.com/{$configBase64}/stream/movie/{$imdbId}.json";
            break;
            
        case 'mediafusion':
            $mfConfig = [
                'streaming_provider' => [
                    'token' => $rdKey,
                    'service' => 'realdebrid'
                ],
                'selected_catalogs' => ['torrentio_streams'],
                'enable_catalogs' => false
            ];
            $configBase64 = base64_encode(json_encode($mfConfig));
            $url = "https://mediafusion.elfhosted.com/{$configBase64}/stream/movie/{$imdbId}.json";
            break;
    }
    
    $ch = curl_init($url);
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_TIMEOUT => 15,
        CURLOPT_USERAGENT => 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
        CURLOPT_FOLLOWLOCATION => true
    ]);
    $response = curl_exec($ch);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);
    
    if ($httpCode !== 200 || !$response) {
        return [];
    }
    
    $data = json_decode($response, true);
    $streams = $data['streams'] ?? [];
    
    // Normalize stream names
    foreach ($streams as &$stream) {
        $name = $stream['name'] ?? '';
        if ($provider === 'comet' && strpos($name, '[RD⚡]') !== false) {
            $stream['name'] = str_replace('[RD⚡]', '[RD+]', $name);
        }
        if ($provider === 'mediafusion') {
            if ((strpos($name, '⚡') !== false || strpos($name, 'RD') !== false) && strpos($name, '[RD+]') === false) {
                $stream['name'] = '[RD+] ' . $name;
            }
        }
    }
    
    return $streams;
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
    
    <a class="nav-item" data-page="providers">
        <svg fill="currentColor" viewBox="0 0 20 20"><path d="M5.5 16a3.5 3.5 0 01-.369-6.98 4 4 0 117.753-1.977A4.5 4.5 0 1113.5 16h-8z"></path></svg>
        Providers
    </a>
    
    <a class="nav-item" data-page="logs">
        <svg fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clip-rule="evenodd"></path></svg>
        Logs
    </a>
    
    <a class="nav-item" data-page="filters">
        <svg fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M3 3a1 1 0 011-1h12a1 1 0 011 1v3a1 1 0 01-.293.707L12 11.414V15a1 1 0 01-.293.707l-2 2A1 1 0 018 17v-5.586L3.293 6.707A1 1 0 013 6V3z" clip-rule="evenodd"></path></svg>
        Filters
    </a>
    
    <a class="nav-item" data-page="settings">
        <svg fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd"></path></svg>
        Settings
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
            </div>
            <div class="stat-card">
                <div class="stat-value" id="stat-series">-</div>
                <div class="stat-label">TV Series</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="stat-episodes">-</div>
                <div class="stat-label">Cached Episodes</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="stat-m3u">-</div>
                <div class="stat-label">M3U8 Entries</div>
            </div>
        </div>
        
        <div class="card" style="margin-top: 1rem;">
            <div class="card-header">
                <span class="card-title">Quick Actions</span>
            </div>
            <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
                <button class="btn btn-primary" onclick="runAction('sync-now')">
                    <svg width="16" height="16" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1z" clip-rule="evenodd"></path></svg>
                    Sync from GitHub
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
        </div>
        
        <div class="card">
            <div class="card-header">
                <span class="card-title">Background Daemons</span>
            </div>
            
            <div class="daemon-row">
                <div class="daemon-info">
                    <h4>Background Sync Daemon</h4>
                    <p>Syncs playlists from GitHub every 6 hours</p>
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
                    <h4>Auto Playlist Generator</h4>
                    <p>Runs daily at 3 AM via cron job</p>
                    <p style="margin-top: 0.5rem;">
                        Status: <span class="status-badge status-warning">Scheduled (Cron)</span>
                    </p>
                </div>
                <div class="daemon-actions">
                    <button class="btn btn-primary" onclick="runAction('generate-playlist')">Run Now</button>
                </div>
            </div>
        </div>
    </div>
    
    <!-- Providers Page -->
    <div id="page-providers" class="page hidden">
        <div class="page-header">
            <h2>Stream Providers</h2>
            <button class="btn btn-secondary" onclick="testAllProviders()">Test All</button>
        </div>
        
        <div class="card">
            <div class="card-header">
                <span class="card-title">Provider Status</span>
            </div>
            
            <div class="provider-row">
                <div>
                    <h4>Comet</h4>
                    <p style="font-size: 0.85rem; color: var(--text-secondary);">Works on datacenter IPs (Hetzner, DO, etc.)</p>
                </div>
                <div class="provider-status">
                    <span id="provider-comet-status" class="status-badge status-stopped">Not Tested</span>
                    <button class="btn btn-secondary" onclick="testProvider('comet')">Test</button>
                </div>
            </div>
            
            <!-- MediaFusion and Torrentio removed - Comet is the only reliable provider -->
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
                💡 <strong>Full Library Mode:</strong> Enable "Use GitHub Cache" for 50k+ movies & 17k+ series.<br>
                💡 <strong>Curated Mode:</strong> Disable "Use GitHub Cache" and enable specific TMDB lists below for a smaller, focused library.
            </div>
            
            <div class="settings-form">
                <div class="form-row">
                    <div class="form-group">
                        <label>Use GitHub Cache (Full Library)</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-useGithubForCache">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">📚 Pull ~50k movies + ~17k series from GitHub</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Create Own Playlist (TMDB API)</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-userCreatePlaylist">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">🔄 Fetch directly from TMDB API (uses API quota)</span>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>Total Pages (for Create Own)</label>
                        <input type="number" id="setting-totalPages" min="1" max="50" value="5">
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">Only used when "Create Own Playlist" is ON</span>
                    </div>
                    
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
                </div>
                
                <div class="form-row">
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
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">Limit M3U8 size for IPTV apps (use lower for initial scan)</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Auto Sync Interval (hours)</label>
                        <input type="number" id="setting-autoCacheInterval" min="0" max="72" value="6">
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">Sync all enabled content sources (0 = manual only)</span>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Content Options Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">📦 Additional Content</span>
            </div>
            
            <div style="background: var(--bg-tertiary); padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.85rem; color: var(--text-secondary);">
                💡 These options add extra content on top of your main playlist (works with both Full Library and Curated modes).
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
                        <label>Include Collections</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeCollections">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">🎬 ~5,000 movies (Marvel, Star Wars, Harry Potter, etc.)</span>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>Include Adult Content</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeAdult">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">⚠️ Adds ~10,000 adult movies</span>
                    </div>
                    
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
        
        <!-- Movie Lists Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">🎬 TMDB Movie Lists</span>
            </div>
            
            <div style="background: var(--bg-tertiary); padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.85rem; color: var(--text-secondary);">
                💡 Curated movie lists from TMDB (~500 movies each). Updated daily via GitHub Actions. Great for smaller, focused libraries!
            </div>
            
            <div class="settings-form">
                <div class="form-row">
                    <div class="form-group">
                        <label>Latest Releases Movies</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeLatestReleasesMovies">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">📀 Digital, Physical & TV releases (last 6 weeks)</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Now Playing</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeNowPlaying">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">🎭 Movies currently in theaters</span>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>Popular Movies</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includePopularMovies">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">🔥 Currently trending movies</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Top Rated Movies</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeTopRatedMovies">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">⭐ Highest rated movies of all time</span>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>Upcoming Movies</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeUpcoming">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">🎬 Coming soon to theaters</span>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Series Lists Section -->
        <div class="card">
            <div class="card-header">
                <span class="card-title">📺 TMDB Series Lists</span>
            </div>
            
            <div style="background: var(--bg-tertiary); padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; font-size: 0.85rem; color: var(--text-secondary);">
                💡 Curated TV series lists from TMDB (~500 series each). Updated daily via GitHub Actions. Great for smaller, focused libraries!
            </div>
            
            <div class="settings-form">
                <div class="form-row">
                    <div class="form-group">
                        <label>Latest Releases Series</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeLatestReleasesSeries">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">📀 New series premieres (last 6 weeks)</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Airing Today</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeAiringToday">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">📅 TV series with episodes airing today</span>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>On The Air</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeOnTheAir">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">📆 Episodes airing in next 7 days</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Popular Series</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includePopularSeries">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">🔥 Currently trending TV series</span>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>Top Rated Series</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-includeTopRatedSeries">
                            <span class="toggle-slider"></span>
                        </label>
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">⭐ Highest rated series of all time</span>
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
                    <label>Stream Provider</label>
                    <div style="display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.5rem;">
                        <label style="display: flex; align-items: center; gap: 0.5rem; background: var(--bg-tertiary); padding: 0.5rem 1rem; border-radius: 6px; cursor: pointer;">
                            <input type="checkbox" id="provider-comet" checked disabled> Comet (Active)
                        </label>
                    </div>
                    <span style="font-size: 0.75rem; color: var(--text-secondary);">Comet is the only reliable provider for datacenter IPs. MediaFusion has rate limits, Torrentio is blocked by Cloudflare.</span>
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
        
        const useGithubEl = document.getElementById('setting-useGithubForCache');
        if (useGithubEl) useGithubEl.checked = status.config.useGithubForCache === true;
        
        const createPlaylistEl = document.getElementById('setting-userCreatePlaylist');
        if (createPlaylistEl) createPlaylistEl.checked = status.config.userCreatePlaylist === true;
        
        // Content options
        const liveTVEl = document.getElementById('setting-includeLiveTV');
        if (liveTVEl) liveTVEl.checked = status.config.includeLiveTV !== false;
        
        const collectionsEl = document.getElementById('setting-includeCollections');
        if (collectionsEl) collectionsEl.checked = status.config.includeCollections !== false;
        
        const adultEl = document.getElementById('setting-includeAdult');
        if (adultEl) adultEl.checked = status.config.includeAdult === true;
        
        const debugEl = document.getElementById('setting-debugMode');
        if (debugEl) debugEl.checked = status.config.debugMode === true;
        
        // Movie lists
        const nowPlayingEl = document.getElementById('setting-includeNowPlaying');
        if (nowPlayingEl) nowPlayingEl.checked = status.config.includeNowPlaying === true;
        
        const popularMoviesEl = document.getElementById('setting-includePopularMovies');
        if (popularMoviesEl) popularMoviesEl.checked = status.config.includePopularMovies === true;
        
        const topRatedMoviesEl = document.getElementById('setting-includeTopRatedMovies');
        if (topRatedMoviesEl) topRatedMoviesEl.checked = status.config.includeTopRatedMovies === true;
        
        const upcomingEl = document.getElementById('setting-includeUpcoming');
        if (upcomingEl) upcomingEl.checked = status.config.includeUpcoming === true;
        
        const latestReleasesMoviesEl = document.getElementById('setting-includeLatestReleasesMovies');
        if (latestReleasesMoviesEl) latestReleasesMoviesEl.checked = status.config.includeLatestReleasesMovies === true;
        
        // Series lists
        const airingTodayEl = document.getElementById('setting-includeAiringToday');
        if (airingTodayEl) airingTodayEl.checked = status.config.includeAiringToday === true;
        
        const onTheAirEl = document.getElementById('setting-includeOnTheAir');
        if (onTheAirEl) onTheAirEl.checked = status.config.includeOnTheAir === true;
        
        const popularSeriesEl = document.getElementById('setting-includePopularSeries');
        if (popularSeriesEl) popularSeriesEl.checked = status.config.includePopularSeries === true;
        
        const topRatedSeriesEl = document.getElementById('setting-includeTopRatedSeries');
        if (topRatedSeriesEl) topRatedSeriesEl.checked = status.config.includeTopRatedSeries === true;
        
        const latestReleasesSeriesEl = document.getElementById('setting-includeLatestReleasesSeries');
        if (latestReleasesSeriesEl) latestReleasesSeriesEl.checked = status.config.includeLatestReleasesSeries === true;
        
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
        useGithubForCache: document.getElementById('setting-useGithubForCache')?.checked || false,
        userCreatePlaylist: document.getElementById('setting-userCreatePlaylist')?.checked || false,
        
        // Content options
        includeLiveTV: document.getElementById('setting-includeLiveTV')?.checked || false,
        includeCollections: document.getElementById('setting-includeCollections')?.checked || false,
        includeAdult: document.getElementById('setting-includeAdult')?.checked || false,
        debugMode: document.getElementById('setting-debugMode')?.checked || false,
        
        // Movie lists
        includeNowPlaying: document.getElementById('setting-includeNowPlaying')?.checked || false,
        includePopularMovies: document.getElementById('setting-includePopularMovies')?.checked || false,
        includeTopRatedMovies: document.getElementById('setting-includeTopRatedMovies')?.checked || false,
        includeUpcoming: document.getElementById('setting-includeUpcoming')?.checked || false,
        includeLatestReleasesMovies: document.getElementById('setting-includeLatestReleasesMovies')?.checked || false,
        
        // Series lists
        includeAiringToday: document.getElementById('setting-includeAiringToday')?.checked || false,
        includeOnTheAir: document.getElementById('setting-includeOnTheAir')?.checked || false,
        includePopularSeries: document.getElementById('setting-includePopularSeries')?.checked || false,
        includeTopRatedSeries: document.getElementById('setting-includeTopRatedSeries')?.checked || false,
        includeLatestReleasesSeries: document.getElementById('setting-includeLatestReleasesSeries')?.checked || false,
        
        // Regional settings
        language: document.getElementById('setting-language')?.value || 'en-US',
        seriesCountry: document.getElementById('setting-seriesCountry')?.value || '',
        moviesCountry: document.getElementById('setting-moviesCountry')?.value || '',
        userSetHost: document.getElementById('setting-userSetHost')?.value || '',
        
        // Stream providers
        streamProviders: getSelectedProviders(),
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
            showToast('Settings saved! Starting sync...', 'success');
            settingsDirty = false; // Reset dirty flag after successful save
            
            // Auto-sync from GitHub after saving list settings
            triggerAutoSync();
        } else {
            showToast('Failed to save settings: ' + (result.error || 'Unknown error'), 'error');
        }
    } catch (error) {
        showToast('Error: ' + error.message, 'error');
    }
}

// Auto-sync after saving settings
async function triggerAutoSync() {
    try {
        showToast('Syncing playlists from GitHub...', 'success');
        const response = await fetch('?api=sync-github');
        const result = await response.json();
        
        if (result.success) {
            showToast('Sync complete! Stream population running in background...', 'success');
            // Start polling for stream population status
            pollStreamPopulationStatus();
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

function getSelectedProviders() {
    // Only Comet is supported - MediaFusion has rate limits, Torrentio is blocked
    return ['comet'];
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

// Initial load
refreshStatus();
setInterval(refreshStatus, 30000); // Auto-refresh every 30 seconds
</script>
<?php endif; ?>

</body>
</html>
