<?php
// Created By gogetta.teams@gmail.com
// Please leave this in this script.
//https://github.com/Zerr0-C00L/tmdb-to-vod-playlist

/// Set to true for debugging. Set to false to run as production.
$GLOBALS['DEBUG'] = false; // Developer option.

// This script no longer by default loads the user created playlist. If you would prefer to create your own playlist
// change the setting $userCreatePlaylist = true;

// Next, go into the HeadlessVidX/Install Instructions.txt and follow the instructions on setting up HeadlessVidX. TheTvApp // which is a Live TV playlist needs to have this installed before it can be used. 

// Replace 'YOUR_API_KEY' with your TMDb API key - https://www.themoviedb.org/
//Entering your key here may be visible through google drive. Check your sharing settings.
$apiKey = 'YOUR_TMDB_API_KEY';

// Replace this with your Real-Debrid Private API token - https://real-debrid.com/apitoken
// Don't worry about this setting if you aren't planning on using Real Debrid.
$PRIVATE_TOKEN = 'YOUR_REAL_DEBRID_API_TOKEN';

// Replace this with your Premiumize Private API token - https://www.premiumize.me/account
// Don't worry about this setting if you aren't planning on using Premiumize.
$premiumizeApiKey = '';

// By default, on a local network the server identifies as "localhost" or "127.0.0.1" which isn't
// accessible from other devices in your local network. Set this if you're running the script on
// a local server and want to access it from other devices (firestick, android, etc. If so, specify
// the server's local IP (e.g., '192.168.x.x') for network access. Leave this blank for default server
// address or if installing on a public accessibe server.
$userSetHost = ''; // Example: 192.168.0.100 see the help file or video for more information.

// Note: The $HTTP_PROXY is utilized only during the scraping of direct movie links. This is particularly necessary if you are making a large number of requests to obtain streaming links, such as when running this script as a service. It is recommended to use backconnect proxies from providers like stormproxies.com to avoid being blocked by streaming websites.
$HTTP_PROXY = "";

//Enable or disable the $HTTP_PROXY setting.
$USE_HTTP_PROXY = false;

//When set to true your playist is created by running the 'create_playlist.php' and 'create_tv_playlist.php'
//When set to false the the movie and tv show playlist will be loaded from github. The playlists on github 
//are around 45k movies and around 12k series.
$userCreatePlaylist = true; // Set to true to create your own playlist

// Adds approximately 10,000 full-length adult movies to the VOD Movie playlist
//under the category 'XXX Adult Movies'. This playlist is refreshed every Sunday.
$INCLUDE_ADULT_VOD = false; // Set to true to include adult content.

// Set how many movies and TV series you want in your playlist. TMDB shows 20 items on each page.
// Pages are fetched PER CATEGORY (networks, genres, etc.), so:
//   5 pages  = ~3,000 series
//   10 pages = ~6,500 series  
//   25 pages = ~17,000 series
// Adjust this for a bigger or smaller playlist.
$totalPages = 5; // Adjust this if needed

// Use GitHub's larger playlist (~17k series) for episode cache instead of local playlist.
// This means even with $totalPages = 5, the background sync will cache episodes for all
// 17,000+ series from GitHub, so any series a user searches for will likely work.
$useGithubForCache = true; // Set to false to only cache episodes for your local playlist

// Auto-cache refresh interval in hours. Set to 0 to disable auto-refresh.
// The server will check for new series and episodes periodically.
$autoCacheIntervalHours = 6; // Check every 6 hours for new content

// Leave blank for any language.
$language = 'en-US'; // TMDB search setting (language)

// Leave blank for any country to be included in the series playlist.
$series_with_origin_country = 'US'; // TMDB search setting (with_origin_country)

// Leave blank for any country to be included in the movies playlist.
$movies_with_origin_country = 'US'; // TMDB search setting (with_origin_country)

// Leave this setting as false if you aren't intending on using Real-Debrid links.
// set it to true if you want to use realdebrid when streaming torrents. 
// Example: The value can be either true or false.
$useRealDebrid = true; // Requires a real debrid private token added above.

// Leave this setting as false if you aren't intending on using Premiumize links.
// set it to true if you want to use premiumize when streaming torrents. 
// Example: The value can be either true or false.
$usePremiumize = false; // Requires a Premiumize API Key added above.

