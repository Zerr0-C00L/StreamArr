package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Zerr0-C00L/StreamArr/internal/models"
)

const (
	tmdbBaseURL      = "https://api.themoviedb.org/3"
	tmdbImageBaseURL = "https://image.tmdb.org/t/p"
)

type TMDBClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewTMDBClient(apiKey string) *TMDBClient {
	return &TMDBClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Movie API responses
type tmdbMovie struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	OriginalLanguage string `json:"original_language"`
	Overview      string `json:"overview"`
	PosterPath    string `json:"poster_path"`
	BackdropPath  string `json:"backdrop_path"`
	ReleaseDate   string `json:"release_date"`
	Runtime       int    `json:"runtime"`
	Genres        []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCountries []struct {
		ISO3166_1 string `json:"iso_3166_1"`
		Name      string `json:"name"`
	} `json:"production_countries"`
	VoteAverage         float64                  `json:"vote_average"`
	VoteCount           int                      `json:"vote_count"`
	Status              string                   `json:"status"`
	IMDbID              string                   `json:"imdb_id"`
	BelongsToCollection *tmdbBelongsToCollection `json:"belongs_to_collection"`
}

// tmdbBelongsToCollection represents the collection a movie belongs to
type tmdbBelongsToCollection struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

// tmdbCollection represents a full collection response
type tmdbCollection struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
	Parts        []struct {
		ID           int     `json:"id"`
		Title        string  `json:"title"`
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		BackdropPath string  `json:"backdrop_path"`
		ReleaseDate  string  `json:"release_date"`
		VoteAverage  float64 `json:"vote_average"`
	} `json:"parts"`
}

type tmdbSeries struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	OriginalName    string `json:"original_name"`
	OriginalLanguage string `json:"original_language"`
	Overview        string `json:"overview"`
	PosterPath      string `json:"poster_path"`
	BackdropPath    string `json:"backdrop_path"`
	FirstAirDate    string `json:"first_air_date"`
	Status          string `json:"status"`
	NumberOfSeasons int    `json:"number_of_seasons"`
	OriginCountry   []string `json:"origin_country"`
	Genres          []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
}

type tmdbSeason struct {
	ID           int           `json:"id"`
	SeasonNumber int           `json:"season_number"`
	Name         string        `json:"name"`
	Overview     string        `json:"overview"`
	PosterPath   string        `json:"poster_path"`
	AirDate      string        `json:"air_date"`
	EpisodeCount int           `json:"episode_count"`
	Episodes     []tmdbEpisode `json:"episodes"`
}

type tmdbEpisode struct {
	ID            int     `json:"id"`
	SeasonNumber  int     `json:"season_number"`
	EpisodeNumber int     `json:"episode_number"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date"`
	StillPath     string  `json:"still_path"`
	Runtime       int     `json:"runtime"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
}

type tmdbSearchResult struct {
	Page         int           `json:"page"`
	Results      []interface{} `json:"results"`
	TotalResults int           `json:"total_results"`
	TotalPages   int           `json:"total_pages"`
}

// GetMovie retrieves movie details from TMDB
func (c *TMDBClient) GetMovie(ctx context.Context, tmdbID int) (*models.Movie, error) {
	endpoint := fmt.Sprintf("%s/movie/%d", tmdbBaseURL, tmdbID)
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var tmdbMovie tmdbMovie
	if err := json.Unmarshal(data, &tmdbMovie); err != nil {
		return nil, fmt.Errorf("failed to unmarshal movie: %w", err)
	}

	return c.convertMovie(&tmdbMovie), nil
}

// GetMovieWithCollection retrieves movie details along with collection info
func (c *TMDBClient) GetMovieWithCollection(ctx context.Context, tmdbID int) (*models.Movie, *models.Collection, error) {
	endpoint := fmt.Sprintf("%s/movie/%d", tmdbBaseURL, tmdbID)
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, nil, err
	}

	var tmdbMovie tmdbMovie
	if err := json.Unmarshal(data, &tmdbMovie); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal movie: %w", err)
	}

	movie := c.convertMovie(&tmdbMovie)

	var collection *models.Collection
	if tmdbMovie.BelongsToCollection != nil {
		collection = &models.Collection{
			TMDBID:       tmdbMovie.BelongsToCollection.ID,
			Name:         tmdbMovie.BelongsToCollection.Name,
			PosterPath:   tmdbMovie.BelongsToCollection.PosterPath,
			BackdropPath: tmdbMovie.BelongsToCollection.BackdropPath,
		}
	}

	return movie, collection, nil
}

// CollectionMovie represents a movie in a collection with library status
type CollectionMovie struct {
	TMDBID       int     `json:"tmdb_id"`
	Title        string  `json:"title"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	ReleaseDate  string  `json:"release_date"`
	VoteAverage  float64 `json:"vote_average"`
	InLibrary    bool    `json:"in_library"`
}

