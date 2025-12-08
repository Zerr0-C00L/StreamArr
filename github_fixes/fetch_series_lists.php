<?php
/**
 * Fetch TMDB TV Series Lists
 * 
 * Fetches TV series from TMDB lists:
 * - Airing Today
 * - On The Air (airing in the next 7 days)
 * - Popular
 * - Top Rated
 * - Latest Releases (Digital, Physical, TV premieres)
 * 
 * Environment Variables:
 * - TMDB_API_KEY: Your TMDB API key
 * - FETCH_LISTS: Comma-separated list of lists to fetch (optional)
 */

$apiKey = getenv('TMDB_API_KEY');
if (!$apiKey) {
    die("Error: TMDB_API_KEY environment variable not set\n");
}

// Calculate date range for latest releases (last 6 weeks)
$dateFrom = date('Y-m-d', strtotime('-6 weeks'));
$dateTo = date('Y-m-d');

// Lists to fetch (can be overridden by environment variable)
$defaultLists = ['airing_today', 'on_the_air', 'popular', 'top_rated', 'latest_releases'];
$fetchLists = getenv('FETCH_SERIES_LISTS') ?: getenv('FETCH_LISTS');
$listsToFetch = $fetchLists ? explode(',', $fetchLists) : $defaultLists;

// Output directory
$outputDir = __DIR__ . '/../series_lists';
if (!is_dir($outputDir)) {
    mkdir($outputDir, 0755, true);
}

// TMDB API base URL
$baseUrl = 'https://api.themoviedb.org/3';
$region = 'US';

/**
 * Fetch data from TMDB API
 */
function fetchFromTMDB($url, $apiKey) {
    $separator = strpos($url, '?') !== false ? '&' : '?';
    $fullUrl = $url . $separator . 'api_key=' . $apiKey;
    
    $context = stream_context_create([
        'http' => [
            'timeout' => 30,
            'header' => "Accept: application/json\r\n"
        ]
    ]);
    
    $response = @file_get_contents($fullUrl, false, $context);
    if ($response === false) {
        return null;
    }
    
    return json_decode($response, true);
}

/**
 * Fetch all pages of a list
 */
function fetchAllPages($endpoint, $apiKey, $baseUrl, $maxPages = 25, $extraParams = []) {
    $allResults = [];
    $page = 1;
    
    while ($page <= $maxPages) {
        // FIXED: Added include_adult=false, region, and with_original_language=en to exclude adult and non-English content
        global $region;
        $url = $baseUrl . $endpoint . '?page=' . $page . '&language=en-US&region=' . $region . '&include_adult=false&with_original_language=en';
        foreach ($extraParams as $param => $value) {
            $url .= '&' . urlencode($param) . '=' . urlencode($value);
        }
        $data = fetchFromTMDB($url, $apiKey);
        
        if (!$data || empty($data['results'])) {
            break;
        }
        
        $allResults = array_merge($allResults, $data['results']);
        
        // Check if we've reached the last page
        if ($page >= ($data['total_pages'] ?? 1)) {
            break;
        }
        
        $page++;
        usleep(100000); // 100ms delay to respect rate limits
    }
    
    return $allResults;
}

/**
 * Enrich series data with additional details
 */
function enrichSeriesData($series) {
    return [
        'id' => $series['id'],
        'name' => $series['name'] ?? '',
        'original_name' => $series['original_name'] ?? '',
        'overview' => $series['overview'] ?? '',
        'first_air_date' => $series['first_air_date'] ?? '',
        'poster_path' => $series['poster_path'] ?? '',
        'backdrop_path' => $series['backdrop_path'] ?? '',
        'vote_average' => $series['vote_average'] ?? 0,
        'vote_count' => $series['vote_count'] ?? 0,
        'popularity' => $series['popularity'] ?? 0,
        'genre_ids' => $series['genre_ids'] ?? [],
        'origin_country' => $series['origin_country'] ?? [],
        'original_language' => $series['original_language'] ?? '',
        'adult' => $series['adult'] ?? false
    ];
}

// Map list names to API endpoints
$listEndpoints = [
    'airing_today' => '/tv/airing_today',
    'on_the_air' => '/tv/on_the_air',
    'popular' => '/tv/popular',
    'top_rated' => '/tv/top_rated',
    'latest_releases' => '/discover/tv'
];

$listDescriptions = [
    'airing_today' => 'TV series airing today',
    'on_the_air' => 'TV series airing in the next 7 days',
    'popular' => 'Popular TV series',
    'top_rated' => 'Top rated TV series',
    'latest_releases' => 'Latest releases (Digital, Physical, Premiere)'
];

// Extra params for discover endpoint - FIXED: Added include_adult=false
$discoverParams = [
    'latest_releases' => [
        'first_air_date.gte' => $dateFrom,
        'first_air_date.lte' => $dateTo,
        'sort_by' => 'first_air_date.desc',
        'with_status' => '0|2|3',  // Returning Series, Planned, In Production, Ended
        'with_type' => '0|1|2|3|4|5|6',  // All types
        'include_adult' => 'false'
    ]
];

echo "=== TMDB TV Series List Fetcher ===\n";
echo "Date: " . date('Y-m-d H:i:s') . "\n";
echo "Latest releases date range: $dateFrom to $dateTo\n\n";

$summary = [];

foreach ($listsToFetch as $listName) {
    $listName = trim($listName);
    
    if (!isset($listEndpoints[$listName])) {
        echo "Warning: Unknown list '$listName', skipping\n";
        continue;
    }
    
    echo "Fetching: {$listDescriptions[$listName]}...\n";
    
    $endpoint = $listEndpoints[$listName];
    $extraParams = $discoverParams[$listName] ?? [];
    $series = fetchAllPages($endpoint, $apiKey, $baseUrl, 25, $extraParams);
    
    if (empty($series)) {
        echo "  Error: No series fetched for $listName\n";
        continue;
    }
    
    // Remove duplicates based on ID and filter adult content
    $uniqueSeries = [];
    $seenIds = [];
    foreach ($series as $show) {
        // FIXED: Skip adult series explicitly
        if (!empty($show['adult']) && $show['adult'] === true) {
            continue;
        }
        
        // FIXED: Skip non-English original language series
        if (!isset($show['original_language']) || $show['original_language'] !== 'en') {
            continue;
        }
        
        if (!in_array($show['id'], $seenIds)) {
            $seenIds[] = $show['id'];
            $uniqueSeries[] = enrichSeriesData($show);
        }
    }
    
    // Sort by popularity
    usort($uniqueSeries, function($a, $b) {
        return $b['popularity'] <=> $a['popularity'];
    });
    
    $outputData = [
        'list_name' => $listName,
        'description' => $listDescriptions[$listName],
        'total_count' => count($uniqueSeries),
        'fetched_at' => date('c'),
        'series' => $uniqueSeries
    ];
    
    $outputFile = $outputDir . '/' . $listName . '_series.json';
    file_put_contents($outputFile, json_encode($outputData, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE));
    
    echo "  Saved " . count($uniqueSeries) . " series to $listName" . "_series.json\n";
    
    $summary[$listName] = count($uniqueSeries);
}

// Create a combined index file
$indexData = [
    'updated_at' => date('c'),
    'lists' => $summary,
    'total_unique_series' => array_sum($summary)
];

file_put_contents($outputDir . '/index.json', json_encode($indexData, JSON_PRETTY_PRINT));

echo "\n=== Summary ===\n";
foreach ($summary as $list => $count) {
    echo "$list: $count series\n";
}
echo "Total: " . array_sum($summary) . " series (with possible overlap)\n";
echo "\nDone!\n";