// maxResolution is the upper limit for video resolution preference in
// pixels (e.g., 1080 for 1080p). If no links match this exact resolution,
// the closest available resolution will be selected. If you don't have the
// internet speed for higher quality you should select a lower resolution, or
// you may experience constant freezing and buffering.
// Example: 1080P is 1080
$maxResolution = 2160; // numerical value only

// At this time the maxFileSize is used only for Stremio Sites.
$maxFileSize = 50000; // numerical value in megabytes

// Set the address HeadlessVidX is listening on.
$HeadlessVidX_Address = "localhost:3202"; // Example: ip:port

// HEADLESSVIDX_MAX_THREADS controls the maximum number of concurrent curl requests (threads) 
// that the script will handle simultaneously. Being headless browser operations, higher 
// numbers of concurrent threads will have a greater impact on CPU and memory usage. 
// Adjust this value based on your server's capacity and the desired balance between 
// performance and resource consumption.
$HeadlessVidX_Max_Threads = 5; // Numerical value only.


// Sets the execution order of the sites stored in HeadlessVidX_sitelist. Also, check out the training guide
// at http://localhost:3202/ to learn how to add your own list of streaming api websites to pull links from.
$HeadlessVidXRunOrder = 'random'; // Options: random, ascending, or descending

// The $cacheSize setting is used to control the size of a cache system, ensuring that
// it doesn't grow to large and consume excessive storage space.
// Example 10 MB is 10
$cacheSize = 10; // numerical value only

// Define the cache expiration duration variable (in hours)
// Most of the non real debrid links last around 3 to 4 hours before their
// token expires. So setting this to 3 or 4 hours should be good enough.
$expirationHours = 3; // Default: 3 (numerical value only)

// The timeout setting has only been added to the video link extractors.
// If you set this to low you might not get any links to return.
// Example: 20 seconds is 20
$timeOut = 20; // numerical value only

// Change the run order here. This can be Used to speed up the process of finding a link.
// Cut the entire line and paste it above or below another. The list is ran
// from top to bottom. You can also disable a website by commenting it out with //
// Example: take 'theMovieArchive_site', and put above or below another.
// Be sure to grab the entire line including the comma.

$userDefinedOrder = [
'stremioSites',  // Torrentio - Real-Debrid cached torrents via Stremio addon
//'torrentSites',  // Disabled - old torrent method
//'primewire_tf',  // Disabled - can be slow
//'autoembed_cc',  // Disabled - can be slow
//'HeadlessVidX',  // Disabled - requires separate service
 ];
 
/* Archived list
//'shegu_net_links',
//'warezcdn_com',
//'justBinge_site',
//'vidsrc_to',
//'rive_vidsrc_scrapper',
//'smashyStream_com',
//'showBox_media',
//'vidsrc_rip',
//'myfilestorage_xyz',
//'vidsrc_pro', 
//'twoembed_skin',
//'oneTwothreeEmbed_net',
//'superEmbed_stream',
//'frembed_pro',
//'upMovies_to',
*/
 
// On my todo list.
// Language mapping between TMDB and Torrent Site. 
$languageMapping = [
    "TorrentGalaxy" => [
        "en-US" => "1",    // English (United States)
        "fr-FR" => "2",    // French (France)
        "de-DE" => "3",    // German (Germany)
        "it-IT" => "4",    // Italian (Italy)
        "ja-JP" => "5",    // Japanese (Japan)
        "es-ES" => "6",    // Spanish (Spain)
        "ru-RU" => "7",    // Russian (Russia)
        "nb-NO" => "12",   // Norwegian (Norway)
        "hi-IN" => "8",    // Hindi (India)
        "ko-KR" => "10",   // Korean (South Korea)
        "da-DK" => "11",   // Danish (Denmark)
        "nl-NL" => "13",   // Dutch (Netherlands)
        "zh-CN" => "14",   // Chinese (Simplified, China)
        "pt-PT" => "15",   // Portuguese (Portugal)
        "pl-PL" => "17",   // Polish (Poland)
        "tr-TR" => "18",   // Turkish (Turkey)
        "te-IN" => "19",   // Telugu (India)
        "sv-SE" => "22",   // Swedish (Sweden)
        "cs-CZ" => "26",   // Czech (Czech Republic)
        "ar-SA" => "21",   // Arabic (Saudi Arabia)
        "ro-RO" => "23",   // Romanian (Romania)
        "bn-BD" => "16",   // Bengali (Bangladesh)
        "ur-PK" => "20",   // Urdu (Pakistan)
        "th-TH" => "24",   // Thai (Thailand)
        "ta-IN" => "25",   // Tamil (India)
        "hr-HR" => "27",   // Croatian (Croatia)
        "other" => "9",    // Other / Multiple
    ],
	
	"Glodls" => [
        "en-US" => "1",    // English (United States)
        "fr-FR" => "2",    // French (France)
        "de-DE" => "3",    // German (Germany)
        "it-IT" => "4",    // Italian (Italy)
        "ja-JP" => "5",    // Japanese (Japan)
        "es-ES" => "6",    // Spanish (Spain)
        "ru-RU" => "7",    // Russian (Russia)
        "nb-NO" => "12",   // Norwegian (Norway)
        "hi-IN" => "8",    // Hindi (India)
        "ko-KR" => "10",   // Korean (South Korea)
        "da-DK" => "11",   // Danish (Denmark)
        "nl-NL" => "13",   // Dutch (Netherlands)
        "zh-CN" => "11",   // Chinese (Simplified, China)
        "pt-PT" => "15",   // Portuguese (Portugal)
        "te-IN" => "14",   // Telugu (India)
        "bn-BD" => "12",   // Bengali (Bangladesh)
        "ta-IN" => "9",    // Tamil (India)
    ]
	
];