// GetCollection retrieves full collection details from TMDB
func (c *TMDBClient) GetCollection(ctx context.Context, collectionID int) (*models.Collection, []int, error) {
	endpoint := fmt.Sprintf("%s/collection/%d", tmdbBaseURL, collectionID)
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, nil, err
	}

	var tc tmdbCollection
	if err := json.Unmarshal(data, &tc); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal collection: %w", err)
	}

	collection := &models.Collection{
		TMDBID:       tc.ID,
		Name:         tc.Name,
		Overview:     tc.Overview,
		PosterPath:   tc.PosterPath,
		BackdropPath: tc.BackdropPath,
		TotalMovies:  len(tc.Parts),
	}

	// Extract movie TMDB IDs
	movieIDs := make([]int, 0, len(tc.Parts))
	for _, part := range tc.Parts {
		movieIDs = append(movieIDs, part.ID)
	}

	return collection, movieIDs, nil
}

// GetCollectionWithMovies retrieves full collection details including all movie info from TMDB
func (c *TMDBClient) GetCollectionWithMovies(ctx context.Context, collectionID int) (*models.Collection, []CollectionMovie, error) {
	endpoint := fmt.Sprintf("%s/collection/%d", tmdbBaseURL, collectionID)
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, nil, err
	}

	var tc tmdbCollection
	if err := json.Unmarshal(data, &tc); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal collection: %w", err)
	}

	collection := &models.Collection{
		TMDBID:       tc.ID,
		Name:         tc.Name,
		Overview:     tc.Overview,
		PosterPath:   tc.PosterPath,
		BackdropPath: tc.BackdropPath,
		TotalMovies:  len(tc.Parts),
	}

	// Convert parts to CollectionMovie slice
	movies := make([]CollectionMovie, 0, len(tc.Parts))
	for _, part := range tc.Parts {
		movies = append(movies, CollectionMovie{
			TMDBID:       part.ID,
			Title:        part.Title,
			Overview:     part.Overview,
			PosterPath:   part.PosterPath,
			BackdropPath: part.BackdropPath,
			ReleaseDate:  part.ReleaseDate,
			VoteAverage:  part.VoteAverage,
			InLibrary:    false, // Will be set by the handler
		})
	}

	return collection, movies, nil
}

// GetSeries retrieves series details from TMDB, including IMDB ID
func (c *TMDBClient) GetSeries(ctx context.Context, tmdbID int) (*models.Series, error) {
	endpoint := fmt.Sprintf("%s/tv/%d", tmdbBaseURL, tmdbID)
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var tmdbSeries tmdbSeries
	if err := json.Unmarshal(data, &tmdbSeries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal series: %w", err)
	}

	series := c.convertSeries(&tmdbSeries)

	// Fetch external IDs to get IMDB ID
	externalIDs, err := c.GetSeriesExternalIDs(ctx, tmdbID)
	if err == nil && externalIDs.IMDBID != "" {
		series.Metadata["imdb_id"] = externalIDs.IMDBID
	}

	return series, nil
}

