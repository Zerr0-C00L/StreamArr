export interface Collection {
  id: number;
  tmdb_id: number;
  name: string;
  overview: string;
  poster_path: string;
  backdrop_path: string;
  created_at: string;
  updated_at: string;
  total_movies?: number;
  movies_in_library?: number;
}

export interface Movie {
  id: number;
  tmdb_id: number;
  title: string;
  original_title: string;
  overview: string;
  poster_path: string;
  backdrop_path: string;
  release_date: string;
  runtime: number;
  genres: string[];
  metadata: Record<string, any>;
  monitored: boolean;
  available: boolean;
  quality_profile: string;
  search_status: string;
  last_checked?: string;
  created_at: string;
  updated_at: string;
  added_at: string;
  collection_id?: number;
  collection?: Collection;
}

export interface Series {
  id: number;
  tmdb_id: number;
  title: string;
  original_title: string;
  overview: string;
  poster_path: string;
  backdrop_path: string;
  first_air_date: string;
  status: string;
  seasons: number;
  total_episodes: number;
  genres: string[];
  metadata: Record<string, any>;
  monitored: boolean;
  quality_profile: string;
  search_status: string;
  last_checked?: string;
  created_at: string;
  updated_at: string;
  added_at: string;
}

export interface Stream {
  id: number;
  movie_id?: number;
  episode_id?: number;
  source: string;
  quality: string;
  codec: string;
  url: string;
  cached: boolean;
  seeds: number;
  size_gb: number;
  created_at: string;
  expires_at: string;
}

export interface AddMovieRequest {
  tmdb_id: number;
  monitored: boolean;
  quality_profile?: string;
}

export interface SearchResult {
  id: number;
  media_type: 'movie' | 'tv';
  title: string;
  release_date: string;
  poster_path: string;
  overview: string;
  vote_average: number;
}

export interface Episode {
  id: number;
  series_id: number;
  season_number: number;
  episode_number: number;
  title: string;
  overview: string;
  air_date: string;
  runtime: number;
  still_path: string;
  monitored: boolean;
  available: boolean;
  created_at: string;
  updated_at: string;
}

export interface Channel {
  id: number;
  name: string;
  logo: string;
  stream_url: string;
  category: string;
  language: string;
  country: string;
  epg_id?: string;
  active: boolean;
  created_at: string;
}

export interface EPGProgram {
  id: number;
  channel_id: number;
  title: string;
  description: string;
  start_time: string;
  end_time: string;
  category: string;
}

export interface DashboardStats {
  total_movies: number;
  total_series: number;
  total_episodes: number;
  total_channels: number;
  monitored_movies: number;
  monitored_series: number;
  available_movies: number;
  available_episodes: number;
  disk_space: {
    total: string;
    used: string;
    free: string;
  };
}

export interface CalendarEntry {
  id: number;
  type: 'movie' | 'episode';
  title: string;
  date: string;
  poster_path: string;
  overview: string;
  vote_average?: number;
  year?: number;
  series_id?: number;
  series_title?: string;
  season_number?: number;
  episode_number?: number;
}
