import { useState, useMemo } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import type { TrendingItem } from '../services/api';
import { Search as SearchIcon, Film, Tv, Plus, Check, Loader2, Star, TrendingUp, Play, Flame, ChevronLeft, ChevronRight } from 'lucide-react';
import type { SearchResult, CalendarEntry } from '../types';

export default function Search() {
  const [searchTerm, setSearchTerm] = useState('');
  const [searchType, setSearchType] = useState<'all' | 'movies' | 'series'>('all');
  const [isSearching, setIsSearching] = useState(false);
  const [movieResults, setMovieResults] = useState<SearchResult[]>([]);
  const [seriesResults, setSeriesResults] = useState<SearchResult[]>([]);
  const [addingId, setAddingId] = useState<number | null>(null);
  const [newlyAddedIds, setNewlyAddedIds] = useState<Set<number>>(new Set());
  
  // Trending toggles
  const [trendingWindow, setTrendingWindow] = useState<'day' | 'week'>('day');
  const [popularType, setPopularType] = useState<'movie' | 'tv'>('movie');
  const [nowPlayingType, setNowPlayingType] = useState<'movie' | 'tv'>('movie');

  const queryClient = useQueryClient();

  // Fetch user's library to check what's already added
  const { data: libraryMovies = [] } = useQuery({
    queryKey: ['movies', 'all'],
    queryFn: () => streamarrApi.getMovies({ limit: 10000 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  const { data: librarySeries = [] } = useQuery({
    queryKey: ['series', 'all'],
    queryFn: () => streamarrApi.getSeries({ limit: 10000 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Create a set of TMDB IDs that are already in the library
  const libraryTmdbIds = useMemo(() => {
    const ids = new Set<number>();
    const movies = Array.isArray(libraryMovies) ? libraryMovies : [];
    const series = Array.isArray(librarySeries) ? librarySeries : [];
    movies.forEach(m => {
      if (m.tmdb_id) ids.add(m.tmdb_id);
    });
    series.forEach(s => {
      if (s.tmdb_id) ids.add(s.tmdb_id);
    });
    return ids;
  }, [libraryMovies, librarySeries]);

  // Check if an item is in the library
  const isInLibrary = (tmdbId: number) => {
    return libraryTmdbIds.has(tmdbId) || newlyAddedIds.has(tmdbId);
  };

  // Get trending data
  const { data: trendingData = [] } = useQuery({
    queryKey: ['trending', trendingWindow],
    queryFn: () => streamarrApi.getTrending('all', trendingWindow).then(res => Array.isArray(res.data) ? res.data : []),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });

  // Get popular data
  const { data: popularData = [] } = useQuery({
    queryKey: ['popular', popularType],
    queryFn: () => streamarrApi.getPopular(popularType).then(res => Array.isArray(res.data) ? res.data : []),
    staleTime: 5 * 60 * 1000,
  });

  // Get now playing data
  const { data: nowPlayingData = [] } = useQuery({
    queryKey: ['nowPlaying', nowPlayingType],
    queryFn: () => streamarrApi.getNowPlaying(nowPlayingType).then(res => Array.isArray(res.data) ? res.data : []),
    staleTime: 5 * 60 * 1000,
  });

  // Get upcoming releases for widgets
  const today = new Date();
  const nextMonth = new Date(today);
  nextMonth.setMonth(nextMonth.getMonth() + 1);
  
  const { data: upcomingEntries = [] } = useQuery({
    queryKey: ['calendar', 'upcoming'],
    queryFn: () => streamarrApi.getCalendar(
      today.toISOString().split('T')[0],
      nextMonth.toISOString().split('T')[0]
    ).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Split upcoming into movies and series/episodes
  const upcomingMovies = upcomingEntries.filter(e => e.type === 'movie').slice(0, 5);
  const upcomingEpisodes = upcomingEntries.filter(e => e.type === 'episode').slice(0, 5);

  // Add movie mutation
  const addMovieMutation = useMutation({
    mutationFn: (tmdbId: number) => {
      console.log('Adding movie with TMDB ID:', tmdbId);
      return streamarrApi.addMovie({ tmdb_id: tmdbId, monitored: true });
    },
    onSuccess: (response, tmdbId) => {
      console.log('Movie added successfully:', response, tmdbId);
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      setNewlyAddedIds(prev => new Set(prev).add(tmdbId));
      setAddingId(null);
    },
    onError: (error) => {
      console.error('Failed to add movie:', error);
      setAddingId(null);
    }
  });

  // Add series mutation
  const addSeriesMutation = useMutation({
    mutationFn: (tmdbId: number) => {
      console.log('Adding series with TMDB ID:', tmdbId);
      return streamarrApi.addSeries({ tmdb_id: tmdbId, monitored: true });
    },
    onSuccess: (response, tmdbId) => {
      console.log('Series added successfully:', response, tmdbId);
      queryClient.invalidateQueries({ queryKey: ['series'] });
      setNewlyAddedIds(prev => new Set(prev).add(tmdbId));
      setAddingId(null);
    },
    onError: (error) => {
      console.error('Failed to add series:', error);
      setAddingId(null);
    }
  });

  const handleSearch = async () => {
    if (!searchTerm.trim()) return;
    
    setIsSearching(true);
    setMovieResults([]);
    setSeriesResults([]);

    try {
      if (searchType === 'all' || searchType === 'movies') {
        const moviesRes = await streamarrApi.searchMovies(searchTerm);
        setMovieResults(moviesRes.data || []);
      }
      if (searchType === 'all' || searchType === 'series') {
        const seriesRes = await streamarrApi.searchSeries(searchTerm);
        setSeriesResults(seriesRes.data || []);
      }
    } catch (error) {
      console.error('Search failed:', error);
    } finally {
      setIsSearching(false);
    }
  };

  const handleAdd = (item: SearchResult | TrendingItem, mediaType?: string) => {
    const id = item.id;
    const type = mediaType || ('media_type' in item ? item.media_type : 'movie');
    console.log('handleAdd called:', { id, type, item });
    setAddingId(id);
    if (type === 'movie') {
      addMovieMutation.mutate(id);
    } else {
      addSeriesMutation.mutate(id);
    }
  };

  const hasResults = movieResults.length > 0 || seriesResults.length > 0;

  return (
    <div className="p-6 space-y-6">
      {/* Header & Search Bar */}
      <div>
        <h1 className="text-2xl font-bold text-white mb-4">Search & Discover</h1>
        
        <div className="flex gap-4">
          <div className="flex-1 relative">
            <SearchIcon className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
              placeholder="Search for movies or TV series..."
              className="w-full pl-12 pr-4 py-3 bg-gray-800 border border-gray-700 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:border-blue-500"
            />
          </div>
          
          <select
            value={searchType}
            onChange={(e) => setSearchType(e.target.value as any)}
            className="px-4 py-3 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
          >
            <option value="all">All</option>
            <option value="movies">Movies</option>
            <option value="series">Series</option>
          </select>
          
          <button
            onClick={handleSearch}
            disabled={isSearching || !searchTerm.trim()}
            className="px-6 py-3 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-600 text-white rounded-lg font-medium flex items-center gap-2"
          >
            {isSearching ? (
              <Loader2 className="w-5 h-5 animate-spin" />
            ) : (
              <SearchIcon className="w-5 h-5" />
            )}
            Search
          </button>
        </div>
      </div>

      {/* Upcoming Widgets - styled same as Library page */}
      <div className="space-y-6">
        {/* Upcoming Movies Widget */}
        {upcomingMovies.length > 0 && (
          <UpcomingWidget
            title="Upcoming Movies (In Your Library)"
            icon={<Film className="w-5 h-5 text-purple-400" />}
            entries={upcomingMovies}
          />
        )}

        {/* Upcoming Episodes Widget */}
        {upcomingEpisodes.length > 0 && (
          <UpcomingWidget
            title="Upcoming Episodes (In Your Library)"
            icon={<Tv className="w-5 h-5 text-green-400" />}
            entries={upcomingEpisodes}
          />
        )}
      </div>

      {/* Search Results */}
      {hasResults && (
        <div className="space-y-6">
          {/* Movie Results */}
          {movieResults.length > 0 && (
            <div>
              <h2 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
                <Film className="w-5 h-5 text-purple-400" />
                Movies ({movieResults.length})
              </h2>
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
                {movieResults.map((result) => (
                  <SearchResultCard
                    key={`movie-${result.id}`}
                    result={{ ...result, media_type: 'movie' }}
                    onAdd={handleAdd}
                    isAdding={addingId === result.id}
                    isAdded={isInLibrary(result.id)}
                  />
                ))}
              </div>
            </div>
          )}

          {/* Series Results */}
          {seriesResults.length > 0 && (
            <div>
              <h2 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
                <Tv className="w-5 h-5 text-green-400" />
                TV Series ({seriesResults.length})
              </h2>
              <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
                {seriesResults.map((result) => (
                  <SearchResultCard
                    key={`series-${result.id}`}
                    result={{ ...result, media_type: 'tv' }}
                    onAdd={handleAdd}
                    isAdding={addingId === result.id}
                    isAdded={isInLibrary(result.id)}
                  />
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Trending Section */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-bold text-white flex items-center gap-2">
            <TrendingUp className="w-6 h-6 text-cyan-400" />
            Trending
          </h2>
          <div className="flex bg-gray-800 rounded-lg p-1">
            <button
              onClick={() => setTrendingWindow('day')}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                trendingWindow === 'day' 
                  ? 'bg-cyan-600 text-white' 
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              Today
            </button>
            <button
              onClick={() => setTrendingWindow('week')}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                trendingWindow === 'week' 
                  ? 'bg-cyan-600 text-white' 
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              This Week
            </button>
          </div>
        </div>
        
        <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent">
          {trendingData.slice(0, 20).map((item, index) => (
            <TrendingCard
              key={`trending-${item.id}-${item.media_type}`}
              item={item}
              rank={index + 1}
              onAdd={(item) => handleAdd(item, item.media_type)}
              isAdding={addingId === item.id}
              isAdded={isInLibrary(item.id)}
            />
          ))}
        </div>
      </section>

      {/* Popular Section */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-bold text-white flex items-center gap-2">
            <Flame className="w-6 h-6 text-orange-400" />
            What's Popular
          </h2>
          <div className="flex bg-gray-800 rounded-lg p-1">
            <button
              onClick={() => setPopularType('movie')}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                popularType === 'movie' 
                  ? 'bg-orange-600 text-white' 
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              Movies
            </button>
            <button
              onClick={() => setPopularType('tv')}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                popularType === 'tv' 
                  ? 'bg-orange-600 text-white' 
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              On TV
            </button>
          </div>
        </div>
        
        <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent">
          {popularData.slice(0, 20).map((item) => (
            <TrendingCard
              key={`popular-${item.id}`}
              item={item}
              onAdd={(item) => handleAdd(item, popularType)}
              isAdding={addingId === item.id}
              isAdded={isInLibrary(item.id)}
            />
          ))}
        </div>
      </section>

      {/* Now Playing Section */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-bold text-white flex items-center gap-2">
            <Play className="w-6 h-6 text-green-400 fill-current" />
            {nowPlayingType === 'movie' ? 'Now Playing in Theaters' : 'Currently Airing'}
          </h2>
          <div className="flex bg-gray-800 rounded-lg p-1">
            <button
              onClick={() => setNowPlayingType('movie')}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                nowPlayingType === 'movie' 
                  ? 'bg-green-600 text-white' 
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              Movies
            </button>
            <button
              onClick={() => setNowPlayingType('tv')}
              className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                nowPlayingType === 'tv' 
                  ? 'bg-green-600 text-white' 
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              TV Shows
            </button>
          </div>
        </div>
        
        <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent">
          {nowPlayingData.slice(0, 20).map((item) => (
            <TrendingCard
              key={`nowplaying-${item.id}`}
              item={item}
              onAdd={(item) => handleAdd(item, nowPlayingType)}
              isAdding={addingId === item.id}
              isAdded={isInLibrary(item.id)}
            />
          ))}
        </div>
      </section>
    </div>
  );
}

// Trending Card Component
function TrendingCard({ 
  item, 
  rank,
  onAdd, 
  isAdding, 
  isAdded 
}: { 
  item: TrendingItem; 
  rank?: number;
  onAdd: (item: TrendingItem) => void; 
  isAdding: boolean;
  isAdded: boolean;
}) {
  const year = item.release_date ? new Date(item.release_date).getFullYear() : null;
  const isMovie = item.media_type === 'movie';
  
  return (
    <div className="flex-shrink-0 w-32 group">
      <div className="relative aspect-[2/3] rounded-lg overflow-hidden bg-gray-700">
        <img
          src={tmdbImageUrl(item.poster_path, 'w200')}
          alt={item.title}
          className="w-full h-full object-cover"
        />
        
        {/* Rank badge */}
        {rank && (
          <div className="absolute -bottom-1 -left-1 text-4xl font-black text-white opacity-80" style={{ textShadow: '2px 2px 4px rgba(0,0,0,0.8), -1px -1px 2px rgba(0,0,0,0.8)' }}>
            {rank}
          </div>
        )}
        
        {/* Rating circle */}
        <div className="absolute top-1 right-1">
          <div className={`w-7 h-7 rounded-full flex items-center justify-center text-[10px] font-bold ${
            item.vote_average >= 7 ? 'bg-green-500' : 
            item.vote_average >= 5 ? 'bg-yellow-500' : 'bg-red-500'
          }`}>
            {Math.round(item.vote_average * 10)}%
          </div>
        </div>
        
        {/* Hover overlay */}
        <div className="absolute inset-0 bg-black/70 opacity-0 group-hover:opacity-100 transition-opacity flex flex-col items-center justify-center gap-1 p-2">
          <button
            onClick={() => onAdd(item)}
            disabled={isAdding || isAdded}
            className={`px-2 py-1 rounded text-xs font-medium flex items-center gap-1 ${
              isAdded 
                ? 'bg-green-600 text-white' 
                : 'bg-blue-600 hover:bg-blue-500 text-white'
            }`}
          >
            {isAdding ? (
              <Loader2 className="w-3 h-3 animate-spin" />
            ) : isAdded ? (
              <Check className="w-3 h-3" />
            ) : (
              <Plus className="w-3 h-3" />
            )}
            {isAdded ? 'Added' : 'Add'}
          </button>
        </div>
        
        {/* Type badge */}
        <div className={`absolute top-1 left-1 px-1 py-0.5 rounded text-[10px] font-medium ${
          isMovie ? 'bg-purple-600 text-white' : 'bg-green-600 text-white'
        }`}>
          {isMovie ? 'Movie' : 'TV'}
        </div>
      </div>
      
      <div className="mt-1.5">
        <h3 className="text-white font-medium text-xs truncate" title={item.title}>
          {item.title}
        </h3>
        {year && (
          <p className="text-gray-400 text-[10px]">{year}</p>
        )}
      </div>
    </div>
  );
}

// Search Result Card Component
function SearchResultCard({ 
  result, 
  onAdd, 
  isAdding, 
  isAdded 
}: { 
  result: SearchResult; 
  onAdd: (r: SearchResult) => void; 
  isAdding: boolean;
  isAdded: boolean;
}) {
  const year = result.release_date ? new Date(result.release_date).getFullYear() : null;
  
  return (
    <div className="bg-gray-700 rounded-lg overflow-hidden group relative">
      <div className="aspect-[2/3] relative">
        <img
          src={tmdbImageUrl(result.poster_path, 'w500')}
          alt={result.title}
          className="w-full h-full object-cover"
        />
        
        {/* Overlay on hover */}
        <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex flex-col items-center justify-center gap-2 p-2">
          <button
            onClick={() => onAdd(result)}
            disabled={isAdding || isAdded}
            className={`px-4 py-2 rounded-lg font-medium flex items-center gap-2 ${
              isAdded 
                ? 'bg-green-600 text-white' 
                : 'bg-blue-600 hover:bg-blue-500 text-white'
            }`}
          >
            {isAdding ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : isAdded ? (
              <Check className="w-4 h-4" />
            ) : (
              <Plus className="w-4 h-4" />
            )}
            {isAdded ? 'Added' : 'Add'}
          </button>
          
          {result.vote_average > 0 && (
            <div className="flex items-center gap-1 text-yellow-400 text-sm">
              <Star className="w-4 h-4 fill-current" />
              {result.vote_average.toFixed(1)}
            </div>
          )}
        </div>
        
        {/* Type badge */}
        <div className={`absolute top-2 left-2 px-2 py-0.5 rounded text-xs font-medium ${
          result.media_type === 'movie' 
            ? 'bg-purple-600 text-white' 
            : 'bg-green-600 text-white'
        }`}>
          {result.media_type === 'movie' ? 'Movie' : 'Series'}
        </div>
      </div>
      
      <div className="p-3">
        <h3 className="text-white font-medium text-sm truncate" title={result.title}>
          {result.title}
        </h3>
        {year && (
          <p className="text-gray-400 text-xs">{year}</p>
        )}
      </div>
    </div>
  );
}

// Upcoming Widget - horizontal scrollable poster cards (same style as Library)
function UpcomingWidget({ title, icon, entries }: { title: string; icon: React.ReactNode; entries: CalendarEntry[] }) {
  const scrollContainer = (id: string, direction: 'left' | 'right') => {
    const container = document.getElementById(id);
    if (container) {
      const scrollAmount = direction === 'left' ? -300 : 300;
      container.scrollBy({ left: scrollAmount, behavior: 'smooth' });
    }
  };

  const widgetId = `upcoming-${title.replace(/\s+/g, '-').toLowerCase()}`;

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  };

  return (
    <div className="bg-slate-800/50 rounded-xl p-4">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          {icon}
          <h3 className="text-lg font-semibold text-white">{title}</h3>
          <span className="text-slate-400 text-sm">({entries.length})</span>
        </div>
        <div className="flex gap-1">
          <button
            onClick={() => scrollContainer(widgetId, 'left')}
            className="p-1.5 rounded-lg bg-slate-700 hover:bg-slate-600 text-slate-300 transition-colors"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>
          <button
            onClick={() => scrollContainer(widgetId, 'right')}
            className="p-1.5 rounded-lg bg-slate-700 hover:bg-slate-600 text-slate-300 transition-colors"
          >
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      </div>
      
      <div
        id={widgetId}
        className="flex gap-3 overflow-x-auto pb-2 scrollbar-hide"
        style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
      >
        {entries.map((entry, idx) => (
          <div
            key={`${entry.type}-${entry.id}-${idx}`}
            className="flex-shrink-0 w-32 group"
          >
            <div className="relative aspect-[2/3] rounded-lg overflow-hidden mb-2">
              <img
                src={tmdbImageUrl(entry.poster_path || '', 'w200')}
                alt={entry.title}
                className="w-full h-full object-cover group-hover:scale-105 transition-transform"
              />
              {/* Type badge */}
              <div className={`absolute top-1 left-1 px-1 py-0.5 rounded text-[10px] font-medium ${
                entry.type === 'movie' ? 'bg-purple-600' : 'bg-green-600'
              } text-white`}>
                {entry.type === 'movie' ? 'Movie' : 'Series'}
              </div>
              {/* Release date badge */}
              <div className="absolute top-1 right-1 px-1 py-0.5 rounded bg-blue-600/90 text-white text-[10px] font-medium">
                {formatDate(entry.date)}
              </div>
              {/* Rating badge */}
              {entry.vote_average && entry.vote_average > 0 && (
                <div className="absolute bottom-1 right-1 px-1 py-0.5 rounded bg-black/70 text-yellow-400 text-[10px] font-medium">
                  ★ {entry.vote_average.toFixed(1)}
                </div>
              )}
            </div>
            <p className="text-white text-xs font-medium truncate" title={entry.title}>
              {entry.title}
            </p>
            {entry.type === 'episode' ? (
              <p className="text-slate-400 text-[10px] truncate">
                S{entry.season_number}E{entry.episode_number}
              </p>
            ) : entry.year ? (
              <p className="text-slate-400 text-[10px]">{entry.year}</p>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}
