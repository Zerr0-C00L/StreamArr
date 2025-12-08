<?php
/**
 * Fetch ALL TMDB collections and their movies
 * Uses TMDB search API to find collections, then fetches all movies from each
 * Saves to collections_playlist.json
 */

set_time_limit(0);
ini_set('memory_limit', '512M');
error_reporting(E_ALL);

$apiKey = getenv('SECRET_API_KEY');
$playVodUrl = "[[SERVER_URL]]/play.php";
$language = 'en-US';
$region = 'US';

// Quality filters - adjust these to control what gets included
$minVoteCount = 50;           // Minimum number of votes (filters out obscure movies)
$minVoteAverage = 5.0;        // Minimum rating (filters out poorly rated movies)
$minCollectionSize = 2;       // Minimum movies in a collection to include it
$minYear = 1970;              // Skip very old movies
$minPopularity = 5.0;         // Minimum popularity score (filters out unknown movies)

echo "Starting TMDB Collections Fetch...\n";
echo "Quality Filters: minVotes=$minVoteCount, minRating=$minVoteAverage, minYear=$minYear, minPopularity=$minPopularity\n";

// Check if API key is set
if (empty($apiKey)) {
    echo "ERROR: SECRET_API_KEY environment variable is not set!\n";
    echo "Please add TMDB API key to repository secrets.\n";
    exit(1);
}

echo "API Key found (length: " . strlen($apiKey) . ")\n";
echo "Strategy: Search for collections A-Z, then fetch all movies from each\n\n";

// Test API connection first
$testUrl = "https://api.themoviedb.org/3/configuration?api_key=$apiKey";
$testResponse = @file_get_contents($testUrl);
if ($testResponse === false) {
    echo "ERROR: Failed to connect to TMDB API. Check your API key.\n";
    exit(1);
}
echo "TMDB API connection successful!\n\n";

$outputData = [];
$addedMovieIds = [];
$processedCollections = [];

// Search for collections using alphabet + numbers + common words
$searchTerms = array_merge(
    range('a', 'z'),
    range('0', '9'),
    ['the ', 'star ', 'super ', 'dark ', 'night ', 'dead ', 'final ', 'last ', 'man ', 'american ', 'mission ']
);

$collectionIds = [];

foreach ($searchTerms as $term) {
    echo "Searching collections: '$term'... ";
    $foundThisTerm = 0;
    
    for ($page = 1; $page <= 100; $page++) {
        // FIXED: Added include_adult=false to exclude adult collections
        $url = "https://api.themoviedb.org/3/search/collection?api_key=$apiKey&language=$language&region=$region&include_adult=false&query=" . urlencode($term) . "&page=$page";
        
        $context = stream_context_create(['http' => ['timeout' => 10]]);
        $response = @file_get_contents($url, false, $context);
        
        if ($response === false) {
            usleep(100000);
            continue;
        }
        
        $data = json_decode($response, true);
        if (!$data || !isset($data['results']) || empty($data['results'])) break;
        
        foreach ($data['results'] as $collection) {
            if (isset($collection['id']) && !isset($collectionIds[$collection['id']])) {
                $collectionIds[$collection['id']] = $collection['name'] ?? 'Unknown';
                $foundThisTerm++;
            }
        }
        
        if ($page >= ($data['total_pages'] ?? 1)) break;
        usleep(40000);
    }
    
    echo "found $foundThisTerm new\n";
}

echo "\n==> Found " . count($collectionIds) . " unique collections\n";
echo "Now fetching all movies from each collection...\n\n";

// Fetch all movies from each collection
$collectionCount = 0;
$totalCollections = count($collectionIds);

