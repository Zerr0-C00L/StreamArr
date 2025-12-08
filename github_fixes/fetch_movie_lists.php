<?php
/**
 * Fetch TMDB Movie Lists
 * Fetches: Now Playing, Popular, Top Rated, Upcoming, Latest Releases
 * Run via GitHub Actions on schedule
 */

// Get API key from environment or use default
$apiKey = getenv('TMDB_API_KEY') ?: getenv('SECRET_API_KEY') ?: '';

if (empty($apiKey)) {
    die("Error: TMDB API key not set. Set TMDB_API_KEY or SECRET_API_KEY environment variable.\n");
}

$language = 'en-US';
$region = 'US';
$maxPages = 25; // ~500 movies per list

// Calculate date range for latest releases (last 6 weeks)
$dateFrom = date('Y-m-d', strtotime('-6 weeks'));
$dateTo = date('Y-m-d');

// Movie list endpoints
$lists = [
    'now_playing' => [
        'endpoint' => 'movie/now_playing',
        'name' => 'Now Playing',
        'filename' => 'now_playing_movies.json',
        'type' => 'standard'
    ],
    'popular' => [
        'endpoint' => 'movie/popular', 
        'name' => 'Popular',
        'filename' => 'popular_movies.json',
        'type' => 'standard'
    ],
    'top_rated' => [
        'endpoint' => 'movie/top_rated',
        'name' => 'Top Rated',
        'filename' => 'top_rated_movies.json',
        'type' => 'standard'
    ],
    'upcoming' => [
        'endpoint' => 'movie/upcoming',
        'name' => 'Upcoming',
        'filename' => 'upcoming_movies.json',
        'type' => 'standard'
    ],
    'latest_releases' => [
        'endpoint' => 'discover/movie',
        'name' => 'Latest Releases',
        'filename' => 'latest_releases_movies.json',
        'type' => 'discover',
        'params' => [
            'with_release_type' => '4|5|6',  // 4=Digital, 5=Physical, 6=TV
            'release_date.gte' => $dateFrom,
            'release_date.lte' => $dateTo,
            'sort_by' => 'popularity.desc'
        ]
    ]
];

// Which lists to fetch (can be filtered via environment)
$fetchLists = getenv('FETCH_LISTS') ? explode(',', getenv('FETCH_LISTS')) : array_keys($lists);

echo "TMDB Movie Lists Fetcher\n";
echo "========================\n";
echo "API Key: " . substr($apiKey, 0, 8) . "...\n";
echo "Lists to fetch: " . implode(', ', $fetchLists) . "\n";
echo "Latest releases date range: $dateFrom to $dateTo\n\n";

$outputDir = dirname(__DIR__) . '/movie_lists';
if (!is_dir($outputDir)) {
    mkdir($outputDir, 0755, true);
}

$summary = [];

foreach ($fetchLists as $listKey) {
    $listKey = trim($listKey);
    if (!isset($lists[$listKey])) {
        echo "Unknown list: $listKey, skipping...\n";
        continue;
    }
    
    $list = $lists[$listKey];
    echo "Fetching {$list['name']}...\n";
    
    $allMovies = [];
    $seenIds = [];
    
    for ($page = 1; $page <= $maxPages; $page++) {
        // Build URL based on list type - FIXED: Added include_adult=false and with_original_language=en
        if ($list['type'] === 'discover') {
            $url = "https://api.themoviedb.org/3/{$list['endpoint']}?api_key={$apiKey}&language={$language}&region={$region}&include_adult=false&with_original_language=en&page={$page}";
            foreach ($list['params'] as $param => $value) {
                $url .= "&" . urlencode($param) . "=" . urlencode($value);
            }
        } else {
            $url = "https://api.themoviedb.org/3/{$list['endpoint']}?api_key={$apiKey}&language={$language}&region={$region}&include_adult=false&page={$page}";
        }
        
        $response = @file_get_contents($url);
        if ($response === false) {
            echo "  Error fetching page $page\n";
            break;
        }
        
        $data = json_decode($response, true);
        if (!isset($data['results']) || empty($data['results'])) {
            break;
        }
        
        foreach ($data['results'] as $movie) {
            // Skip duplicates
            if (isset($seenIds[$movie['id']])) {
                continue;
            }
            $seenIds[$movie['id']] = true;
            
            // Skip movies without essential data
            if (empty($movie['title']) || empty($movie['id'])) {
                continue;
            }
            
            // FIXED: Skip adult movies explicitly
            if (!empty($movie['adult']) && $movie['adult'] === true) {
                continue;
            }
            
            // FIXED: Skip non-English original language movies
            if (!isset($movie['original_language']) || $movie['original_language'] !== 'en') {
                continue;
            }
            
            // Format movie data
            $allMovies[] = [
                'id' => $movie['id'],
                'title' => $movie['title'],
                'original_title' => $movie['original_title'] ?? $movie['title'],
                'overview' => $movie['overview'] ?? '',
                'release_date' => $movie['release_date'] ?? '',
                'poster_path' => $movie['poster_path'] ?? '',
                'backdrop_path' => $movie['backdrop_path'] ?? '',
                'vote_average' => $movie['vote_average'] ?? 0,
                'vote_count' => $movie['vote_count'] ?? 0,
                'popularity' => $movie['popularity'] ?? 0,
                'genre_ids' => $movie['genre_ids'] ?? [],
                'adult' => $movie['adult'] ?? false,
                'original_language' => $movie['original_language'] ?? 'en'
            ];
        }
        
        $totalPages = $data['total_pages'] ?? 1;
        if ($page >= $totalPages) {
            break;
        }
        
        // Rate limiting
        usleep(100000); // 100ms delay
    }
    
    // Save to file
    $outputFile = $outputDir . '/' . $list['filename'];
    $output = [
        'list_type' => $listKey,
        'list_name' => $list['name'],
        'total_movies' => count($allMovies),
        'updated_at' => date('Y-m-d H:i:s'),
        'movies' => $allMovies
    ];
    
    file_put_contents($outputFile, json_encode($output, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE));
    
    echo "  Saved " . count($allMovies) . " movies to {$list['filename']}\n";
    $summary[$listKey] = count($allMovies);
}

// Create combined summary file
$summaryFile = $outputDir . '/summary.json';
$summaryData = [
    'updated_at' => date('Y-m-d H:i:s'),
    'lists' => $summary,
    'total_unique_movies' => array_sum($summary)
];
file_put_contents($summaryFile, json_encode($summaryData, JSON_PRETTY_PRINT));

echo "\n=== Summary ===\n";
foreach ($summary as $list => $count) {
    echo "$list: $count movies\n";
}
echo "Total: " . array_sum($summary) . " movies\n";
echo "\nDone!\n";