// GetSeason retrieves season details including episodes
func (c *TMDBClient) GetSeason(ctx context.Context, seriesID, seasonNumber int) (*tmdbSeason, error) {
	endpoint := fmt.Sprintf("%s/tv/%d/season/%d", tmdbBaseURL, seriesID, seasonNumber)
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var season tmdbSeason
	if err := json.Unmarshal(data, &season); err != nil {
		return nil, fmt.Errorf("failed to unmarshal season: %w", err)
	}

	return &season, nil
}

// ExternalIDs represents external IDs for a TV series
type ExternalIDs struct {
	ID         int    `json:"id"`
	IMDBID     string `json:"imdb_id"`
	FreebaseID string `json:"freebase_id"`
	TVDBID     int    `json:"tvdb_id"`
}

// Video represents a TMDB video (trailer, teaser, etc.)
type Video struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	Site      string `json:"site"`
	Size      int    `json:"size"`
	Type      string `json:"type"`
	Official  bool   `json:"official"`
	Published string `json:"published_at"`
}

// GetVideos retrieves videos (trailers, teasers, etc.) for a movie or TV show
func (c *TMDBClient) GetVideos(ctx context.Context, mediaType string, tmdbID int) ([]Video, error) {
	var endpoint string
	if mediaType == "movie" {
		endpoint = fmt.Sprintf("%s/movie/%d/videos", tmdbBaseURL, tmdbID)
	} else {
		endpoint = fmt.Sprintf("%s/tv/%d/videos", tmdbBaseURL, tmdbID)
	}
	
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("language", "en-US")

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []Video `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal videos: %w", err)
	}

	return result.Results, nil
}

// GetSeriesExternalIDs retrieves external IDs (IMDB, TVDB, etc.) for a series
func (c *TMDBClient) GetSeriesExternalIDs(ctx context.Context, seriesID int) (*ExternalIDs, error) {
	endpoint := fmt.Sprintf("%s/tv/%d/external_ids", tmdbBaseURL, seriesID)
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var ids ExternalIDs
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("failed to unmarshal external IDs: %w", err)
	}

	return &ids, nil
}

// GetEpisodes retrieves all episodes for a series
func (c *TMDBClient) GetEpisodes(ctx context.Context, seriesID int64, tmdbID int, seasons int) ([]*models.Episode, error) {
	var allEpisodes []*models.Episode

	for seasonNum := 1; seasonNum <= seasons; seasonNum++ {
		season, err := c.GetSeason(ctx, tmdbID, seasonNum)
		if err != nil {
			return nil, fmt.Errorf("failed to get season %d: %w", seasonNum, err)
		}

		for _, ep := range season.Episodes {
			episode := c.convertEpisode(seriesID, &ep)
			allEpisodes = append(allEpisodes, episode)
		}
	}

	return allEpisodes, nil
}

// SearchMovies searches for movies
func (c *TMDBClient) SearchMovies(ctx context.Context, query string, page int) ([]*models.Movie, error) {
	endpoint := fmt.Sprintf("%s/search/movie", tmdbBaseURL)
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("query", query)
	params.Set("page", fmt.Sprintf("%d", page))

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []tmdbMovie `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search results: %w", err)
	}

	movies := make([]*models.Movie, 0, len(result.Results))
	for _, tmdbMovie := range result.Results {
		movies = append(movies, c.convertMovie(&tmdbMovie))
	}

	return movies, nil
}

