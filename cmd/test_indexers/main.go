package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func testIndexer(indexer string, imdbID string) int {
	// Construct Comet URL with single indexer
	encodedIndexers := fmt.Sprintf(`["%s"]`, indexer)

	// Example Comet request
	url := fmt.Sprintf("https://comet.elfhosted.com/search?indexers=%s&imdbID=%s&cat=movies",
		strings.Replace(encodedIndexers, `"`, "%22", -1), imdbID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return 0
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func main() {
	indexers := []string{"bitorrent", "therarbg", "yts", "eztv", "thepiratebay", "kickasstorrent", "galaxy", "magnetdl"}
	imdbID := "tt1375666" // Inception

	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println("Testing Comet Indexers (Direct HTTP Test)")
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println()

	results := make(map[string]int)

	for _, indexer := range indexers {
		fmt.Printf("Testing: %s ... ", indexer)
		statusCode := testIndexer(indexer, imdbID)
		results[indexer] = statusCode
		if statusCode == 200 {
			fmt.Printf("✅ HTTP %d\n", statusCode)
		} else {
			fmt.Printf("❌ HTTP %d\n", statusCode)
		}
	}

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println("SUMMARY")
	fmt.Println("════════════════════════════════════════════════════════════════")
	for _, indexer := range indexers {
		fmt.Printf("%-20s: HTTP %d\n", indexer, results[indexer])
	}
}
