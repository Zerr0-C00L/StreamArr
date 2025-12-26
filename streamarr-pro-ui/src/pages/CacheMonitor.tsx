import { useState, useEffect } from 'react';
import { Database, Clock, TrendingUp, XCircle, CheckCircle, RefreshCw, Film, Calendar } from 'lucide-react';
import axios from 'axios';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('auth_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

interface CacheStats {
  total_cached: number;
  available: number;
  unavailable: number;
  avg_quality_score: number;
}

interface CachedMovie {
  movie_id: number;
  title: string;
  year: number;
  quality_score: number;
  resolution: string;
  source_type: string;
  hdr_type: string;
  audio_format: string;
  file_size_gb: number;
  is_available: boolean;
  upgrade_available: boolean;
  last_checked: string;
  cached_at: string;
  indexer: string;
}

export default function CacheMonitor() {
  const [stats, setStats] = useState<CacheStats | null>(null);
  const [movies, setMovies] = useState<CachedMovie[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'all' | 'available' | 'unavailable' | 'upgrades'>('all');
  const [refreshing, setRefreshing] = useState(false);
  const [cleaningUp, setCleaningUp] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [deleting, setDeleting] = useState(false);

  const fetchData = async () => {
    try {
      setRefreshing(true);
      
      // Fetch cache stats
      const statsRes = await api.get('/streams/cache/stats');
      setStats(statsRes.data);

      // Fetch all cached movies (we'll need to create this endpoint)
      const moviesRes = await api.get('/streams/cache/list');
      setMovies(moviesRes.data);
      
      setLoading(false);
    } catch (error) {
      console.error('Failed to fetch cache data:', error);
      setLoading(false);
    } finally {
      setRefreshing(false);
    }
  };

  const cleanupUnreleased = async () => {
    if (!confirm('Remove cached streams for unreleased movies? This cannot be undone.')) {
      return;
    }

    try {
      setCleaningUp(true);
      const response = await api.post('/streams/cache/cleanup-unreleased');
      alert(`✅ ${response.data.message}`);
      fetchData(); // Refresh data
    } catch (error: any) {
      alert(`❌ Cleanup failed: ${error.response?.data?.error || error.message}`);
    } finally {
      setCleaningUp(false);
    }
  };

  const toggleSelection = (movieId: number) => {
    const newSelected = new Set(selectedIds);
    if (newSelected.has(movieId)) {
      newSelected.delete(movieId);
    } else {
      newSelected.add(movieId);
    }
    setSelectedIds(newSelected);
  };

  const toggleSelectAll = () => {
    if (selectedIds.size === filteredMovies.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(filteredMovies.map(m => m.movie_id)));
    }
  };

  const deleteSelected = async () => {
    if (selectedIds.size === 0) {
      alert('No items selected');
      return;
    }

    if (!confirm(`Delete ${selectedIds.size} cached stream(s)? This cannot be undone.`)) {
      return;
    }

    try {
      setDeleting(true);
      const movieIds = Array.from(selectedIds);
      
      // Delete each selected movie
      for (const movieId of movieIds) {
        try {
          await api.delete(`/streams/cache/${movieId}`);
        } catch (error) {
          console.error(`Failed to delete movie ${movieId}:`, error);
        }
      }
      
      alert(`✅ Deleted ${selectedIds.size} cached stream(s)`);
      setSelectedIds(new Set());
      fetchData(); // Refresh data
    } catch (error: any) {
      alert(`❌ Deletion failed: ${error.response?.data?.error || error.message}`);
    } finally {
      setDeleting(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 30000); // Refresh every 30 seconds
    return () => clearInterval(interval);
  }, []);

  const filteredMovies = movies.filter(movie => {
    if (filter === 'available') return movie.is_available;
    if (filter === 'unavailable') return !movie.is_available;
    if (filter === 'upgrades') return movie.upgrade_available;
    return true;
  });

  const isAllSelected = filteredMovies.length > 0 && selectedIds.size === filteredMovies.length;

  const getQualityBadgeColor = (score: number) => {
    if (score >= 80) return 'bg-green-500/20 text-green-400 border-green-500/50';
    if (score >= 60) return 'bg-blue-500/20 text-blue-400 border-blue-500/50';
    if (score >= 40) return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/50';
    return 'bg-red-500/20 text-red-400 border-red-500/50';
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    
    if (diffDays === 0) return 'Today';
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7) return `${diffDays} days ago`;
    return date.toLocaleDateString();
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-96">
        <RefreshCw className="w-8 h-8 text-blue-500 animate-spin" />
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-white flex items-center gap-3">
            <Database className="w-8 h-8" />
            Stream Cache Monitor
          </h1>
          <p className="text-slate-400 mt-1">Track cached streams, quality, and upgrade status</p>
        </div>
        <div className="flex gap-2 flex-wrap">
          {selectedIds.size > 0 && (
            <button
              onClick={deleteSelected}
              disabled={deleting}
              className="px-4 py-2 bg-red-600 hover:bg-red-700 rounded-lg text-white flex items-center gap-2 disabled:opacity-50 font-medium"
            >
              <XCircle className={`w-4 h-4 ${deleting ? 'animate-spin' : ''}`} />
              Delete Selected ({selectedIds.size})
            </button>
          )}
          <button
            onClick={cleanupUnreleased}
            disabled={cleaningUp}
            className="px-4 py-2 bg-red-500 hover:bg-red-600 rounded-lg text-white flex items-center gap-2 disabled:opacity-50"
          >
            <XCircle className={`w-4 h-4 ${cleaningUp ? 'animate-spin' : ''}`} />
            Cleanup Unreleased
          </button>
          <button
            onClick={fetchData}
            disabled={refreshing}
            className="px-4 py-2 bg-blue-500 hover:bg-blue-600 rounded-lg text-white flex items-center gap-2 disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      {/* Stats Cards */}
      {stats && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div className="bg-gradient-to-br from-blue-500/10 to-blue-600/10 border border-blue-500/20 rounded-xl p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-slate-400">Total Cached</p>
                <p className="text-3xl font-bold text-white mt-1">{stats.total_cached}</p>
              </div>
              <Film className="w-10 h-10 text-blue-400 opacity-50" />
            </div>
          </div>

          <div className="bg-gradient-to-br from-green-500/10 to-green-600/10 border border-green-500/20 rounded-xl p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-slate-400">Available</p>
                <p className="text-3xl font-bold text-white mt-1">{stats.available}</p>
              </div>
              <CheckCircle className="w-10 h-10 text-green-400 opacity-50" />
            </div>
          </div>

          <div className="bg-gradient-to-br from-red-500/10 to-red-600/10 border border-red-500/20 rounded-xl p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-slate-400">Unavailable</p>
                <p className="text-3xl font-bold text-white mt-1">{stats.unavailable}</p>
              </div>
              <XCircle className="w-10 h-10 text-red-400 opacity-50" />
            </div>
          </div>

          <div className="bg-gradient-to-br from-purple-500/10 to-purple-600/10 border border-purple-500/20 rounded-xl p-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-slate-400">Avg Quality</p>
                <p className="text-3xl font-bold text-white mt-1">{stats.avg_quality_score.toFixed(0)}</p>
              </div>
              <TrendingUp className="w-10 h-10 text-purple-400 opacity-50" />
            </div>
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="flex gap-2">
        <button
          onClick={() => setFilter('all')}
          className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            filter === 'all' ? 'bg-blue-500 text-white' : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
          }`}
        >
          All ({movies.length})
        </button>
        <button
          onClick={() => setFilter('available')}
          className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            filter === 'available' ? 'bg-green-500 text-white' : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
          }`}
        >
          Available ({movies.filter(m => m.is_available).length})
        </button>
        <button
          onClick={() => setFilter('unavailable')}
          className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            filter === 'unavailable' ? 'bg-red-500 text-white' : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
          }`}
        >
          Unavailable ({movies.filter(m => !m.is_available).length})
        </button>
        <button
          onClick={() => setFilter('upgrades')}
          className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
            filter === 'upgrades' ? 'bg-purple-500 text-white' : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
          }`}
        >
          Upgrades Available ({movies.filter(m => m.upgrade_available).length})
        </button>
      </div>

      {/* Movies Table */}
      <div className="bg-[#1e1e1e] rounded-xl border border-white/10 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-slate-800/50 border-b border-white/10">
              <tr>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300 w-10">
                  <input
                    type="checkbox"
                    checked={isAllSelected}
                    onChange={toggleSelectAll}
                    className="w-4 h-4 rounded cursor-pointer"
                    title="Select all"
                  />
                </th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Movie</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Quality</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Resolution</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Source</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Size</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Status</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Cached</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-slate-300">Last Check</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/5">
              {filteredMovies.length === 0 ? (
                <tr>
                  <td colSpan={9} className="px-4 py-8 text-center text-slate-400">
                    No cached streams found
                  </td>
                </tr>
              ) : (
                filteredMovies.map((movie) => (
                  <tr
                    key={movie.movie_id}
                    className={`transition-colors ${
                      selectedIds.has(movie.movie_id)
                        ? 'bg-blue-500/10'
                        : 'hover:bg-white/5'
                    }`}
                  >
                    <td className="px-4 py-3 text-center">
                      <input
                        type="checkbox"
                        checked={selectedIds.has(movie.movie_id)}
                        onChange={() => toggleSelection(movie.movie_id)}
                        className="w-4 h-4 rounded cursor-pointer"
                      />
                    </td>
                    <td className="px-4 py-3">
                      <div>
                        <p className="text-white font-medium">{movie.title}</p>
                        <p className="text-xs text-slate-400">{movie.year}</p>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className={`inline-flex items-center px-2 py-1 rounded-md text-xs font-medium border ${getQualityBadgeColor(movie.quality_score)}`}>
                        Score: {movie.quality_score}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-col gap-1">
                        <span className="text-sm text-white">{movie.resolution}</span>
                        {movie.hdr_type && movie.hdr_type !== 'SDR' && (
                          <span className="text-xs text-purple-400">{movie.hdr_type}</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-col gap-1">
                        <span className="text-sm text-white">{movie.source_type}</span>
                        {movie.audio_format && (
                          <span className="text-xs text-blue-400">{movie.audio_format}</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-sm text-slate-300">{movie.file_size_gb.toFixed(1)} GB</span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-col gap-1">
                        {movie.is_available ? (
                          <span className="inline-flex items-center gap-1 text-xs text-green-400">
                            <CheckCircle className="w-3 h-3" />
                            Available
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 text-xs text-red-400">
                            <XCircle className="w-3 h-3" />
                            Unavailable
                          </span>
                        )}
                        {movie.upgrade_available && (
                          <span className="inline-flex items-center gap-1 text-xs text-purple-400">
                            <TrendingUp className="w-3 h-3" />
                            Upgrade Ready
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-xs text-slate-400">
                        <Calendar className="w-3 h-3" />
                        {formatDate(movie.cached_at)}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1 text-xs text-slate-400">
                        <Clock className="w-3 h-3" />
                        {formatDate(movie.last_checked)}
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
