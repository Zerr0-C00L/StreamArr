import { useState, useMemo, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import type { TrendingItem } from '../services/api';
import { 
  Search as SearchIcon, Film, Tv, Plus, Check, Loader2, 
  TrendingUp, Flame, ChevronLeft, ChevronRight, Star
} from 'lucide-react';
import type { SearchResult } from '../types';
import MediaDetailsModal from '../components/MediaDetailsModal';

export default function Search() {
  const [searchTerm, setSearchTerm] = useState('');
  const [isSearching, setIsSearching] = useState(false);
  const [movieResults, setMovieResults] = useState<SearchResult[]>([]);
  const [seriesResults, setSeriesResults] = useState<SearchResult[]>([]);
  const [addingId, setAddingId] = useState<number | null>(null);
  const [newlyAddedIds, setNewlyAddedIds] = useState<Set<number>>(new Set());
  const [trendingWindow, setTrendingWindow] = useState<'day' | 'week'>('day');
  const [selectedMedia, setSelectedMedia] = useState<{ item: SearchResult | TrendingItem; mediaType: string } | null>(null);
  const [addError, setAddError] = useState<string | null>(null);

  const queryClient = useQueryClient();

  // Fetch user's library
  const { data: libraryMovies = [] } = useQuery({
    queryKey: ['movies', 'all'],
    queryFn: () => streamarrApi.getMovies({ limit: 10000 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  const { data: librarySeries = [] } = useQuery({
    queryKey: ['series', 'all'],
    queryFn: () => streamarrApi.getSeries({ limit: 10000 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  const libraryTmdbIds = useMemo(() => {
    const ids = new Set<number>();
    libraryMovies.forEach((m: any) => m.tmdb_id && ids.add(m.tmdb_id));
    librarySeries.forEach((s: any) => s.tmdb_id && ids.add(s.tmdb_id));
    return ids;
  }, [libraryMovies, librarySeries]);

  const isInLibrary = (tmdbId: number) => libraryTmdbIds.has(tmdbId) || newlyAddedIds.has(tmdbId);

  // Trending data
  const { data: trendingData = [] } = useQuery({
    queryKey: ['trending', trendingWindow],
    queryFn: () => streamarrApi.getTrending('all', trendingWindow).then(res => Array.isArray(res.data) ? res.data : []),
    staleTime: 5 * 60 * 1000,
  });

  // Popular data
  const { data: popularMovies = [] } = useQuery({
    queryKey: ['popular', 'movie'],
    queryFn: () => streamarrApi.getPopular('movie').then(res => Array.isArray(res.data) ? res.data : []),
    staleTime: 5 * 60 * 1000,
  });

  const { data: popularSeries = [] } = useQuery({
    queryKey: ['popular', 'tv'],
    queryFn: () => streamarrApi.getPopular('tv').then(res => Array.isArray(res.data) ? res.data : []),
    staleTime: 5 * 60 * 1000,
  });

  // Add mutations
  const addMovieMutation = useMutation({
    mutationFn: (tmdbId: number) => streamarrApi.addMovie({ tmdb_id: tmdbId, monitored: true }),
    onSuccess: (data, tmdbId) => {
      console.log(`✓ Movie added successfully: ${data.title} (TMDB ID: ${tmdbId})`);
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      setNewlyAddedIds(prev => new Set(prev).add(tmdbId));
      setAddingId(null);
      setAddError(null);
    },
    onError: (error: any) => {
      const errorMsg = error.response?.data?.error || error.message || 'Failed to add movie';
      console.error('Failed to add movie:', errorMsg);
      setAddingId(null);
      setAddError(errorMsg);
      setTimeout(() => setAddError(null), 5000);
    },
    retry: 0,
  });

  const addSeriesMutation = useMutation({
    mutationFn: (tmdbId: number) => streamarrApi.addSeries({ tmdb_id: tmdbId, monitored: true }),
    onSuccess: (data, tmdbId) => {
      console.log(`✓ Series added successfully: ${data.title} (TMDB ID: ${tmdbId})`);
      queryClient.invalidateQueries({ queryKey: ['series'] });
      setNewlyAddedIds(prev => new Set(prev).add(tmdbId));
      setAddingId(null);
      setAddError(null);
    },
    onError: (error: any) => {
      const errorMsg = error.response?.data?.error || error.message || 'Failed to add series';
      console.error('Failed to add series:', errorMsg);
      setAddingId(null);
      setAddError(errorMsg);
      setTimeout(() => setAddError(null), 5000);
    },
    retry: 0,
  });

  const handleSearch = async () => {
    if (!searchTerm.trim()) return;
    setIsSearching(true);
    setMovieResults([]);
    setSeriesResults([]);

    try {
      const [moviesRes, seriesRes] = await Promise.all([
        streamarrApi.searchMovies(searchTerm),
        streamarrApi.searchSeries(searchTerm),
      ]);
      setMovieResults(moviesRes.data || []);
      setSeriesResults(seriesRes.data || []);
    } catch (error) {
      console.error('Search failed:', error);
    } finally {
      setIsSearching(false);
    }
  };

  const handleAdd = (item: SearchResult | TrendingItem, mediaType?: string) => {
    const tmdbId = ('tmdb_id' in item && item.tmdb_id) ? item.tmdb_id : item.id;
    const type = mediaType || ('media_type' in item ? item.media_type : 'movie');
    setAddingId(tmdbId as number);
    if (type === 'movie') {
      addMovieMutation.mutate(tmdbId as number);
    } else {
      addSeriesMutation.mutate(tmdbId as number);
    }
  };

  const hasResults = movieResults.length > 0 || seriesResults.length > 0;

  return (
    <div className="min-h-screen bg-[#141414] -m-6">
      {/* Error Message */}
      {addError && (
        <div className="fixed top-20 left-1/2 -translate-x-1/2 z-50 px-6 py-3 bg-red-600 text-white rounded-lg shadow-lg">
          {addError}
        </div>
      )}

      {/* Hero Search Section */}
      <div className="relative h-[40vh] min-h-[300px] flex items-center justify-center">
        <div className="absolute inset-0 bg-gradient-to-b from-[#141414]/50 via-transparent to-[#141414]" />
        <div className="absolute inset-0 bg-gradient-to-r from-purple-900/20 via-transparent to-cyan-900/20" />
        
        <div className="relative z-10 w-full max-w-3xl px-6">
          <h1 className="text-4xl md:text-5xl font-black text-white text-center mb-6">
            Discover New Content
          </h1>
          
          <div className="relative">
            <SearchIcon className="absolute left-5 top-1/2 -translate-y-1/2 w-6 h-6 text-slate-400" />
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
              placeholder="Search for movies or TV shows..."
              className="w-full pl-14 pr-32 py-4 bg-[#333] border-2 border-transparent rounded-full text-white text-lg
                         placeholder-slate-500 focus:outline-none focus:border-white/30 transition-all"
            />
            <button
              onClick={handleSearch}
              disabled={isSearching || !searchTerm.trim()}
              className="absolute right-2 top-1/2 -translate-y-1/2 px-6 py-2 bg-white text-black font-bold rounded-full
                         hover:bg-white/90 disabled:bg-slate-600 disabled:text-slate-400 transition-colors"
            >
              {isSearching ? <Loader2 className="w-5 h-5 animate-spin" /> : 'Search'}
            </button>
          </div>
        </div>
      </div>

      {/* Search Results */}
      {hasResults && (
        <div className="px-8 pb-8 -mt-10 relative z-10">
          <h2 className="text-2xl font-bold text-white mb-6">Search Results</h2>
          
          {movieResults.length > 0 && (
            <div className="mb-8">
              <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
                <Film className="w-5 h-5 text-purple-500" />
                Movies ({movieResults.length})
              </h3>
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-4">
                {movieResults.map((result) => (
                  <MediaCard
                    key={`movie-${result.tmdb_id || result.id}`}
                    item={result}
                    mediaType="movie"
                    onAdd={handleAdd}
                    isAdding={addingId === (result.tmdb_id || result.id)}
                    isAdded={isInLibrary(result.tmdb_id || result.id)}
                    onClick={setSelectedMedia}
                  />
                ))}
              </div>
            </div>
          )}

          {seriesResults.length > 0 && (
            <div className="mb-8">
              <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
                <Tv className="w-5 h-5 text-green-500" />
                TV Series ({seriesResults.length})
              </h3>
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-4">
                {seriesResults.map((result) => (
                  <MediaCard
                    key={`series-${result.tmdb_id || result.id}`}
                    item={result}
                    mediaType="tv"
                    onAdd={handleAdd}
                    isAdding={addingId === (result.tmdb_id || result.id)}
                    isAdded={isInLibrary(result.tmdb_id || result.id)}
                    onClick={setSelectedMedia}
                  />
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Trending Section */}
      <div className="px-8 pb-8">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-bold text-white flex items-center gap-2">
            <TrendingUp className="w-6 h-6 text-cyan-500" />
            Trending Now
          </h2>
          <div className="flex bg-[#333] rounded-full p-1">
            <button
              onClick={() => setTrendingWindow('day')}
              className={`px-4 py-1.5 rounded-full text-sm font-medium transition-colors ${
                trendingWindow === 'day' ? 'bg-white text-black' : 'text-slate-400 hover:text-white'
              }`}
            >
              Today
            </button>
            <button
              onClick={() => setTrendingWindow('week')}
              className={`px-4 py-1.5 rounded-full text-sm font-medium transition-colors ${
                trendingWindow === 'week' ? 'bg-white text-black' : 'text-slate-400 hover:text-white'
              }`}
            >
              This Week
            </button>
          </div>
        </div>
        
        <ContentRow
          items={trendingData}
          onAdd={handleAdd}
          addingId={addingId}
          isInLibrary={isInLibrary}
          onCardClick={setSelectedMedia}
        />
      </div>

      {/* Popular Movies */}
      <div className="px-8 pb-8">
        <h2 className="text-xl font-bold text-white flex items-center gap-2 mb-4">
          <Flame className="w-6 h-6 text-orange-500" />
          Popular Movies
        </h2>
        <ContentRow
          items={popularMovies}
          onAdd={handleAdd}
          addingId={addingId}
          isInLibrary={isInLibrary}
          mediaType="movie"
          onCardClick={setSelectedMedia}
        />
      </div>

      {/* Popular Series */}
      <div className="px-8 pb-8">
        <h2 className="text-xl font-bold text-white flex items-center gap-2 mb-4">
          <Tv className="w-6 h-6 text-green-500" />
          Popular TV Shows
        </h2>
        <ContentRow
          items={popularSeries}
          onAdd={handleAdd}
          addingId={addingId}
          isInLibrary={isInLibrary}
          mediaType="tv"
          onCardClick={setSelectedMedia}
        />
      </div>

      {/* Media Details Modal */}
      {selectedMedia && (
        <MediaDetailsModal
          item={selectedMedia.item}
          mediaType={selectedMedia.mediaType}
          onClose={() => setSelectedMedia(null)}
          onAdd={handleAdd}
          isAdding={addingId === (('tmdb_id' in selectedMedia.item && selectedMedia.item.tmdb_id) ? selectedMedia.item.tmdb_id : selectedMedia.item.id)}
          isAdded={isInLibrary(('tmdb_id' in selectedMedia.item && selectedMedia.item.tmdb_id) ? selectedMedia.item.tmdb_id : selectedMedia.item.id)}
        />
      )}
    </div>
  );
}

// Content Row with horizontal scroll
function ContentRow({ 
  items, 
  onAdd, 
  addingId, 
  isInLibrary,
  mediaType,
  onCardClick
}: { 
  items: TrendingItem[]; 
  onAdd: (item: TrendingItem, type?: string) => void;
  addingId: number | null;
  isInLibrary: (id: number) => boolean;
  mediaType?: string;
  onCardClick?: (media: { item: TrendingItem; mediaType: string }) => void;
}) {
  const rowRef = useRef<HTMLDivElement>(null);

  const scroll = (direction: 'left' | 'right') => {
    if (rowRef.current) {
      const scrollAmount = rowRef.current.clientWidth * 0.8;
      rowRef.current.scrollBy({
        left: direction === 'left' ? -scrollAmount : scrollAmount,
        behavior: 'smooth'
      });
    }
  };

  if (items.length === 0) return null;

  return (
    <div className="relative group/row">
      <button
        onClick={() => scroll('left')}
        className="absolute left-0 top-0 bottom-0 z-30 w-12 bg-gradient-to-r from-[#141414] to-transparent
                   flex items-center justify-center opacity-0 group-hover/row:opacity-100 transition-opacity"
      >
        <ChevronLeft className="w-8 h-8 text-white" />
      </button>

      <div
        ref={rowRef}
        className="flex gap-3 overflow-x-auto scrollbar-hide pb-4"
        style={{ scrollbarWidth: 'none', msOverflowStyle: 'none' }}
      >
        {items.map((item) => (
          <MediaCard
            key={item.id}
            item={item}
            mediaType={mediaType || item.media_type || 'movie'}
            onAdd={onAdd}
            isAdding={addingId === item.id}
            isAdded={isInLibrary(item.id)}
            onClick={onCardClick}
          />
        ))}
      </div>

      <button
        onClick={() => scroll('right')}
        className="absolute right-0 top-0 bottom-0 z-30 w-12 bg-gradient-to-l from-[#141414] to-transparent
                   flex items-center justify-center opacity-0 group-hover/row:opacity-100 transition-opacity"
      >
        <ChevronRight className="w-8 h-8 text-white" />
      </button>
    </div>
  );
}

// Media Card Component
function MediaCard({ 
  item, 
  mediaType, 
  onAdd, 
  isAdding, 
  isAdded,
  onClick
}: { 
  item: SearchResult | TrendingItem;
  mediaType: string;
  onAdd: (item: any, type: string) => void;
  isAdding: boolean;
  isAdded: boolean;
  onClick?: (media: { item: SearchResult | TrendingItem; mediaType: string }) => void;
}) {
  const title = item.title || item.name || 'Unknown';
  const posterPath = item.poster_path;
  const year = item.release_date?.substring(0, 4) || item.first_air_date?.substring(0, 4);
  const rating = item.vote_average;
  const isMovie = mediaType === 'movie';

  return (
    <div 
      className="w-[150px] flex-shrink-0 group/card cursor-pointer"
      onClick={() => onClick?.({ item, mediaType })}
    >
      <div className="relative aspect-[2/3] rounded-lg overflow-hidden bg-[#333] mb-2">
        {posterPath ? (
          <img
            src={tmdbImageUrl(posterPath, 'w342')}
            alt={title}
            className="w-full h-full object-cover group-hover/card:scale-105 transition-transform duration-300"
            loading="lazy"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center">
            {isMovie ? <Film className="w-10 h-10 text-slate-600" /> : <Tv className="w-10 h-10 text-slate-600" />}
          </div>
        )}

        {/* Overlay */}
        <div className="absolute inset-0 bg-gradient-to-t from-black/90 via-black/20 to-transparent 
                       opacity-0 group-hover/card:opacity-100 transition-opacity flex flex-col justify-end p-3">
          <div className="flex gap-2 mb-2">
            {isAdded ? (
              <div className="p-2 rounded-full bg-green-600 text-white">
                <Check className="w-4 h-4" />
              </div>
            ) : (
              <button
                onClick={(e) => { e.stopPropagation(); onAdd(item, mediaType); }}
                disabled={isAdding}
                className="p-2 rounded-full bg-white/20 hover:bg-white/40 backdrop-blur-sm transition-colors"
              >
                {isAdding ? (
                  <Loader2 className="w-4 h-4 text-white animate-spin" />
                ) : (
                  <Plus className="w-4 h-4 text-white" />
                )}
              </button>
            )}
          </div>
          
          {rating && rating > 0 && (
            <div className="flex items-center gap-1 text-xs text-white/80">
              <Star className="w-3 h-3 text-yellow-500 fill-yellow-500" />
              {rating.toFixed(1)}
            </div>
          )}
        </div>

        {/* Type Badge */}
        <div className="absolute top-2 left-2">
          <span className={`px-1.5 py-0.5 rounded text-[10px] font-bold ${
            isMovie ? 'bg-purple-600' : 'bg-green-600'
          } text-white`}>
            {isMovie ? 'MOVIE' : 'TV'}
          </span>
        </div>

        {/* Added Badge */}
        {isAdded && (
          <div className="absolute top-2 right-2">
            <span className="px-1.5 py-0.5 rounded text-[10px] font-bold bg-green-600 text-white">
              ✓ ADDED
            </span>
          </div>
        )}
      </div>

      <h3 className="text-white text-sm font-medium truncate">{title}</h3>
      {year && <p className="text-slate-500 text-xs">{year}</p>}
    </div>
  );
}