function locateBaseURL() {
    global $userSetHost;

    $protocol = (!empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off' || $_SERVER['SERVER_PORT'] == 443) ? "https://" : "http://";

    $domain = isset($userSetHost) && !empty($userSetHost) ? $protocol . $userSetHost : $protocol . $_SERVER['HTTP_HOST'];

    $scriptDir = dirname($_SERVER['SCRIPT_NAME']);
    $scriptDir = ($scriptDir === '/' || $scriptDir === '\\') ? '' : trim($scriptDir, '/\\');

    $baseUrl = rtrim($domain, '/') . '/' . $scriptDir;
    $baseUrl = rtrim($baseUrl, '/') . '/'; // Ensure only one trailing slash

    return $baseUrl;
}


	

function accessLog() {
    $logFile = 'access.log';
	
    $urlComponents = parse_url($_SERVER['REQUEST_URI']);
    $queryString = isset($urlComponents['query']) ? $urlComponents['query'] : '';
	
    parse_str($queryString, $queryParams);
    
    if (!isset($queryParams['dev'])) {
        $queryParams['dev'] = 'true'; // Set only if not already set
    }
	
    $newQueryString = http_build_query($queryParams);
	
    $modifiedUri = $urlComponents['path'];
    if (!empty($newQueryString)) {
        $modifiedUri .= '?' . $newQueryString;
    }
    if (isset($urlComponents['fragment'])) {
        $modifiedUri .= '#' . $urlComponents['fragment'];
    }
    
    // Log the data with the modified URI
    $logData = date('Y-m-d H:i:s') . ' ' . $_SERVER['REMOTE_ADDR'] . ' ' . $_SERVER['REQUEST_METHOD'] . ' ' . $modifiedUri . PHP_EOL;
    file_put_contents($logFile, $logData, FILE_APPEND);
}

// ========== MULTI-QUALITY PLAYLIST SETTINGS ==========
// Enable this to generate separate playlist entries for different quality levels
// When enabled, movies will appear as:
//   - "Movie Name (2025) [4K]" 
//   - "Movie Name (2025) [1080p]"
//   - "Movie Name (2025) [720p]"
// This allows IPTV apps to select quality directly from the movie list
$GLOBALS['ENABLE_QUALITY_VARIANTS'] = false; // Set to true to enable multi-quality entries

// Which qualities to include when ENABLE_QUALITY_VARIANTS is true
// Options: '4k', '1080p', '720p', '480p'
$GLOBALS['QUALITY_VARIANTS'] = ['4k', '1080p', '720p'];

// Show full stream/release name instead of just quality label
// When enabled, entries show the actual torrent name like:
//   "Fight.Club.1999.2160p.BluRay.x265.HEVC.10bit.AAC.5.1-UTR"
// Instead of just "Fight Club (1999) [4K]"
// Requires streams to be cached in SQLite (run check_availability.php first)
$GLOBALS['SHOW_FULL_STREAM_NAME'] = true;

// Alternative approach: Single entry per movie with best quality auto-selected
// Set to a specific quality to always use that (e.g., '1080p')
// Set to null to auto-select best available quality
$GLOBALS['DEFAULT_QUALITY'] = null;

// ========== STREAM PROVIDER SETTINGS ==========
// Configure multiple stream providers with fallback support
// Order determines priority - first working provider is used
// Options: 'comet', 'mediafusion', 'torrentio'
$GLOBALS['STREAM_PROVIDERS'] = ['comet', 'mediafusion', 'torrentio'];

// Comet configuration (works well with datacenter IPs like Hetzner)
// Base64 encoded config with RD key will be auto-generated
$GLOBALS['COMET_INDEXERS'] = ['bktorrent', 'thepiratebay', 'yts', 'eztv', 'kickasstorrents'];

// MediaFusion configuration
// Uses ElfHosted instance which is datacenter-friendly
$GLOBALS['MEDIAFUSION_ENABLED'] = true;

// Torrentio configuration (may be blocked by Cloudflare on some IPs)
// Kept as fallback for residential IPs
$GLOBALS['TORRENTIO_PROVIDERS'] = 'yts,eztv,rarbg,1337x,thepiratebay,kickasstorrents,torrentgalaxy,magnetdl';

// ========== RELEASE FILTERING SETTINGS ==========
// Filter out unwanted releases by release group, language tags, or custom patterns
// These patterns are checked against the stream title/name

// Excluded release groups/sources (e.g., TVHUB, FILM for Russian releases)
// Add groups separated by | (pipe) - case insensitive
// Examples: 'TVHUB|FILM' to block Russian releases, 'YTS' to block YTS releases
$GLOBALS['EXCLUDED_RELEASE_GROUPS'] = 'TVHUB|FILM';

// Excluded language tags in release names
// Add language indicators separated by | (pipe) - case insensitive  
// Examples: 'RUSSIAN|RUS|RU' for Russian, 'GERMAN|GER|DE' for German
$GLOBALS['EXCLUDED_LANGUAGES'] = 'RUSSIAN|RUS|HINDI|HIN|GERMAN|GER|FRENCH|FRE|ITALIAN|ITA|SPANISH|SPA|LATINO';

// Quality filters - exclude REMUX, HDR, DV, 3D, CAM, etc.
// These are filtered out to ensure compatibility with most players
$GLOBALS['EXCLUDED_QUALITIES'] = 'REMUX|HDR|DV|Dolby.?Vision|3D|CAM|TS|SCR|HDTS|HDCAM|TELESYNC|TELECINE|TC';

// Custom exclude patterns (regex) - advanced users only
// Add any custom regex pattern to filter out specific releases
// Example: 'Sample|Trailer|\[Dual\]' to exclude samples, trailers, and dual audio
$GLOBALS['EXCLUDED_CUSTOM'] = '';

// Enable/disable filtering (set to false to disable all filtering)
$GLOBALS['ENABLE_RELEASE_FILTERS'] = true;

// ========== MDBLIST INTEGRATION SETTINGS ==========
// MDBList.com provides curated movie/TV lists that can be automatically 
// imported into your playlist. Get your API key from https://mdblist.com/preferences/
// 
// Features:
// - Import from any public MDBList URL
// - Support for custom user lists
// - Automatic TMDB ID resolution
// - Scheduled sync with your configured lists

// Your MDBList API key (optional - needed for private lists and user lists)
// Get it from: https://mdblist.com/preferences/
$GLOBALS['MDBLIST_API_KEY'] = '';

// Enable MDBList integration
$GLOBALS['MDBLIST_ENABLED'] = false;

// MDBList URLs to import (add your favorite lists here)
// Example URLs:
// - https://mdblist.com/lists/linaspuransen/top-watched-movies-of-the-week
// - https://mdblist.com/lists/hdlists/top-ten-pirated-movies-of-the-week
// - https://mdblist.com/lists/garycrawfordgc/top-100-movies-on-imdb
$GLOBALS['MDBLIST_URLS'] = [];

// Auto-sync interval for MDBList (in hours, 0 to disable auto-sync)
// Lists will be re-fetched and merged into your playlist at this interval
$GLOBALS['MDBLIST_SYNC_INTERVAL'] = 6;

// Include MDBList items in main playlist generation
// When true, MDBList items are merged with TMDB lists during playlist generation
$GLOBALS['MDBLIST_MERGE_PLAYLIST'] = true;

 
?>