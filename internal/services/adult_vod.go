package services

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/Zerr0-C00L/StreamArr/internal/database"
    "github.com/Zerr0-C00L/StreamArr/internal/models"
)

// ImportAdultMoviesFromGitHub imports adult movies from the public-files repository
func ImportAdultMoviesFromGitHub(ctx context.Context, movieStore *database.MovieStore) (imported, skipped, errors int, err error) {
    url := "https://raw.githubusercontent.com/Zerr0-C00L/public-files/main/adult-movies.json"
    resp, e := http.Get(url)
    if e != nil {
        return 0, 0, 0, fmt.Errorf("fetch adult movies: %w", e)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return 0, 0, 0, fmt.Errorf("github returned %d", resp.StatusCode)
    }

    var adultMovies []map[string]interface{}
    if e := json.NewDecoder(resp.Body).Decode(&adultMovies); e != nil {
        return 0, 0, 0, fmt.Errorf("parse adult movies: %w", e)
    }
    log.Printf("[Adult VOD Import] Loaded %d entries from GitHub", len(adultMovies))

    for _, movieData := range adultMovies {
        tmdbID, _ := movieData["num"].(float64)
        if tmdbID == 0 { continue }
        title, _ := movieData["name"].(string)
        posterPath, _ := movieData["stream_icon"].(string)
        plot, _ := movieData["plot"].(string)
        genresStr, _ := movieData["genres"].(string)
        director, _ := movieData["director"].(string)
        cast, _ := movieData["cast"].(string)
        rating, _ := movieData["rating"].(float64)
        addedTimestamp, _ := movieData["added"].(float64)

        genres := []string{}
        if genresStr != "" {
            parts := strings.Split(genresStr, ",")
            for _, g := range parts { genres = append(genres, strings.TrimSpace(g)) }
        }

        m := &models.Movie{
            TMDBID:         int(tmdbID),
            Title:          title,
            OriginalTitle:  title,
            Overview:       plot,
            PosterPath:     posterPath,
            Genres:         genres,
            Monitored:      true,
            Available:      true,
            QualityProfile: "1080p",
            Metadata: models.Metadata{
                "stream_type": "adult",
                "category_id": "999993",
                "director":    director,
                "cast":        cast,
                "rating":      rating,
                "source":      "github_public_files",
                "imported_at": time.Now().Format(time.RFC3339),
            },
        }
        if addedTimestamp > 0 {
            m.AddedAt = time.Unix(int64(addedTimestamp), 0)
        }

        if e := movieStore.Add(ctx, m); e != nil {
            if strings.Contains(e.Error(), "already exists") {
                skipped++
            } else {
                errors++
                log.Printf("[Adult VOD Import] add '%s' error: %v", title, e)
            }
        } else {
            imported++
        }
    }
    return imported, skipped, errors, nil
}
