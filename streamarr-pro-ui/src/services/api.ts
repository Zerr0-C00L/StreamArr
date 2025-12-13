import axios from 'axios';
import type { Movie, Series, AddMovieRequest, SearchResult, Stream, Episode, Channel, EPGProgram, CalendarEntry } from '../types';

// Use relative URL so Vite proxy can intercept /api requests in development
const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add request interceptor for debugging
api.interceptors.request.use(
  (config) => {
    console.log('API Request:', config.method?.toUpperCase(), (config.baseURL || '') + (config.url || ''));
    return config;
  },
  (error) => {
    console.error('API Request Error:', error);
    return Promise.reject(error);
  }
);

// Add response interceptor for debugging
api.interceptors.response.use(
  (response) => {
    console.log('API Response:', response.config.url, response.status, response.data);
    return response;
  },
  (error) => {
    console.error('API Response Error:', error.config?.url, error.response?.status, error.message);
    return Promise.reject(error);
  }
);

export const streamarrApi = {
  // Health
  health: () => api.get('/health'),

  // Movies
  getMovies: (params?: { offset?: number; limit?: number; monitored?: boolean }) =>
    api.get<Movie[]>('/movies', { params }),
  
  getMovie: (id: number) =>
    api.get<Movie>(`/movies/${id}`),
  
  addMovie: (data: AddMovieRequest) =>
    api.post<Movie>('/movies', data),
  
  updateMovie: (id: number, data: Partial<Movie>) =>
    api.put<Movie>(`/movies/${id}`, data),
  
  deleteMovie: (id: number) =>
    api.delete(`/movies/${id}`),
  
  getMovieStreams: (id: number) =>
    api.get<Stream[]>(`/movies/${id}/streams`),
  
  getMoviePlayUrl: (id: number) =>
    api.get<{ stream_url: string; quality: string }>(`/movies/${id}/play`),

  // Series
  getSeries: (params?: { offset?: number; limit?: number; monitored?: boolean }) =>
    api.get<Series[]>('/series', { params }),
  
  getSingleSeries: (id: number) =>
    api.get<Series>(`/series/${id}`),
  
  addSeries: (data: { tmdb_id: number; monitored: boolean; quality_profile?: string }) =>
    api.post<Series>('/series', data),
  
  updateSeries: (id: number, data: Partial<Series>) =>
    api.put<Series>(`/series/${id}`, data),
  
  deleteSeries: (id: number) =>
    api.delete(`/series/${id}`),

  // Episodes
  getEpisodes: (seriesId: number, season?: number) =>
    api.get<Episode[]>(`/series/${seriesId}/episodes`, { params: { season } }),
  
  getEpisode: (id: number) =>
    api.get<Episode>(`/episodes/${id}`),
  
  updateEpisode: (id: number, data: Partial<Episode>) =>
    api.put<Episode>(`/episodes/${id}`, data),
  
  getEpisodePlayUrl: (id: number) =>
    api.get<{ stream_url: string; quality: string }>(`/episodes/${id}/play`),

  // Live TV / IPTV
  getChannels: (params?: { category?: string; country?: string }) =>
    api.get<Channel[]>('/channels', { params }),
  
  getChannel: (id: number) =>
    api.get<Channel>(`/channels/${id}`),
  
  getChannelEPG: (channelId: number, date?: string) =>
    api.get<EPGProgram[]>(`/channels/${channelId}/epg`, { params: { date } }),
  
  getChannelStream: (id: number) =>
    api.get<{ stream_url: string }>(`/channels/${id}/stream`),

  // Search
  searchMovies: (query: string) =>
    api.get<SearchResult[]>('/search/movies', { params: { q: query } }),
  
  searchSeries: (query: string) =>
    api.get<SearchResult[]>('/search/series', { params: { q: query } }),

  // Calendar
  getCalendar: (start: string, end: string) =>
    api.get<CalendarEntry[]>('/calendar', { params: { start, end } }),

  // Discover / Trending
  getTrending: (mediaType: 'all' | 'movie' | 'tv' = 'all', timeWindow: 'day' | 'week' = 'day') =>
    api.get<TrendingItem[]>('/discover/trending', { params: { media_type: mediaType, time_window: timeWindow } }),
  
  getPopular: (mediaType: 'movie' | 'tv' = 'movie') =>
    api.get<TrendingItem[]>('/discover/popular', { params: { media_type: mediaType } }),
  
  getNowPlaying: (mediaType: 'movie' | 'tv' = 'movie') =>
    api.get<TrendingItem[]>('/discover/now-playing', { params: { media_type: mediaType } }),

  // Statistics
  getStats: () =>
    api.get('/stats'),
};

// Trending item type from TMDB
export interface TrendingItem {
  id: number;
  title: string;
  media_type: string;
  poster_path: string;
  backdrop_path: string;
  overview: string;
  release_date: string;
  vote_average: number;
}

export const tmdbImageUrl = (path: string, size: 'w200' | 'w500' | 'w780' | 'original' = 'w500') => {
  if (!path) return 'https://via.placeholder.com/500x750/1e293b/64748b?text=No+Poster';
  return `https://image.tmdb.org/t/p/${size}${path}`;
};
