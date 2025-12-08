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
            
        case 'test-provider':
            $provider = $_GET['provider'] ?? 'comet';
            $result = testProvider($provider);
            echo json_encode($result);
            break;
            
        default:
            echo json_encode(['error' => 'Unknown action']);
    }
    exit;
}

function getSystemStatus() {
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
        $status['daemons']['background_sync']['last_sync'] = $syncStatus['last_sync'] ?? 'Never';
        $status['daemons']['background_sync']['next_sync'] = $syncStatus['next_sync'] ?? 'Unknown';
        $status['daemons']['background_sync']['movies_count'] = $syncStatus['movies_count'] ?? 0;
        $status['daemons']['background_sync']['series_count'] = $syncStatus['series_count'] ?? 0;
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
        'totalPages' => $GLOBALS['totalPages'] ?? 5,
        'useGithubForCache' => $GLOBALS['useGithubForCache'] ?? true,
        'useRealDebrid' => $GLOBALS['useRealDebrid'] ?? false,
        'maxResolution' => $GLOBALS['maxResolution'] ?? 1080,
        'language' => $GLOBALS['language'] ?? 'en-US'
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
    
    $mappings = [
        'totalPages' => '/\$totalPages\s*=\s*\d+/',
        'useGithubForCache' => '/\$useGithubForCache\s*=\s*(true|false)/',
        'maxResolution' => '/\$maxResolution\s*=\s*\d+/',
        'useRealDebrid' => '/\$useRealDebrid\s*=\s*(true|false)/'
    ];
    
    foreach ($settings as $key => $value) {
        if (isset($mappings[$key])) {
            if (is_bool($value)) {
                $value = $value ? 'true' : 'false';
            }
            $content = preg_replace($mappings[$key], "\$$key = $value", $content);
        }
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
                <button class="btn btn-primary" onclick="runAction('generate-playlist')">
                    <svg width="16" height="16" fill="currentColor" viewBox="0 0 20 20"><path d="M4 3a2 2 0 100 4h12a2 2 0 100-4H4z"></path><path fill-rule="evenodd" d="M3 8h14v7a2 2 0 01-2 2H5a2 2 0 01-2-2V8z" clip-rule="evenodd"></path></svg>
                    Generate Playlist
                </button>
                <button class="btn btn-secondary" onclick="runAction('cache-episodes')">
                    <svg width="16" height="16" fill="currentColor" viewBox="0 0 20 20"><path d="M3 12v3c0 1.657 3.134 3 7 3s7-1.343 7-3v-3c0 1.657-3.134 3-7 3s-7-1.343-7-3z"></path><path d="M3 7v3c0 1.657 3.134 3 7 3s7-1.343 7-3V7c0 1.657-3.134 3-7 3S3 8.657 3 7z"></path><path d="M17 5c0 1.657-3.134 3-7 3S3 6.657 3 5s3.134-3 7-3 7 1.343 7 3z"></path></svg>
                    Cache Episodes
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
            
            <div class="provider-row">
                <div>
                    <h4>MediaFusion</h4>
                    <p style="font-size: 0.85rem; color: var(--text-secondary);">ElfHosted instance, datacenter friendly</p>
                </div>
                <div class="provider-status">
                    <span id="provider-mediafusion-status" class="status-badge status-stopped">Not Tested</span>
                    <button class="btn btn-secondary" onclick="testProvider('mediafusion')">Test</button>
                </div>
            </div>
            
            <div class="provider-row">
                <div>
                    <h4>Torrentio</h4>
                    <p style="font-size: 0.85rem; color: var(--text-secondary);">May be blocked by Cloudflare on datacenter IPs</p>
                </div>
                <div class="provider-status">
                    <span id="provider-torrentio-status" class="status-badge status-stopped">Not Tested</span>
                    <button class="btn btn-secondary" onclick="testProvider('torrentio')">Test</button>
                </div>
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
        </div>
        
        <div class="card">
            <div class="card-header">
                <span class="card-title">Playlist Settings</span>
            </div>
            
            <div class="settings-form">
                <div class="form-row">
                    <div class="form-group">
                        <label>Total Pages (per category)</label>
                        <input type="number" id="setting-totalPages" min="1" max="50" value="5">
                        <span style="font-size: 0.75rem; color: var(--text-secondary);">5 pages ≈ 3,000 series</span>
                    </div>
                    
                    <div class="form-group">
                        <label>Max Resolution</label>
                        <select id="setting-maxResolution">
                            <option value="2160">4K (2160p)</option>
                            <option value="1080">1080p</option>
                            <option value="720">720p</option>
                            <option value="480">480p</option>
                        </select>
                    </div>
                </div>
                
                <div class="form-row">
                    <div class="form-group">
                        <label>Use GitHub for Cache</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-useGithubForCache" checked>
                            <span class="toggle-slider"></span>
                        </label>
                    </div>
                    
                    <div class="form-group">
                        <label>Use Real-Debrid</label>
                        <label class="toggle-switch">
                            <input type="checkbox" id="setting-useRealDebrid" checked>
                            <span class="toggle-slider"></span>
                        </label>
                    </div>
                </div>
                
                <button class="btn btn-primary" onclick="saveSettings()">Save Settings</button>
            </div>
        </div>
        
        <div class="card">
            <div class="card-header">
                <span class="card-title">Connection Info</span>
            </div>
            <div style="background: var(--bg-tertiary); padding: 1rem; border-radius: 6px; font-family: monospace;">
                <p><strong>Server URL:</strong> <?php echo (isset($_SERVER['HTTPS']) ? 'https' : 'http') . '://' . $_SERVER['HTTP_HOST']; ?></p>
                <p style="margin-top: 0.5rem;"><strong>Xtream API:</strong> <?php echo (isset($_SERVER['HTTPS']) ? 'https' : 'http') . '://' . $_SERVER['HTTP_HOST']; ?>/player_api.php</p>
                <p style="margin-top: 0.5rem;"><strong>Username:</strong> user</p>
                <p style="margin-top: 0.5rem;"><strong>Password:</strong> pass</p>
            </div>
        </div>
    </div>
</main>

<script>
let currentStatus = {};

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
    
    // Settings
    if (status.config) {
        document.getElementById('setting-totalPages').value = status.config.totalPages;
        document.getElementById('setting-maxResolution').value = status.config.maxResolution;
        document.getElementById('setting-useGithubForCache').checked = status.config.useGithubForCache;
        document.getElementById('setting-useRealDebrid').checked = status.config.useRealDebrid;
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
async function saveSettings() {
    const settings = {
        totalPages: parseInt(document.getElementById('setting-totalPages').value),
        maxResolution: parseInt(document.getElementById('setting-maxResolution').value),
        useGithubForCache: document.getElementById('setting-useGithubForCache').checked,
        useRealDebrid: document.getElementById('setting-useRealDebrid').checked
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
        } else {
            showToast('Failed to save settings', 'error');
        }
    } catch (error) {
        showToast('Error: ' + error.message, 'error');
    }
}

// Toast notifications
function showToast(message, type = 'success') {
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);
    
    setTimeout(() => toast.remove(), 3000);
}

// Initial load
refreshStatus();
setInterval(refreshStatus, 30000); // Auto-refresh every 30 seconds
</script>
<?php endif; ?>

</body>
</html>
