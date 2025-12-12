import { useState, useMemo } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import { Search, Film, Tv, Grid, List, Star, Clock, ChevronLeft, ChevronRight, Trash2, Loader2, ArrowUpDown, SortAsc, SortDesc, Calendar, TrendingUp } from 'lucide-react';
import type { Movie, Series, CalendarEntry } from '../types';

type SortOption = 'title-asc' | 'title-desc' | 'added-desc' | 'added-asc' | 'release-desc' | 'release-asc' | 'rating-desc' | 'rating-asc' | 'year-desc' | 'year-asc';

type MediaItem = {
  id: number;
  tmdb_id: number;
  title: string;
  poster_path: string;
  backdrop_path?: string;
  year?: number;
  type: 'movie' | 'series';
  monitored: boolean;
  overview?: string;
  vote_average?: number;
  added_at?: string;
  release_date?: string;
};

export default function Library() {
  const [searchTerm, setSearchTerm] = useState('');
  const [mediaFilter, setMediaFilter] = useState<'all' | 'movies' | 'series'>('all');
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [showWidgets, setShowWidgets] = useState(true);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [sortBy, setSortBy] = useState<SortOption>('title-asc');
  const [showSortMenu, setShowSortMenu] = useState(false);

  const queryClient = useQueryClient();

  // Delete movie mutation
  const deleteMovieMutation = useMutation({
    mutationFn: (id: number) => streamarrApi.deleteMovie(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['movies'] });
      setDeletingId(null);
    },
    onError: () => setDeletingId(null),
  });

  // Delete series mutation
  const deleteSeriesMutation = useMutation({
    mutationFn: (id: number) => streamarrApi.deleteSeries(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['series'] });
      setDeletingId(null);
    },
    onError: () => setDeletingId(null),
  });

  const handleRemove = (item: MediaItem) => {
    if (confirm(`Remove "${item.title}" from your library?`)) {
      setDeletingId(`${item.type}-${item.id}`);
      if (item.type === 'movie') {
        deleteMovieMutation.mutate(item.id);
      } else {
        deleteSeriesMutation.mutate(item.id);
      }
    }
  };

  // Fetch movies
  const { data: movies = [], isLoading: moviesLoading } = useQuery({
    queryKey: ['movies', 'library'],
    queryFn: () => streamarrApi.getMovies({ limit: 10000 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Fetch series
  const { data: series = [], isLoading: seriesLoading } = useQuery({
    queryKey: ['series', 'library'],
    queryFn: () => streamarrApi.getSeries({ limit: 10000 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Fetch upcoming calendar entries
  const today = new Date();
  const nextMonth = new Date(today);
  nextMonth.setMonth(nextMonth.getMonth() + 1);
  
  const { data: upcomingEntries = [] } = useQuery({
    queryKey: ['calendar', 'library-upcoming'],
    queryFn: () => streamarrApi.getCalendar(
      today.toISOString().split('T')[0],
      nextMonth.toISOString().split('T')[0]
    ).then(res => Array.isArray(res.data) ? res.data : []),
  });

  const isLoading = moviesLoading || seriesLoading;

  // Combine and normalize movies and series
  const allMedia: MediaItem[] = useMemo(() => {
    const movieItems: MediaItem[] = movies.map((m: Movie) => ({
      id: m.id,
      tmdb_id: m.tmdb_id,
      title: m.title,
      poster_path: m.poster_path || '',
      backdrop_path: m.backdrop_path,
      year: m.release_date ? new Date(m.release_date).getFullYear() : undefined,
      type: 'movie' as const,
      monitored: m.monitored,
      overview: m.overview,
      vote_average: m.metadata?.vote_average,
      added_at: m.added_at,
      release_date: m.release_date,
    }));

    const seriesItems: MediaItem[] = series.map((s: Series) => ({
      id: s.id,
      tmdb_id: s.tmdb_id,
      title: s.title,
      poster_path: s.poster_path || '',
      backdrop_path: s.backdrop_path,
      year: s.first_air_date ? new Date(s.first_air_date).getFullYear() : undefined,
      type: 'series' as const,
      monitored: s.monitored,
      overview: s.overview,
      vote_average: s.metadata?.vote_average,
      added_at: s.added_at,
      release_date: s.first_air_date,
    }));

    return [...movieItems, ...seriesItems];
  }, [movies, series]);

  // Widget data
  const recentlyAdded = useMemo(() => {
    return [...allMedia]
      .filter(item => item.added_at)
      .sort((a, b) => new Date(b.added_at!).getTime() - new Date(a.added_at!).getTime())
      .slice(0, 10);
  }, [allMedia]);

  const topRated = useMemo(() => {
    return [...allMedia]
      .filter(item => item.vote_average && item.vote_average > 0)
      .sort((a, b) => (b.vote_average || 0) - (a.vote_average || 0))
      .slice(0, 10);
  }, [allMedia]);

  const upcomingMovies = upcomingEntries.filter(e => e.type === 'movie').slice(0, 10);
  const upcomingEpisodes = upcomingEntries.filter(e => e.type === 'episode').slice(0, 10);

  // Filter and sort media
  const filteredMedia = useMemo(() => {
    let result = allMedia.filter(item => {
      const matchesSearch = item.title.toLowerCase().includes(searchTerm.toLowerCase());
      const matchesType = mediaFilter === 'all' || 
        (mediaFilter === 'movies' && item.type === 'movie') ||
        (mediaFilter === 'series' && item.type === 'series');
      return matchesSearch && matchesType;
    });

    // Apply sorting
    result.sort((a, b) => {
      switch (sortBy) {
        case 'title-asc':
          return a.title.localeCompare(b.title);
        case 'title-desc':
          return b.title.localeCompare(a.title);
        case 'added-desc':
          return new Date(b.added_at || 0).getTime() - new Date(a.added_at || 0).getTime();
        case 'added-asc':
          return new Date(a.added_at || 0).getTime() - new Date(b.added_at || 0).getTime();
        case 'release-desc':
          return new Date(b.release_date || 0).getTime() - new Date(a.release_date || 0).getTime();
        case 'release-asc':
          return new Date(a.release_date || 0).getTime() - new Date(b.release_date || 0).getTime();
        case 'rating-desc':
          return (b.vote_average || 0) - (a.vote_average || 0);
        case 'rating-asc':
          return (a.vote_average || 0) - (b.vote_average || 0);
        case 'year-desc':
          return (b.year || 0) - (a.year || 0);
        case 'year-asc':
          return (a.year || 0) - (b.year || 0);
        default:
          return 0;
      }
    });

    return result;
  }, [allMedia, searchTerm, mediaFilter, sortBy]);

  const sortOptions: { value: SortOption; label: string; icon: typeof SortAsc }[] = [
    { value: 'title-asc', label: 'Title (A-Z)', icon: SortAsc },
    { value: 'title-desc', label: 'Title (Z-A)', icon: SortDesc },
    { value: 'added-desc', label: 'Recently Added', icon: Clock },
    { value: 'added-asc', label: 'Oldest Added', icon: Clock },
    { value: 'release-desc', label: 'Release (Newest)', icon: Calendar },
    { value: 'release-asc', label: 'Release (Oldest)', icon: Calendar },
    { value: 'rating-desc', label: 'Rating (High)', icon: Star },
    { value: 'rating-asc', label: 'Rating (Low)', icon: Star },
    { value: 'year-desc', label: 'Year (Newest)', icon: TrendingUp },
    { value: 'year-asc', label: 'Year (Oldest)', icon: TrendingUp },
  ];

  const currentSortLabel = sortOptions.find(o => o.value === sortBy)?.label || 'Sort';

  const movieCount = movies.length;
  const seriesCount = series.length;

  return (
    <div className="p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white mb-1">Library</h1>
          <p className="text-slate-400 text-sm">
            {movieCount} movies · {seriesCount} series
          </p>
        </div>
        <button
          onClick={() => setShowWidgets(!showWidgets)}
          className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
            showWidgets 
              ? 'bg-blue-600 text-white' 
              : 'bg-slate-700 text-slate-300 hover:bg-slate-600'
          }`}
        >
          {showWidgets ? 'Hide Widgets' : 'Show Widgets'}
        </button>
      </div>

      {/* Search and Filters */}
      <div className="flex flex-wrap gap-4 mb-6">
        <div className="flex-1 min-w-[200px] relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-400" />
          <input
            type="text"
            placeholder="Search library..."
            className="w-full pl-10 pr-4 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-white placeholder-slate-400 focus:outline-none focus:border-blue-500"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </div>

        {/* Type Filter */}
        <div className="flex bg-slate-800 rounded-lg p-1">
          <button
            onClick={() => setMediaFilter('all')}
            className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
              mediaFilter === 'all' 
                ? 'bg-blue-600 text-white' 
                : 'text-slate-400 hover:text-white'
            }`}
          >
            All
          </button>
          <button
            onClick={() => setMediaFilter('movies')}
            className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1.5 ${
              mediaFilter === 'movies' 
                ? 'bg-purple-600 text-white' 
                : 'text-slate-400 hover:text-white'
            }`}
          >
            <Film className="w-4 h-4" />
            Movies
          </button>
          <button
            onClick={() => setMediaFilter('series')}
            className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1.5 ${
              mediaFilter === 'series' 
                ? 'bg-green-600 text-white' 
                : 'text-slate-400 hover:text-white'
            }`}
          >
            <Tv className="w-4 h-4" />
            Series
          </button>
        </div>

        {/* View Toggle */}
        <div className="flex bg-slate-800 rounded-lg p-1">
          <button
            onClick={() => setViewMode('grid')}
            className={`p-2 rounded-md transition-colors ${
              viewMode === 'grid' 
                ? 'bg-slate-700 text-white' 
                : 'text-slate-400 hover:text-white'
            }`}
          >
            <Grid className="w-5 h-5" />
          </button>
          <button
            onClick={() => setViewMode('list')}
            className={`p-2 rounded-md transition-colors ${
              viewMode === 'list' 
                ? 'bg-slate-700 text-white' 
                : 'text-slate-400 hover:text-white'
            }`}
          >
            <List className="w-5 h-5" />
          </button>
        </div>

        {/* Sort Dropdown */}
        <div className="relative">
          <button
            onClick={() => setShowSortMenu(!showSortMenu)}
            className="flex items-center gap-2 px-4 py-2.5 bg-slate-800 border border-slate-700 rounded-lg text-white hover:bg-slate-700 transition-colors"
          >
            <ArrowUpDown className="w-4 h-4" />
            <span className="text-sm">{currentSortLabel}</span>
          </button>
          
          {showSortMenu && (
            <>
              <div 
                className="fixed inset-0 z-10" 
                onClick={() => setShowSortMenu(false)}
              />
              <div className="absolute right-0 top-full mt-2 w-48 bg-slate-800 border border-slate-700 rounded-lg shadow-xl z-20 py-1 overflow-hidden">
                {sortOptions.map((option) => {
                  const Icon = option.icon;
                  return (
                    <button
                      key={option.value}
                      onClick={() => {
                        setSortBy(option.value);
                        setShowSortMenu(false);
                      }}
                      className={`w-full flex items-center gap-2 px-4 py-2 text-sm transition-colors ${
                        sortBy === option.value
                          ? 'bg-blue-600 text-white'
                          : 'text-slate-300 hover:bg-slate-700'
                      }`}
                    >
                      <Icon className="w-4 h-4" />
                      {option.label}
                    </button>
                  );
                })}
              </div>
            </>
          )}
        </div>
      </div>

      {/* Media Widgets */}
      {showWidgets && !searchTerm && (
        <div className="space-y-6 mb-8">
          {/* Recently Added */}
          {recentlyAdded.length > 0 && (
            <MediaWidget
              title="Recently Added"
              icon={<Clock className="w-5 h-5 text-blue-400" />}
              items={recentlyAdded}
              onRemove={handleRemove}
              deletingId={deletingId}
            />
          )}

          {/* Top Rated in Library */}
          {topRated.length > 0 && (
            <MediaWidget
              title="Top Rated"
              icon={<Star className="w-5 h-5 text-yellow-400" />}
              items={topRated}
              onRemove={handleRemove}
              deletingId={deletingId}
            />
          )}

          {/* Upcoming Movies */}
          {upcomingMovies.length > 0 && (
            <CalendarWidget
              title="Upcoming Movies"
              icon={<Film className="w-5 h-5 text-purple-400" />}
              entries={upcomingMovies}
            />
          )}

          {/* Upcoming Episodes */}
          {upcomingEpisodes.length > 0 && (
            <CalendarWidget
              title="Upcoming Episodes"
              icon={<Tv className="w-5 h-5 text-green-400" />}
              entries={upcomingEpisodes}
            />
          )}
        </div>
      )}

      {/* Results count */}
      <p className="text-slate-400 text-sm mb-4">
        Showing {filteredMedia.length} items
      </p>

      {/* Content */}
      {isLoading ? (
        <div className="flex items-center justify-center h-64">
          <div className="text-slate-400">Loading library...</div>
        </div>
      ) : filteredMedia.length === 0 ? (
        <div className="text-center py-16">
          <div className="text-slate-400 text-lg mb-2">No items found</div>
          <p className="text-slate-500">
            {searchTerm ? 'Try adjusting your search' : 'Your library is empty'}
          </p>
        </div>
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-4">
          {filteredMedia.map((item) => (
            <MediaCard 
              key={`${item.type}-${item.id}`} 
              item={item} 
              onRemove={handleRemove}
              isDeleting={deletingId === `${item.type}-${item.id}`}
            />
          ))}
        </div>
      ) : (
        <div className="space-y-2">
          {filteredMedia.map((item) => (
            <MediaListItem 
              key={`${item.type}-${item.id}`} 
              item={item}
              onRemove={handleRemove}
              isDeleting={deletingId === `${item.type}-${item.id}`}
            />
          ))}
        </div>
      )}
    </div>
  );
}

// Grid Card Component
function MediaCard({ item, onRemove, isDeleting }: { item: MediaItem; onRemove: (item: MediaItem) => void; isDeleting: boolean }) {
  return (
    <div className="group relative bg-slate-800 rounded-lg overflow-hidden hover:ring-2 hover:ring-blue-500 transition-all">
      <div className="aspect-[2/3] relative">
        <img
          src={tmdbImageUrl(item.poster_path, 'w500')}
          alt={item.title}
          className="w-full h-full object-cover"
        />
        
        {/* Type badge */}
        <div className={`absolute top-2 left-2 px-1.5 py-0.5 rounded text-xs font-medium ${
          item.type === 'movie' 
            ? 'bg-purple-600 text-white' 
            : 'bg-green-600 text-white'
        }`}>
          {item.type === 'movie' ? 'Movie' : 'Series'}
        </div>

        {/* Remove button - shows on hover */}
        <button
          onClick={(e) => { e.stopPropagation(); onRemove(item); }}
          disabled={isDeleting}
          className="absolute top-2 right-2 p-1.5 rounded-full bg-red-600 text-white opacity-0 group-hover:opacity-100 hover:bg-red-700 transition-all disabled:opacity-50"
          title="Remove from library"
        >
          {isDeleting ? (
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
          ) : (
            <Trash2 className="w-3.5 h-3.5" />
          )}
        </button>

        {/* Monitored indicator */}
        {!item.monitored && (
          <div className="absolute top-10 right-2 w-3 h-3 rounded-full bg-yellow-500" title="Unmonitored" />
        )}

        {/* Hover overlay */}
        <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none">
          <div className="absolute bottom-0 left-0 right-0 p-3">
            {item.vote_average && item.vote_average > 0 && (
              <div className="text-yellow-400 text-xs mb-1">
                ★ {item.vote_average.toFixed(1)}
              </div>
            )}
          </div>
        </div>
      </div>
      
      <div className="p-2">
        <h3 className="text-white text-sm font-medium truncate" title={item.title}>
          {item.title}
        </h3>
        {item.year && (
          <p className="text-slate-400 text-xs">{item.year}</p>
        )}
      </div>
    </div>
  );
}

// List Item Component
function MediaListItem({ item, onRemove, isDeleting }: { item: MediaItem; onRemove: (item: MediaItem) => void; isDeleting: boolean }) {
  return (
    <div className="flex gap-4 p-3 bg-slate-800 rounded-lg hover:bg-slate-700 transition-colors">
      <img
        src={tmdbImageUrl(item.poster_path, 'w200')}
        alt={item.title}
        className="w-16 h-24 object-cover rounded flex-shrink-0"
      />
      
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className={`px-1.5 py-0.5 rounded text-xs font-medium ${
            item.type === 'movie' 
              ? 'bg-purple-600 text-white' 
              : 'bg-green-600 text-white'
          }`}>
            {item.type === 'movie' ? 'Movie' : 'Series'}
          </span>
          {!item.monitored && (
            <span className="px-1.5 py-0.5 rounded text-xs bg-yellow-600 text-white">
              Unmonitored
            </span>
          )}
        </div>
        
        <h3 className="text-white font-medium truncate">{item.title}</h3>
        
        <div className="flex items-center gap-3 text-sm text-slate-400 mt-1">
          {item.year && <span>{item.year}</span>}
          {item.vote_average && item.vote_average > 0 && (
            <span className="text-yellow-400">★ {item.vote_average.toFixed(1)}</span>
          )}
        </div>
        
        {item.overview && (
          <p className="text-slate-400 text-sm mt-2 line-clamp-2">{item.overview}</p>
        )}
      </div>

      {/* Remove button */}
      <button
        onClick={() => onRemove(item)}
        disabled={isDeleting}
        className="self-center p-2 rounded-lg bg-slate-700 hover:bg-red-600 text-slate-400 hover:text-white transition-colors disabled:opacity-50"
        title="Remove from library"
      >
        {isDeleting ? (
          <Loader2 className="w-5 h-5 animate-spin" />
        ) : (
          <Trash2 className="w-5 h-5" />
        )}
      </button>
    </div>
  );
}

// Horizontal scrollable media widget
function MediaWidget({ title, icon, items, onRemove, deletingId }: { 
  title: string; 
  icon: React.ReactNode; 
  items: MediaItem[];
  onRemove: (item: MediaItem) => void;
  deletingId: string | null;
}) {
  const scrollContainer = (id: string, direction: 'left' | 'right') => {
    const container = document.getElementById(id);
    if (container) {
      const scrollAmount = direction === 'left' ? -300 : 300;
      container.scrollBy({ left: scrollAmount, behavior: 'smooth' });
    }
  };

  const widgetId = `widget-${title.replace(/\s+/g, '-').toLowerCase()}`;

  return (
    <div className="bg-slate-800/50 rounded-xl p-4">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          {icon}
          <h3 className="text-lg font-semibold text-white">{title}</h3>
          <span className="text-slate-400 text-sm">({items.length})</span>
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
        {items.map((item) => {
          const isDeleting = deletingId === `${item.type}-${item.id}`;
          return (
            <div
              key={`${item.type}-${item.id}`}
              className="flex-shrink-0 w-32 group"
            >
              <div className="relative aspect-[2/3] rounded-lg overflow-hidden mb-2">
                <img
                  src={tmdbImageUrl(item.poster_path, 'w200')}
                  alt={item.title}
                  className="w-full h-full object-cover group-hover:scale-105 transition-transform"
                />
                <div className={`absolute top-1 left-1 px-1 py-0.5 rounded text-[10px] font-medium ${
                  item.type === 'movie' ? 'bg-purple-600' : 'bg-green-600'
                } text-white`}>
                  {item.type === 'movie' ? 'Movie' : 'Series'}
                </div>
                
                {/* Remove button - shows on hover */}
                <button
                  onClick={(e) => { e.stopPropagation(); onRemove(item); }}
                  disabled={isDeleting}
                  className="absolute top-1 right-1 p-1 rounded-full bg-red-600 text-white opacity-0 group-hover:opacity-100 hover:bg-red-700 transition-all disabled:opacity-50"
                  title="Remove from library"
                >
                  {isDeleting ? (
                    <Loader2 className="w-3 h-3 animate-spin" />
                  ) : (
                    <Trash2 className="w-3 h-3" />
                  )}
                </button>
                
                {item.vote_average && item.vote_average > 0 && (
                  <div className="absolute bottom-1 right-1 px-1 py-0.5 rounded bg-black/70 text-yellow-400 text-[10px] font-medium">
                    ★ {item.vote_average.toFixed(1)}
                  </div>
                )}
              </div>
              <p className="text-white text-xs font-medium truncate" title={item.title}>
                {item.title}
              </p>
              {item.year && (
                <p className="text-slate-400 text-[10px]">{item.year}</p>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// Calendar widget for upcoming releases - styled same as MediaWidget
function CalendarWidget({ title, icon, entries }: { title: string; icon: React.ReactNode; entries: CalendarEntry[] }) {
  const scrollContainer = (id: string, direction: 'left' | 'right') => {
    const container = document.getElementById(id);
    if (container) {
      const scrollAmount = direction === 'left' ? -300 : 300;
      container.scrollBy({ left: scrollAmount, behavior: 'smooth' });
    }
  };

  const widgetId = `calendar-${title.replace(/\s+/g, '-').toLowerCase()}`;

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
              {/* Type badge - same style as MediaWidget */}
              <div className={`absolute top-1 left-1 px-1 py-0.5 rounded text-[10px] font-medium ${
                entry.type === 'movie' ? 'bg-purple-600' : 'bg-green-600'
              } text-white`}>
                {entry.type === 'movie' ? 'Movie' : 'Series'}
              </div>
              {/* Release date badge */}
              <div className="absolute top-1 right-1 px-1 py-0.5 rounded bg-blue-600/90 text-white text-[10px] font-medium">
                {formatDate(entry.date)}
              </div>
              {/* Rating badge - same style as MediaWidget */}
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
