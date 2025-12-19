import { useState, useMemo, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import { 
  Play, Info, ChevronLeft, ChevronRight, X, Plus, 
  Tv, Film, Loader2, ChevronDown, Search
} from 'lucide-react';
import type { Movie, Series, Episode } from '../types';

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
  metadata?: Record<string, any>;
  imdb_id?: string;
  collection_id?: number;
};

// Netflix-style Hero Banner
function HeroBanner({ item, onPlay, onMoreInfo }: { 
  item: MediaItem | null; 
  onPlay: () => void;
  onMoreInfo: () => void;
}) {
  if (!item) return null;

  return (
    <div className="relative h-[80vh] min-h-[500px] max-h-[800px]">
      {/* Background Image */}
      <div className="absolute inset-0">
        {item.backdrop_path ? (
          <img
            src={tmdbImageUrl(item.backdrop_path, 'original')}
            alt={item.title}
            className="w-full h-full object-cover"
          />
        ) : item.poster_path ? (
          <img
            src={tmdbImageUrl(item.poster_path, 'original')}
            alt={item.title}
            className="w-full h-full object-cover object-top blur-sm"
          />
        ) : (
          <div className="w-full h-full bg-gradient-to-br from-slate-800 to-slate-900" />
        )}
        {/* Gradients */}
        <div className="absolute inset-0 bg-gradient-to-r from-[#141414] via-[#141414]/60 to-transparent" />
        <div className="absolute inset-0 bg-gradient-to-t from-[#141414] via-transparent to-[#141414]/30" />
      </div>

      {/* Content */}
      <div className="absolute bottom-[15%] left-12 right-12 max-w-2xl">
        <div className="flex items-center gap-3 mb-4">
          <span className={`px-3 py-1 rounded text-sm font-bold ${
            item.type === 'movie' ? 'bg-purple-600' : 'bg-green-600'
          } text-white`}>
            {item.type === 'movie' ? 'MOVIE' : 'SERIES'}
          </span>
          {item.vote_average && item.vote_average > 0 && (
            <span className="flex items-center gap-1 text-green-400 font-semibold">
              {(item.vote_average * 10).toFixed(0)}% Match
            </span>
          )}
          {item.year && <span className="text-slate-300 font-medium">{item.year}</span>}
        </div>

        <h1 className="text-4xl md:text-6xl font-black text-white mb-4 drop-shadow-2xl leading-tight">
          {item.title}
        </h1>

        {item.overview && (
          <p className="text-base md:text-lg text-slate-200 mb-6 line-clamp-3 leading-relaxed">
            {item.overview}
          </p>
        )}

        <div className="flex items-center gap-3">
          <button
            onClick={onPlay}
            className="flex items-center gap-2 px-6 md:px-8 py-2.5 md:py-3 bg-white text-black font-bold rounded 
                       hover:bg-white/80 transition-colors text-base md:text-lg"
          >
            <Play className="w-5 h-5 md:w-6 md:h-6 fill-black" />
            Play
          </button>
          <button
            onClick={onMoreInfo}
            className="flex items-center gap-2 px-6 md:px-8 py-2.5 md:py-3 bg-gray-500/70 text-white font-bold rounded 
                       hover:bg-gray-500/50 transition-colors text-base md:text-lg backdrop-blur-sm"
          >
            <Info className="w-5 h-5 md:w-6 md:h-6" />
            More Info
          </button>
        </div>
      </div>
    </div>
  );
}

