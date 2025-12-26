import axios from 'axios';
import type { Movie, Series, AddMovieRequest, SearchResult, Stream, Episode, Channel, EPGProgram, CalendarEntry, TVGuideResponse, Video } from '../types';

// Use relative URL so Vite proxy can intercept /api requests in development
const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add auth token to all requests
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('auth_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    console.log('API Request:', config.method?.toUpperCase(), (config.baseURL || '') + (config.url || ''));
    return config;
  },
  (error) => {
    console.error('API Request Error:', error);
    return Promise.reject(error);
  }
);

// Handle 401 responses (redirect to login)
api.interceptors.response.use(
  (response) => {
    console.log('API Response:', response.config.url, response.status, response.data);
    return response;
  },
  (error) => {
    console.error('API Response Error:', error.config?.url, error.response?.status, error.message);
    if (error.response?.status === 401) {
      localStorage.removeItem('auth_token');
      localStorage.removeItem('username');
      localStorage.removeItem('is_admin');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export const streamarrApi = {
  // Health
  health: () => api.get('/health'),

  // Settings
  getSettings: () => api.get('/settings'),
  
  updateSettings: (data: any) =>
    api.put('/settings', data),

  // Movies
  getMovies: (params?: { offset?: number; limit?: number; monitored?: boolean; sort?: string; order?: string }) =>
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
  
  getMovieVideos: (id: number) =>
    api.get<Video[]>(`/movies/${id}/videos`),

  // Series
  getSeries: (params?: { offset?: number; limit?: number; monitored?: boolean; sort?: string; order?: string }) =>
    api.get<Series[]>('/series', { params }),
  
  getSingleSeries: (id: number) =>
    api.get<Series>(`/series/${id}`),
  
  addSeries: (data: { tmdb_id: number; monitored: boolean; quality_profile?: string }) =>
    api.post<Series>('/series', data),
  
  updateSeries: (id: number, data: Partial<Series>) =>
    api.put<Series>(`/series/${id}`, data),
  
  deleteSeries: (id: number) =>
    api.delete(`/series/${id}`),
  
  getSeriesStreams: (tmdb_id: number) =>
    api.get<Stream[]>(`/stream/series/${tmdb_id}`),
  
  getSeriesVideos: (id: number) =>
    api.get<Video[]>(`/series/${id}/videos`),

  // Episodes
  getEpisodes: (seriesId: number, season?: number) =>
    api.get<Episode[]>(`/series/${seriesId}/episodes`, { params: { season } }),
  
  getEpisode: (id: number) =>
    api.get<Episode>(`/episodes/${id}`),
  
  updateEpisode: (id: number, data: Partial<Episode>) =>
    api.put<Episode>(`/episodes/${id}`, data),
  
  getEpisodeStreams: (seriesIMDBId: string, season: number, episode: number) =>
    api.get<Stream[]>(`/stream/series/${seriesIMDBId}:${season}:${episode}`),
  
  getEpisodePlayUrl: (id: number) =>
    api.get<{ stream_url: string; quality: string }>(`/episodes/${id}/play`),

  // Live TV / IPTV
  getChannels: (params?: { category?: string; country?: string }) =>
    api.get<Channel[]>('/channels', { params }),
  
  getChannelCategories: () =>
    api.get<{ categories: string[] }>('/channels/categories'),
  
  getChannel: (id: number) =>
    api.get<Channel>(`/channels/${id}`),
  
  getChannelEPG: (channelId: number, date?: string) =>
    api.get<EPGProgram[]>(`/channels/${channelId}/epg`, { params: { date } }),
  
  getChannelStream: (id: number) =>
    api.get<{ stream_url: string }>(`/channels/${id}/stream`),

  getTVGuide: (params?: { category?: string; limit?: number; hours?: number }) =>
    api.get<TVGuideResponse>('/channels/epg/guide', { params }),

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

  // Collections
  getCollections: (params?: { offset?: number; limit?: number }) =>
    api.get('/collections', { params }),
  
  getCollection: (id: number) =>
    api.get(`/collections/${id}`),
  
  syncCollection: (id: number) =>
    api.post(`/collections/${id}/sync`),

  // Blacklist
  removeAndBlacklist: (type: 'movie' | 'series', id: number, reason?: string) =>
    api.post(`/${type}/${id}/remove-and-blacklist`, { reason }),
  
  getBlacklist: (params?: { limit?: number; offset?: number }) =>
    api.get('/blacklist', { params }),
  
  removeFromBlacklist: (id: number) =>
    api.delete(`/blacklist/${id}`),
  
  clearBlacklist: () =>
    api.post('/blacklist/clear'),
};

// Trending item type from TMDB
export interface TrendingItem {
  id: number;
  title: string;
  name?: string;
  media_type: string;
  poster_path: string;
  backdrop_path?: string;
  overview: string;
  release_date: string;
  first_air_date?: string;
  vote_average: number;
  vote_count?: number;
  popularity?: number;
}

export const tmdbImageUrl = (path: string, size: 'w200' | 'w300' | 'w342' | 'w500' | 'w780' | 'w1280' | 'original' = 'w500') => {
  if (!path) return 'https://via.placeholder.com/500x750/1e293b/64748b?text=No+Poster';
  return `https://image.tmdb.org/t/p/${size}${path}`;
};