// SearchSeries searches for TV series
func (c *TMDBClient) SearchSeries(ctx context.Context, query string, page int) ([]*models.Series, error) {
	endpoint := fmt.Sprintf("%s/search/tv", tmdbBaseURL)
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("query", query)
	params.Set("page", fmt.Sprintf("%d", page))

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []tmdbSeries `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search results: %w", err)
	}

	series := make([]*models.Series, 0, len(result.Results))
	for _, tmdbSeries := range result.Results {
		series = append(series, c.convertSeries(&tmdbSeries))
	}

	return series, nil
}

// SearchCollections searches for collections
func (c *TMDBClient) SearchCollections(ctx context.Context, query string) ([]*models.Collection, error) {
	endpoint := fmt.Sprintf("%s/search/collection", tmdbBaseURL)
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("query", query)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			Overview     string `json:"overview"`
			PosterPath   string `json:"poster_path"`
			BackdropPath string `json:"backdrop_path"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search results: %w", err)
	}

	collections := make([]*models.Collection, 0, len(result.Results))
	for _, c := range result.Results {
		collections = append(collections, &models.Collection{
			TMDBID:       c.ID,
			Name:         c.Name,
			Overview:     c.Overview,
			PosterPath:   c.PosterPath,
			BackdropPath: c.BackdropPath,
		})
	}

	return collections, nil
}

// DiscoverMovies discovers movies with filters
func (c *TMDBClient) DiscoverMovies(ctx context.Context, page int, year *int, genre *string) ([]*models.Movie, error) {
	endpoint := fmt.Sprintf("%s/discover/movie", tmdbBaseURL)
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("sort_by", "popularity.desc")

	if year != nil {
		params.Set("year", fmt.Sprintf("%d", *year))
	}
	if genre != nil {
		params.Set("with_genres", *genre)
	}

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []tmdbMovie `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal discover results: %w", err)
	}

	movies := make([]*models.Movie, 0, len(result.Results))
	for _, tmdbMovie := range result.Results {
		movies = append(movies, c.convertMovie(&tmdbMovie))
	}

	return movies, nil
}

// makeRequest performs an HTTP GET request to TMDB API
func (c *TMDBClient) makeRequest(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	// Add API key to params
	params.Set("api_key", c.apiKey)

	// Build full URL without double-prepending the TMDB base
	baseURL := endpoint
	if !strings.HasPrefix(endpoint, "http") {
		baseURL = fmt.Sprintf("%s%s", tmdbBaseURL, endpoint)
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TMDB endpoint %s: %w", baseURL, err)
	}

	q := u.Query()
	for k, vals := range params {
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", u.String(), err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API returned status %d for %s: %s", resp.StatusCode, u.String(), string(data))
	}

	return data, nil
}

// convertMovie converts TMDB movie to internal model
func (c *TMDBClient) convertMovie(tm *tmdbMovie) *models.Movie {
	genres := make([]string, len(tm.Genres))
	for i, g := range tm.Genres {
		genres[i] = g.Name
	}

	var releaseDate *time.Time
	if tm.ReleaseDate != "" {
		if parsed, err := time.Parse("2006-01-02", tm.ReleaseDate); err == nil {
			releaseDate = &parsed
		}
	}

	return &models.Movie{
		TMDBID:        tm.ID,
		Title:         tm.Title,
		OriginalTitle: tm.OriginalTitle,
		Overview:      tm.Overview,
		PosterPath:    tm.PosterPath,
		BackdropPath:  tm.BackdropPath,
		ReleaseDate:   releaseDate,
		Runtime:       tm.Runtime,
		Genres:        genres,
		Metadata: models.Metadata{
			"vote_average": tm.VoteAverage,
			"vote_count":   tm.VoteCount,
			"status":       tm.Status,
			"imdb_id":      tm.IMDbID,
			"original_language": tm.OriginalLanguage,
			"production_countries": func() []string {
				codes := make([]string, 0, len(tm.ProductionCountries))
				for _, pc := range tm.ProductionCountries {
					if pc.ISO3166_1 != "" { codes = append(codes, pc.ISO3166_1) }
				}
				return codes
			}(),
		},
	}
}

// convertSeries converts TMDB series to internal model
func (c *TMDBClient) convertSeries(ts *tmdbSeries) *models.Series {
	genres := make([]string, len(ts.Genres))
	for i, g := range ts.Genres {
		genres[i] = g.Name
	}

	var firstAirDate *time.Time
	if ts.FirstAirDate != "" {
		if parsed, err := time.Parse("2006-01-02", ts.FirstAirDate); err == nil {
			firstAirDate = &parsed
		}
	}

	return &models.Series{
		TMDBID:        ts.ID,
		Title:         ts.Name,
		OriginalTitle: ts.OriginalName,
		Overview:      ts.Overview,
		PosterPath:    ts.PosterPath,
		BackdropPath:  ts.BackdropPath,
		FirstAirDate:  firstAirDate,
		Status:        ts.Status,
		Seasons:       ts.NumberOfSeasons,
		Genres:        genres,
		Metadata: models.Metadata{
			"vote_average": ts.VoteAverage,
			"vote_count":   ts.VoteCount,
			"original_language": ts.OriginalLanguage,
			"origin_country":    ts.OriginCountry,
		},
	}
}

// convertEpisode converts TMDB episode to internal model
func (c *TMDBClient) convertEpisode(seriesID int64, te *tmdbEpisode) *models.Episode {
	var airDate *time.Time
	if te.AirDate != "" {
		if parsed, err := time.Parse("2006-01-02", te.AirDate); err == nil {
			airDate = &parsed
		}
	}

	return &models.Episode{
		SeriesID:      seriesID,
		TMDBID:        te.ID,
		SeasonNumber:  te.SeasonNumber,
		EpisodeNumber: te.EpisodeNumber,
		Title:         te.Name,
		Overview:      te.Overview,
		AirDate:       airDate,
		StillPath:     te.StillPath,
		Runtime:       te.Runtime,
		Metadata: models.Metadata{
			"vote_average": te.VoteAverage,
			"vote_count":   te.VoteCount,
		},
	}
}

// GetPosterURL returns the full poster URL
func (c *TMDBClient) GetPosterURL(path string, size string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s%s", tmdbImageBaseURL, size, path)
}

// IMDBToTMDB converts an IMDB ID to TMDB ID
func (c *TMDBClient) IMDBToTMDB(imdbID string, mediaType string) (int, error) {
	ctx := context.Background()

	endpoint := fmt.Sprintf("/find/%s", imdbID)
	params := url.Values{}
	params.Set("external_source", "imdb_id")

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return 0, err
	}

	var result struct {
		MovieResults []struct {
			ID int `json:"id"`
		} `json:"movie_results"`
		TVResults []struct {
			ID int `json:"id"`
		} `json:"tv_results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if mediaType == "movie" && len(result.MovieResults) > 0 {
		return result.MovieResults[0].ID, nil
	}

	if mediaType == "tv" && len(result.TVResults) > 0 {
		return result.TVResults[0].ID, nil
	}

	return 0, fmt.Errorf("no TMDB ID found for IMDB ID %s", imdbID)
}