// Netflix-style Detail Modal
function DetailModal({ 
  media, 
  onClose 
}: { 
  media: MediaItem; 
  onClose: () => void;
}) {
  const [selectedSeason, setSelectedSeason] = useState(1);
  const [seriesImdbId, setSeriesImdbId] = useState<string | null>(
    media.imdb_id || media.metadata?.imdb_id as string || null
  );

  // Fetch episodes for series
  const { data: episodes = [], isLoading: episodesLoading } = useQuery<Episode[]>({
    queryKey: ['episodes', media.id],
    queryFn: async () => {
      if (media.type === 'series') {
        const response = await streamarrApi.getEpisodes(media.id);
        return Array.isArray(response.data) ? response.data : [];
      }
      return [];
    },
    enabled: media.type === 'series',
  });

  // Extract IMDB ID from episode metadata
  useEffect(() => {
    if (episodes.length > 0 && !seriesImdbId) {
      const firstEpisode = episodes[0];
      if (firstEpisode.metadata?.series_imdb_id) {
        setSeriesImdbId(firstEpisode.metadata.series_imdb_id as string);
      }
    }
  }, [episodes, seriesImdbId]);

  // Group episodes by season
  const episodesBySeason = useMemo(() => {
    const grouped: Record<number, Episode[]> = {};
    episodes.forEach(ep => {
      if (!grouped[ep.season_number]) {
        grouped[ep.season_number] = [];
      }
      grouped[ep.season_number].push(ep);
    });
    Object.values(grouped).forEach(eps => eps.sort((a, b) => a.episode_number - b.episode_number));
    return grouped;
  }, [episodes]);

  const seasons = Object.keys(episodesBySeason).map(Number).sort((a, b) => a - b);

  // Fetch streams for movies
  const { data: streams = [], isLoading: streamsLoading } = useQuery({
    queryKey: ['streams', media.type, media.id],
    queryFn: async () => {
      if (media.type === 'movie') {
        const response = await streamarrApi.getMovieStreams(media.id);
        return Array.isArray(response.data) ? response.data : [];
      }
      return [];
    },
    enabled: media.type === 'movie',
  });

  // Close on escape
  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleEsc);
    return () => window.removeEventListener('keydown', handleEsc);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto" onClick={onClose}>
      <div className="min-h-screen bg-black/80 backdrop-blur-sm flex items-start justify-center py-8 px-4">
        <div 
          className="relative w-full max-w-5xl bg-[#181818] rounded-xl overflow-hidden shadow-2xl"
          onClick={(e) => e.stopPropagation()}
        >
          {/* Close button */}
          <button
            onClick={onClose}
            className="absolute top-4 right-4 z-30 p-2 rounded-full bg-[#181818] hover:bg-[#282828] transition-colors"
          >
            <X className="w-6 h-6 text-white" />
          </button>

          {/* Hero section */}
          <div className="relative aspect-video max-h-[60vh]">
            {media.backdrop_path ? (
              <img
                src={tmdbImageUrl(media.backdrop_path, 'w1280')}
                alt={media.title}
                className="w-full h-full object-cover"
              />
            ) : media.poster_path ? (
              <img
                src={tmdbImageUrl(media.poster_path, 'w780')}
                alt={media.title}
                className="w-full h-full object-cover"
              />
            ) : (
              <div className="w-full h-full bg-gradient-to-br from-slate-700 to-slate-900" />
            )}
            <div className="absolute inset-0 bg-gradient-to-t from-[#181818] via-[#181818]/20 to-transparent" />
            
            {/* Title and buttons */}
            <div className="absolute bottom-6 left-8 right-8">
              <h1 className="text-3xl md:text-5xl font-black text-white mb-4 drop-shadow-lg">{media.title}</h1>
              <div className="flex items-center gap-3 flex-wrap">
                <button className="flex items-center gap-2 px-6 py-2 bg-white text-black font-bold rounded hover:bg-white/80 transition-colors">
                  <Play className="w-5 h-5 fill-black" />
                  Play
                </button>
                <button className="p-2 rounded-full border-2 border-gray-400 hover:border-white transition-colors" title="Add to My List">
                  <Plus className="w-5 h-5 text-white" />
                </button>
              </div>
            </div>
          </div>

          {/* Content section */}
          <div className="p-6 md:p-8">
            {/* Meta info */}
            <div className="flex flex-wrap items-center gap-3 mb-4">
              {media.vote_average && media.vote_average > 0 && (
                <span className="text-green-400 font-bold text-lg">
                  {(media.vote_average * 10).toFixed(0)}% Match
                </span>
              )}
              {media.year && <span className="text-slate-400 text-lg">{media.year}</span>}
              <span className={`px-2 py-1 rounded text-xs font-bold ${
                media.type === 'movie' ? 'bg-purple-600' : 'bg-green-600'
              } text-white`}>
                {media.type === 'movie' ? 'MOVIE' : 'SERIES'}
              </span>
            </div>

            {/* Overview */}
            {media.overview && (
              <p className="text-slate-300 text-base md:text-lg mb-8 leading-relaxed max-w-4xl">
                {media.overview}
              </p>
            )}

            {/* Series Episodes */}
            {media.type === 'series' && (
              <div className="mb-8">
                <div className="flex items-center justify-between mb-6 flex-wrap gap-4">
                  <h2 className="text-2xl font-bold text-white">Episodes</h2>
                  {seasons.length > 0 && (
                    <div className="relative">
                      <select
                        value={selectedSeason}
                        onChange={(e) => setSelectedSeason(Number(e.target.value))}
                        className="appearance-none bg-[#242424] text-white px-4 py-2 pr-10 rounded border border-gray-600 
                                   hover:border-gray-400 transition-colors cursor-pointer font-medium"
                      >
                        {seasons.map(season => (
                          <option key={season} value={season}>Season {season}</option>
                        ))}
                      </select>
                      <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 pointer-events-none" />
                    </div>
                  )}
                </div>

                {episodesLoading ? (
                  <div className="flex items-center justify-center py-16">
                    <Loader2 className="w-10 h-10 animate-spin text-red-600" />
                  </div>
                ) : episodes.length === 0 ? (
                  <div className="text-center py-16 text-slate-400">
                    <Tv className="w-16 h-16 mx-auto mb-4 text-slate-600" />
                    <p className="text-lg">No episodes found</p>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {(episodesBySeason[selectedSeason] || []).map((episode) => (
                      <EpisodeCard 
                        key={episode.id} 
                        episode={episode} 
                        seriesImdbId={seriesImdbId}
                      />
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* Movie Streams */}
            {media.type === 'movie' && (
              <div>
                <h2 className="text-2xl font-bold text-white mb-6 flex items-center gap-2">
                  Available Streams
                  {streams.length > 0 && (
                    <span className="text-base font-normal text-slate-400">
                      ({streams.length})
                    </span>
                  )}
                </h2>

                {streamsLoading ? (
                  <div className="flex items-center justify-center py-16">
                    <Loader2 className="w-10 h-10 animate-spin text-red-600" />
                  </div>
                ) : streams.length === 0 ? (
                  <div className="text-center py-16 text-slate-400">
                    <Film className="w-16 h-16 mx-auto mb-4 text-slate-600" />
                    <p className="text-lg">No streams found</p>
                    <p className="text-sm mt-2">Check your addon configuration in Settings</p>
                  </div>
                ) : (
                  <div className="grid gap-2">
                    {streams.map((stream: any, index: number) => (
                      <StreamCard key={index} stream={stream} />
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// Episode Card Component (Netflix style)
function EpisodeCard({ episode, seriesImdbId }: { episode: Episode; seriesImdbId: string | null }) {
  const [showStreams, setShowStreams] = useState(false);

  const { data: streams = [], isLoading: streamsLoading, refetch } = useQuery({
    queryKey: ['episode-streams', seriesImdbId, episode.season_number, episode.episode_number],
    queryFn: async () => {
      if (seriesImdbId) {
        const response = await streamarrApi.getEpisodeStreams(
          seriesImdbId, 
          episode.season_number, 
          episode.episode_number
        );
        return Array.isArray(response.data) ? response.data : [];
      }
      return [];
    },
    enabled: showStreams && !!seriesImdbId,
  });

  const handleToggle = () => {
    const newState = !showStreams;
    setShowStreams(newState);
    if (newState && seriesImdbId) {
      refetch();
    }
  };

  return (
    <div className="bg-[#242424] rounded-lg overflow-hidden">
      <div 
        className="flex gap-4 p-4 cursor-pointer hover:bg-[#2f2f2f] transition-colors"
        onClick={handleToggle}
      >
        {/* Episode thumbnail */}
        <div className="relative w-36 md:w-44 aspect-video flex-shrink-0 rounded overflow-hidden bg-slate-800 group">
          {episode.still_path ? (
            <img
              src={tmdbImageUrl(episode.still_path, 'w300')}
              alt={episode.title}
              className="w-full h-full object-cover"
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center bg-gradient-to-br from-slate-700 to-slate-800">
              <Tv className="w-8 h-8 text-slate-600" />
            </div>
          )}
          {/* Play overlay */}
          <div className="absolute inset-0 flex items-center justify-center bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity">
            <div className="p-3 rounded-full bg-white/20 backdrop-blur-sm border border-white/30">
              <Play className="w-6 h-6 text-white fill-white" />
            </div>
          </div>
        </div>

        {/* Episode info */}
        <div className="flex-1 min-w-0 py-1">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <h3 className="text-white font-semibold text-base md:text-lg mb-1">
                <span className="text-slate-400">{episode.episode_number}.</span> {episode.title}
              </h3>
              {episode.air_date && (
                <p className="text-sm text-slate-500 mb-2">
                  {new Date(episode.air_date).toLocaleDateString('en-US', { 
                    year: 'numeric', month: 'short', day: 'numeric' 
                  })}
                </p>
              )}
            </div>
            <ChevronDown className={`w-5 h-5 text-slate-400 flex-shrink-0 transition-transform duration-200 ${
              showStreams ? 'rotate-180' : ''
            }`} />
          </div>
          <p className="text-sm text-slate-400 line-clamp-2 leading-relaxed">
            {episode.overview || 'No description available'}
          </p>
        </div>
      </div>

      {/* Streams section */}
      {showStreams && (
        <div className="border-t border-slate-700/50 px-4 py-4 bg-[#1c1c1c]">
          {!seriesImdbId ? (
            <p className="text-slate-400 text-center py-6">Unable to fetch streams - IMDB ID not available</p>
          ) : streamsLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="w-8 h-8 animate-spin text-red-600" />
            </div>
          ) : streams.length === 0 ? (
            <p className="text-slate-400 text-center py-6">No streams found for this episode</p>
          ) : (
            <div className="space-y-2">
              <p className="text-sm text-slate-500 mb-3 font-medium">{streams.length} streams available</p>
              {streams.slice(0, 15).map((stream: any, index: number) => (
                <StreamCard key={index} stream={stream} compact />
              ))}
              {streams.length > 15 && (
                <p className="text-sm text-slate-500 text-center pt-3">
                  + {streams.length - 15} more streams available
                </p>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Stream Card Component (Netflix style)
function StreamCard({ stream, compact = false }: { stream: any; compact?: boolean }) {
  const [showFullName, setShowFullName] = useState(false);
  
  const getQualityColor = (quality: string) => {
    if (quality?.includes('2160') || quality?.includes('4K')) return 'bg-purple-600';
    if (quality?.includes('1080')) return 'bg-blue-600';
    if (quality?.includes('720')) return 'bg-green-600';
    return 'bg-slate-600';
  };

  const getFileName = () => {
    // Prioritize filename/title that looks like an actual file
    if (stream.filename) return stream.filename;
    if (stream.title && (stream.title.includes('.') || stream.title.length > 30)) return stream.title;
    if (stream.name && (stream.name.includes('.') || stream.name.length > 30)) return stream.name;
    if (stream.title) return stream.title;
    if (stream.name) return stream.name;
    return null;
  };

  const fileName = getFileName();

  return (
    <div className={`flex items-center gap-3 ${compact ? 'p-2.5' : 'p-3'} bg-[#2a2a2a] rounded-lg hover:bg-[#333] transition-colors group`}>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap mb-1.5">
          <span className={`px-2 py-0.5 rounded text-xs font-bold text-white ${getQualityColor(stream.quality)}`}>
            {stream.quality || 'Unknown'}
          </span>
          {stream.source && (
            <span className="px-2 py-0.5 rounded text-xs font-semibold bg-red-600 text-white">
              {stream.source}
            </span>
          )}
          {stream.codec && (
            <span className="px-2 py-0.5 rounded text-xs bg-slate-700 text-slate-300">
              {stream.codec.toUpperCase()}
            </span>
          )}
          {stream.cached && (
            <span className="px-2 py-0.5 rounded text-xs bg-green-600/80 text-white font-medium">⚡ Cached</span>
          )}
        </div>
        
        {fileName && (
          <button
            onClick={(e) => {
              e.stopPropagation();
              setShowFullName(!showFullName);
            }}
            className={`text-left text-slate-300 ${compact ? 'text-xs' : 'text-sm'} ${showFullName ? 'whitespace-normal break-all' : 'line-clamp-1'} hover:text-white transition-colors cursor-pointer`}
            title={showFullName ? "Click to collapse" : fileName}
          >
            {fileName}
          </button>
        )}
        
        <div className="flex items-center gap-3 mt-1.5 text-xs text-slate-500">
          {stream.size_gb && stream.size_gb > 0 && (
            <span className="flex items-center gap-1">
              <span>📁</span> {stream.size_gb.toFixed(1)} GB
            </span>
          )}
          {stream.seeds && stream.seeds > 0 && (
            <span className="text-green-500 flex items-center gap-1">
              <span>🌱</span> {stream.seeds} seeds
            </span>
          )}
          {stream.languages && stream.languages.length > 0 && (
            <span>🌐 {stream.languages.join(', ')}</span>
          )}
        </div>
      </div>

      <button
        onClick={(e) => {
          e.stopPropagation();
          if (stream.url) window.open(stream.url, '_blank');
        }}
        className="flex-shrink-0 p-2.5 rounded-full bg-white hover:bg-white/80 hover:scale-110 transition-all opacity-70 group-hover:opacity-100"
        title="Play stream"
      >
        <Play className="w-4 h-4 text-black fill-black" />
      </button>
    </div>
  );
}

// Main Library Component
export default function Library() {
  const [selectedMedia, setSelectedMedia] = useState<MediaItem | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [showSearch, setShowSearch] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();
  const [currentPage, setCurrentPage] = useState(1);
  const ITEMS_PER_PAGE = 50;

  // Get current view from URL params (default to 'all')
  const currentView = searchParams.get('view') || 'all';

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

  const isLoading = moviesLoading || seriesLoading;

  // Transform data to MediaItems
  const allMedia: MediaItem[] = useMemo(() => {
    const movieItems: MediaItem[] = movies.map((m: Movie) => ({
      id: m.id,
      tmdb_id: m.tmdb_id,
      title: m.title,
      poster_path: m.poster_path,
      backdrop_path: m.backdrop_path,
      year: m.year,
      type: 'movie' as const,
      monitored: m.monitored,
      overview: m.overview,
      vote_average: m.vote_average,
      added_at: m.added_at,
      release_date: m.release_date,
      metadata: m.metadata,
      imdb_id: m.imdb_id,
    }));

    const seriesItems: MediaItem[] = series.map((s: Series) => ({
      id: s.id,
      tmdb_id: s.tmdb_id,
      title: s.name || s.title,
      poster_path: s.poster_path,
      backdrop_path: s.backdrop_path,
      year: s.first_air_date ? new Date(s.first_air_date).getFullYear() : undefined,
      type: 'series' as const,
      monitored: s.monitored,
      overview: s.overview,
      vote_average: s.vote_average,
      added_at: s.added_at,
      metadata: s.metadata,
      imdb_id: s.imdb_id,
    }));

    return [...movieItems, ...seriesItems];
  }, [movies, series]);

  // Featured item (random from top rated with backdrop)
  const [featuredItem, setFeaturedItem] = useState<MediaItem | null>(null);
  
  useEffect(() => {
    const withBackdrop = allMedia.filter(m => m.backdrop_path);
    if (withBackdrop.length > 0) {
      const sorted = withBackdrop.sort((a, b) => (b.vote_average || 0) - (a.vote_average || 0));
      const top = sorted.slice(0, Math.min(10, sorted.length));
      setFeaturedItem(top[Math.floor(Math.random() * top.length)]);
    } else if (allMedia.length > 0) {
      setFeaturedItem(allMedia[0]);
    }
  }, [allMedia]);

  // Filtered media based on current view
  const filteredMedia = useMemo(() => {
    let filtered = [...allMedia];

    switch (currentView) {
      case 'recently-added-movies':
        filtered = filtered
          .filter(m => m.type === 'movie')
          .sort((a, b) => new Date(b.added_at || 0).getTime() - new Date(a.added_at || 0).getTime());
        break;
      case 'recently-added-series':
        filtered = filtered
          .filter(m => m.type === 'series')
          .sort((a, b) => new Date(b.added_at || 0).getTime() - new Date(a.added_at || 0).getTime());
        break;
      case 'top-rated':
        filtered = filtered
          .filter(m => m.vote_average && m.vote_average > 0)
          .sort((a, b) => (b.vote_average || 0) - (a.vote_average || 0));
        break;
      case 'movies':
        filtered = filtered.filter(m => m.type === 'movie');
        break;
      case 'series':
        filtered = filtered.filter(m => m.type === 'series');
        break;
      default:
        filtered = filtered.sort((a, b) => new Date(b.added_at || 0).getTime() - new Date(a.added_at || 0).getTime());
    }

    return filtered;
  }, [allMedia, currentView]);

  // Pagination
  const totalPages = Math.ceil(filteredMedia.length / ITEMS_PER_PAGE);
  const startIndex = (currentPage - 1) * ITEMS_PER_PAGE;
  const endIndex = startIndex + ITEMS_PER_PAGE;
  const currentItems = filteredMedia.slice(startIndex, endIndex);

  // Reset page when view changes
  useEffect(() => {
    setCurrentPage(1);
  }, [currentView]);

  // View titles
  const viewTitles: Record<string, string> = {
    'all': 'All Media',
    'recently-added-movies': 'Recently Added Movies',
    'recently-added-series': 'Recently Added Series',
    'top-rated': 'Top Rated',
    'movies': 'Movies',
    'series': 'TV Shows',
  };

  // Search results
  const searchResults = useMemo(() => {
    if (!searchTerm.trim()) return [];
    const term = searchTerm.toLowerCase();
    return allMedia.filter(m => m.title.toLowerCase().includes(term));
  }, [allMedia, searchTerm]);

  if (isLoading) {
    return (
      <div className="min-h-screen bg-[#141414] flex items-center justify-center">
        <Loader2 className="w-12 h-12 animate-spin text-red-600" />
      </div>
    );
  }

  if (allMedia.length === 0) {
    return (
      <div className="min-h-screen bg-[#141414] flex flex-col items-center justify-center text-center px-4">
        <Film className="w-24 h-24 text-slate-600 mb-6" />
        <h2 className="text-3xl font-bold text-white mb-3">Your Library is Empty</h2>
        <p className="text-slate-400 text-lg max-w-md">
          Add some movies and TV shows to get started. Use the Discovery page to find content.
        </p>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#141414] -m-6 -mt-6">
      {/* Search overlay */}
      {showSearch && (
        <div className="fixed inset-0 z-50 bg-black/95 pt-20 px-6 md:px-12 overflow-y-auto">
          <div className="max-w-5xl mx-auto">
            <div className="flex items-center gap-4 mb-8">
              <Search className="w-8 h-8 text-slate-400 flex-shrink-0" />
              <input
                type="text"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder="Search titles..."
                className="flex-1 bg-transparent text-2xl md:text-3xl text-white placeholder-slate-500 outline-none"
                autoFocus
              />
              <button 
                onClick={() => { setShowSearch(false); setSearchTerm(''); }}
                className="p-2 rounded-full hover:bg-slate-800 transition-colors"
              >
                <X className="w-8 h-8 text-white" />
              </button>
            </div>
            
            {searchResults.length > 0 && (
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
                {searchResults.map(item => (
                  <div
                    key={`${item.type}-${item.id}`}
                    className="cursor-pointer hover:scale-105 transition-transform"
                    onClick={() => { setSelectedMedia(item); setShowSearch(false); setSearchTerm(''); }}
                  >
                    <div className="aspect-[2/3] rounded-md overflow-hidden bg-slate-800">
                      {item.poster_path ? (
                        <img src={tmdbImageUrl(item.poster_path, 'w342')} alt={item.title} className="w-full h-full object-cover" />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center">
                          {item.type === 'movie' ? <Film className="w-8 h-8 text-slate-600" /> : <Tv className="w-8 h-8 text-slate-600" />}
                        </div>
                      )}
                    </div>
                    <p className="text-white text-sm mt-2 truncate font-medium">{item.title}</p>
                  </div>
                ))}
              </div>
            )}
            
            {searchTerm && searchResults.length === 0 && (
              <div className="text-center py-20">
                <Search className="w-16 h-16 text-slate-600 mx-auto mb-4" />
                <p className="text-slate-400 text-xl">No results found for "{searchTerm}"</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Search button (floating) */}
      <button
        onClick={() => setShowSearch(true)}
        className="fixed top-4 right-72 z-30 p-2.5 rounded-full bg-black/60 hover:bg-black/80 transition-colors backdrop-blur-sm"
        title="Search"
      >
        <Search className="w-5 h-5 text-white" />
      </button>

      {/* Hero Banner */}
      <HeroBanner 
        item={featuredItem}
        onPlay={() => featuredItem && setSelectedMedia(featuredItem)}
        onMoreInfo={() => featuredItem && setSelectedMedia(featuredItem)}
      />

      {/* Filter Tabs */}
      <div className="relative -mt-20 z-10 px-12 mb-6">
        <div className="flex gap-2 overflow-x-auto pb-2 scrollbar-hide">
          {[
            { key: 'all', label: 'All' },
            { key: 'recently-added-movies', label: 'Recent Movies' },
            { key: 'recently-added-series', label: 'Recent Series' },
            { key: 'top-rated', label: 'Top Rated' },
            { key: 'movies', label: 'Movies' },
            { key: 'series', label: 'TV Shows' },
          ].map((tab) => (
            <button
              key={tab.key}
              onClick={() => {
                setSearchParams({ view: tab.key });
                setCurrentPage(1);
              }}
              className={`px-4 py-2 rounded-full text-sm font-medium transition-colors whitespace-nowrap ${
                currentView === tab.key
                  ? 'bg-white text-black'
                  : 'bg-white/10 text-white hover:bg-white/20'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Content Grid */}
      <div className="relative z-10 px-12 pb-12">
        <div className="mb-4">
          <h2 className="text-2xl font-bold text-white mb-2">{viewTitles[currentView]}</h2>
          <p className="text-slate-400">
            Showing {startIndex + 1}-{Math.min(endIndex, filteredMedia.length)} of {filteredMedia.length} items
          </p>
        </div>

        {currentItems.length > 0 ? (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-4 mb-8">
              {currentItems.map((item) => (
                <div
                  key={`${item.type}-${item.id}`}
                  onClick={() => setSelectedMedia(item)}
                  className="group cursor-pointer"
                >
                  <div className="relative aspect-[2/3] rounded-lg overflow-hidden bg-slate-800 mb-2 
                                 group-hover:ring-2 ring-white transition-all">
                    {item.poster_path ? (
                      <img
                        src={tmdbImageUrl(item.poster_path, 'w342')}
                        alt={item.title}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <div className="w-full h-full flex items-center justify-center">
                        {item.type === 'movie' ? (
                          <Film className="w-12 h-12 text-slate-600" />
                        ) : (
                          <Tv className="w-12 h-12 text-slate-600" />
                        )}
                      </div>
                    )}
                    
                    {/* Type Badge */}
                    <div className="absolute top-2 left-2">
                      <span className={`px-2 py-1 rounded text-xs font-bold ${
                        item.type === 'movie' ? 'bg-purple-600' : 'bg-green-600'
                      } text-white`}>
                        {item.type === 'movie' ? 'MOVIE' : 'SERIES'}
                      </span>
                    </div>

                    {/* Rating */}
                    {item.vote_average && item.vote_average > 0 && (
                      <div className="absolute bottom-2 right-2 bg-black/80 px-2 py-1 rounded backdrop-blur-sm">
                        <span className="text-yellow-400 text-xs font-bold">
                          {item.vote_average.toFixed(1)}
                        </span>
                      </div>
                    )}
                  </div>

                  <h3 className="text-white text-sm font-medium line-clamp-2 group-hover:text-slate-300 transition-colors">
                    {item.title}
                  </h3>
                  {item.year && (
                    <p className="text-slate-400 text-xs">{item.year}</p>
                  )}
                </div>
              ))}
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex items-center justify-center gap-2">
                <button
                  onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                  className="p-2 rounded bg-slate-800 text-white disabled:opacity-50 disabled:cursor-not-allowed
                           hover:bg-slate-700 transition-colors"
                >
                  <ChevronLeft className="w-5 h-5" />
                </button>

                <div className="flex items-center gap-1">
                  {Array.from({ length: Math.min(7, totalPages) }, (_, i) => {
                    let pageNum;
                    if (totalPages <= 7) {
                      pageNum = i + 1;
                    } else if (currentPage <= 4) {
                      pageNum = i + 1;
                    } else if (currentPage >= totalPages - 3) {
                      pageNum = totalPages - 6 + i;
                    } else {
                      pageNum = currentPage - 3 + i;
                    }

                    return (
                      <button
                        key={pageNum}
                        onClick={() => setCurrentPage(pageNum)}
                        className={`px-3 py-1 rounded transition-colors ${
                          currentPage === pageNum
                            ? 'bg-red-600 text-white'
                            : 'bg-slate-800 text-slate-300 hover:bg-slate-700'
                        }`}
                      >
                        {pageNum}
                      </button>
                    );
                  })}
                </div>

                <button
                  onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                  disabled={currentPage === totalPages}
                  className="p-2 rounded bg-slate-800 text-white disabled:opacity-50 disabled:cursor-not-allowed
                           hover:bg-slate-700 transition-colors"
                >
                  <ChevronRight className="w-5 h-5" />
                </button>
              </div>
            )}
          </>
        ) : (
          <div className="text-center py-20">
            <p className="text-slate-400 text-xl">No items found</p>
          </div>
        )}
      </div>

      {/* Detail Modal */}
      {selectedMedia && (
        <DetailModal 
          media={selectedMedia} 
          onClose={() => setSelectedMedia(null)} 
        />
      )}
    </div>
  );
}