foreach ($collectionIds as $collectionId => $collectionName) {
    $collectionCount++;
    
    // FIXED: Added include_adult=false to exclude adult movies from collections
    $url = "https://api.themoviedb.org/3/collection/$collectionId?api_key=$apiKey&language=$language&region=$region&include_adult=false";
    $context = stream_context_create(['http' => ['timeout' => 10]]);
    $response = @file_get_contents($url, false, $context);
    
    if ($response === false) {
        usleep(100000);
        continue;
    }
    
    $collection = json_decode($response, true);
    if (!$collection || !isset($collection['parts'])) continue;
    
    $collectionName = $collection['name'] ?? 'Unknown Collection';
    $movieCount = 0;
    
    // QUALITY CHECK: Skip collections with too few movies
    $collectionMovies = $collection['parts'] ?? [];
    if (count($collectionMovies) < $minCollectionSize) {
        continue;
    }
    
    // QUALITY CHECK: Skip non-English language collections
    $englishMovieCount = 0;
    $qualityMovieCount = 0;
    foreach ($collectionMovies as $m) {
        if (isset($m['original_language']) && $m['original_language'] === 'en') {
            $englishMovieCount++;
            // Count movies that also meet quality thresholds
            $votes = $m['vote_count'] ?? 0;
            $rating = $m['vote_average'] ?? 0;
            $popularity = $m['popularity'] ?? 0;
            if ($votes >= $minVoteCount && $rating >= $minVoteAverage && $popularity >= $minPopularity) {
                $qualityMovieCount++;
            }
        }
    }
    
    // Skip collection if less than half of movies are English
    if (count($collectionMovies) > 0 && $englishMovieCount < count($collectionMovies) / 2) {
        continue;
    }
    
    // QUALITY CHECK: Need at least 2 quality movies to include this collection
    // If collection qualifies, we'll include ALL movies from it (not just quality ones)
    if ($qualityMovieCount < $minCollectionSize) {
        continue;
    }
    
    // Collection passed quality check - now add ALL English movies from it
    foreach ($collection['parts'] as $movie) {
        if (!isset($movie['id']) || isset($addedMovieIds[$movie['id']])) continue;
        
        // FIXED: Skip adult movies explicitly
        if (!empty($movie['adult']) && $movie['adult'] === true) continue;
        
        // FIXED: Skip non-English original language movies
        if (!isset($movie['original_language']) || $movie['original_language'] !== 'en') continue;
        
        // Skip unreleased movies
        if (!empty($movie['release_date'])) {
            if (strtotime($movie['release_date']) > time()) continue;
            $releaseYear = (int)substr($movie['release_date'], 0, 4);
            if ($releaseYear > (int)date('Y')) continue;
            // Keep minYear filter - we still don't want very old movies
            if ($releaseYear < $minYear) continue;
        } else {
            continue;
        }
        
        // Skip movies without poster
        if (empty($movie['poster_path'])) continue;
        
        // NO quality filters here - collection already qualified, include all movies
        $voteCount = $movie['vote_count'] ?? 0;
        $voteAverage = $movie['vote_average'] ?? 0;
        $popularity = $movie['popularity'] ?? 0;
        
        $addedMovieIds[$movie['id']] = true;
        $movieCount++;
        
        $outputData[] = [
            'num' => count($outputData) + 1,
            'name' => $movie['title'] ?? 'Unknown',
            'stream_type' => 'movie',
            'stream_id' => $movie['id'],
            'stream_icon' => 'https://image.tmdb.org/t/p/w500' . $movie['poster_path'],
            'rating' => $voteAverage,
            'vote_count' => $voteCount,
            'popularity' => $popularity,
            'added' => time(),
            'category_id' => (string)$collectionId,
            'category_name' => $collectionName,
            'collection_id' => $collectionId,
            'collection_name' => $collectionName,
            'year' => substr($movie['release_date'] ?? '', 0, 4),
            'container_extension' => 'mp4',
            'custom_sid' => '',
            'direct_source' => $playVodUrl . '?id=' . $movie['id'] . '&type=movie'
        ];
    }
    
    if ($movieCount > 0) {
        $processedCollections[$collectionId] = $collectionName;
    }
    
    // Progress every 100 collections
    if ($collectionCount % 100 == 0) {
        echo "Progress: $collectionCount/$totalCollections collections, " . count($outputData) . " movies\n";
    }
    
    usleep(40000); // Rate limiting
}

echo "\n========================================\n";
echo "Processed: $collectionCount collections\n";
echo "Found: " . count($outputData) . " movies in " . count($processedCollections) . " collections\n";
echo "========================================\n\n";

// Sort by collection name, then by year
usort($outputData, function($a, $b) {
    $coll = strcmp($a['collection_name'], $b['collection_name']);
    if ($coll !== 0) return $coll;
    return strcmp($a['year'] ?? '', $b['year'] ?? '');
});

// Re-number after sorting
foreach ($outputData as $i => &$item) {
    $item['num'] = $i + 1;
}

// Save to JSON
file_put_contents('collections_playlist.json', json_encode($outputData));
$size = round(filesize('collections_playlist.json') / 1024, 2);
echo "Saved: collections_playlist.json ($size KB, " . count($outputData) . " movies)\n";

// Also save a list of collections found
$collectionsList = [];
foreach ($processedCollections as $id => $name) {
    $collectionsList[] = ['id' => $id, 'name' => $name];
}
usort($collectionsList, fn($a, $b) => strcmp($a['name'], $b['name']));
file_put_contents('collections_list.json', json_encode($collectionsList, JSON_PRETTY_PRINT));
echo "Saved: collections_list.json (" . count($collectionsList) . " collections)\n";

echo "\nDone!\n";