// TrendingItem represents a trending movie or TV show
type TrendingItem struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	MediaType    string  `json:"media_type"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	Overview     string  `json:"overview"`
	ReleaseDate  string  `json:"release_date"`
	VoteAverage  float64 `json:"vote_average"`
}

// GetTrending returns trending movies and TV shows
// timeWindow can be "day" or "week"
func (c *TMDBClient) GetTrending(ctx context.Context, mediaType string, timeWindow string) ([]TrendingItem, error) {
	endpoint := fmt.Sprintf("%s/trending/%s/%s", tmdbBaseURL, mediaType, timeWindow)
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			ID           int     `json:"id"`
			Title        string  `json:"title"`
			Name         string  `json:"name"`
			MediaType    string  `json:"media_type"`
			PosterPath   string  `json:"poster_path"`
			BackdropPath string  `json:"backdrop_path"`
			Overview     string  `json:"overview"`
			ReleaseDate  string  `json:"release_date"`
			FirstAirDate string  `json:"first_air_date"`
			VoteAverage  float64 `json:"vote_average"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trending: %w", err)
	}

	items := make([]TrendingItem, 0, len(result.Results))
	for _, r := range result.Results {
		title := r.Title
		if title == "" {
			title = r.Name
		}
		releaseDate := r.ReleaseDate
		if releaseDate == "" {
			releaseDate = r.FirstAirDate
		}
		mediaTypeResult := r.MediaType
		if mediaType != "all" {
			mediaTypeResult = mediaType
		}
		items = append(items, TrendingItem{
			ID:           r.ID,
			Title:        title,
			MediaType:    mediaTypeResult,
			PosterPath:   r.PosterPath,
			BackdropPath: r.BackdropPath,
			Overview:     r.Overview,
			ReleaseDate:  releaseDate,
			VoteAverage:  r.VoteAverage,
		})
	}

	return items, nil
}

// GetPopular returns popular movies or TV shows
func (c *TMDBClient) GetPopular(ctx context.Context, mediaType string) ([]TrendingItem, error) {
	var endpoint string
	if mediaType == "movie" {
		endpoint = fmt.Sprintf("%s/movie/popular", tmdbBaseURL)
	} else {
		endpoint = fmt.Sprintf("%s/tv/popular", tmdbBaseURL)
	}

	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			ID           int     `json:"id"`
			Title        string  `json:"title"`
			Name         string  `json:"name"`
			PosterPath   string  `json:"poster_path"`
			BackdropPath string  `json:"backdrop_path"`
			Overview     string  `json:"overview"`
			ReleaseDate  string  `json:"release_date"`
			FirstAirDate string  `json:"first_air_date"`
			VoteAverage  float64 `json:"vote_average"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal popular: %w", err)
	}

	items := make([]TrendingItem, 0, len(result.Results))
	for _, r := range result.Results {
		title := r.Title
		if title == "" {
			title = r.Name
		}
		releaseDate := r.ReleaseDate
		if releaseDate == "" {
			releaseDate = r.FirstAirDate
		}
		items = append(items, TrendingItem{
			ID:           r.ID,
			Title:        title,
			MediaType:    mediaType,
			PosterPath:   r.PosterPath,
			BackdropPath: r.BackdropPath,
			Overview:     r.Overview,
			ReleaseDate:  releaseDate,
			VoteAverage:  r.VoteAverage,
		})
	}

	return items, nil
}

// GetNowPlaying returns movies currently in theaters or TV shows on air
func (c *TMDBClient) GetNowPlaying(ctx context.Context, mediaType string) ([]TrendingItem, error) {
	var endpoint string
	if mediaType == "movie" {
		endpoint = fmt.Sprintf("%s/movie/now_playing", tmdbBaseURL)
	} else {
		endpoint = fmt.Sprintf("%s/tv/on_the_air", tmdbBaseURL)
	}

	params := url.Values{}
	params.Set("api_key", c.apiKey)

	data, err := c.makeRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Results []struct {
			ID           int     `json:"id"`
			Title        string  `json:"title"`
			Name         string  `json:"name"`
			PosterPath   string  `json:"poster_path"`
			BackdropPath string  `json:"backdrop_path"`
			Overview     string  `json:"overview"`
			ReleaseDate  string  `json:"release_date"`
			FirstAirDate string  `json:"first_air_date"`
			VoteAverage  float64 `json:"vote_average"`
		} `json:"results"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal now playing: %w", err)
	}

	items := make([]TrendingItem, 0, len(result.Results))
	for _, r := range result.Results {
		title := r.Title
		if title == "" {
			title = r.Name
		}
		releaseDate := r.ReleaseDate
		if releaseDate == "" {
			releaseDate = r.FirstAirDate
		}
		items = append(items, TrendingItem{
			ID:           r.ID,
			Title:        title,
			MediaType:    mediaType,
			PosterPath:   r.PosterPath,
			BackdropPath: r.BackdropPath,
			Overview:     r.Overview,
			ReleaseDate:  releaseDate,
			VoteAverage:  r.VoteAverage,
		})
	}

	return items, nil
}
