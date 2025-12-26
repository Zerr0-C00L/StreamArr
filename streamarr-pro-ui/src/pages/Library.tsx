import { useState, useMemo, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import { 
  ChevronLeft, ChevronRight, ArrowLeft, X,
  Tv, Film, Loader2, ChevronDown, Search, Trash2, Star, Calendar,
  CheckSquare, Square, XCircle, Library as LibraryIcon, RefreshCw
} from 'lucide-react';
import type { Movie, Series, Episode, Collection } from '../types';

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
  const [showRemoveConfirm, setShowRemoveConfirm] = useState(false);
  const [removing, setRemoving] = useState(false);

  const handleRemoveAndBlacklist = async () => {
    setRemoving(true);
    try {
      await streamarrApi.removeAndBlacklist(media.type, media.id, 'Removed by user');
      onClose();
      window.location.reload(); // Refresh to update the library
    } catch (error) {
      console.error('Failed to remove item:', error);
      alert('Failed to remove item. Please try again.');
    } finally {
      setRemoving(false);
      setShowRemoveConfirm(false);
    }
  };

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

  // Fetch settings to control stream card display behavior
  const { data: settingsData } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => {
      const res = await streamarrApi.getSettings();
      return res.data as any;
    },
  });
  const showFullStreamNames = !!settingsData?.show_full_stream_name;

  // Close on escape
  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleEsc);
    return () => window.removeEventListener('keydown', handleEsc);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 bg-[#141414] overflow-y-auto">
      {/* Full-screen hero section */}
      <div className="relative min-h-[85vh] w-full">
        {/* Background image */}
        <div className="absolute inset-0">
          {media.backdrop_path ? (
            <img
              src={tmdbImageUrl(media.backdrop_path, 'original')}
              alt={media.title}
              className="w-full h-full object-cover"
            />
          ) : media.poster_path ? (
            <img
              src={tmdbImageUrl(media.poster_path, 'original')}
              alt={media.title}
              className="w-full h-full object-cover object-top"
            />
          ) : (
            <div className="w-full h-full bg-gradient-to-br from-slate-800 to-slate-950" />
          )}
          {/* Gradient overlays */}
          <div className="absolute inset-0 bg-gradient-to-r from-[#141414] via-[#141414]/60 to-transparent" />
          <div className="absolute inset-0 bg-gradient-to-t from-[#141414] via-transparent to-[#141414]/30" />
          <div className="absolute bottom-0 left-0 right-0 h-64 bg-gradient-to-t from-[#141414] to-transparent" />
        </div>

        {/* Back button */}
        <button
          onClick={onClose}
          className="absolute top-6 left-6 z-30 flex items-center gap-2 px-4 py-2 rounded-full bg-black/50 hover:bg-black/70 backdrop-blur-sm transition-all group"
        >
          <ArrowLeft className="w-5 h-5 text-white group-hover:scale-110 transition-transform" />
          <span className="text-white font-medium">Back</span>
        </button>

        {/* Content info - positioned at bottom left */}
        <div className="absolute bottom-16 left-0 right-0 px-8 md:px-16 lg:px-20">
          <div className="max-w-3xl">
            {/* Type badge */}
            <div className="flex items-center gap-3 mb-4">
              <span className={`px-3 py-1 rounded-md text-sm font-bold uppercase tracking-wide ${
                media.type === 'movie' ? 'bg-purple-600' : 'bg-green-600'
              } text-white`}>
                {media.type === 'movie' ? 'Movie' : 'Series'}
              </span>
            </div>

            {/* Title */}
            <h1 className="text-4xl md:text-6xl lg:text-7xl font-black text-white mb-6 drop-shadow-2xl leading-tight">
              {media.title}
            </h1>

            {/* Meta info row */}
            <div className="flex flex-wrap items-center gap-4 mb-6">
              {media.vote_average && media.vote_average > 0 && (
                <div className="flex items-center gap-1.5">
                  <Star className="w-5 h-5 text-yellow-400 fill-yellow-400" />
                  <span className="text-white font-bold text-lg">
                    {media.vote_average.toFixed(1)}
                  </span>
                </div>
              )}
              {media.year && (
                <div className="flex items-center gap-1.5">
                  <Calendar className="w-5 h-5 text-slate-400" />
                  <span className="text-slate-300 text-lg">{media.year}</span>
                </div>
              )}
              {media.type === 'series' && seasons.length > 0 && (
                <div className="flex items-center gap-1.5">
                  <Tv className="w-5 h-5 text-slate-400" />
                  <span className="text-slate-300 text-lg">{seasons.length} Season{seasons.length !== 1 ? 's' : ''}</span>
                </div>
              )}
            </div>

            {/* Overview */}
            {media.overview && (
              <p className="text-slate-200 text-lg md:text-xl leading-relaxed mb-8 line-clamp-4">
                {media.overview}
              </p>
            )}

            {/* Action buttons */}
            <div className="flex items-center gap-4">
              <button 
                onClick={() => setShowRemoveConfirm(true)}
                className="flex items-center gap-2 px-6 py-3 bg-red-600 hover:bg-red-500 text-white font-semibold rounded-lg transition-all hover:scale-105" 
              >
                <Trash2 className="w-5 h-5" />
                Remove from Library
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Content sections - below the hero */}
      <div className="relative z-10 px-8 md:px-16 lg:px-20 pb-20 -mt-8 bg-[#141414]">

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
                      <StreamCard 
                        key={index} 
                        stream={stream} 
                        forceFullName={showFullStreamNames}
                      />
                    ))}
                  </div>
                )}
              </div>
            )}
      </div>

      {/* Remove Confirmation Dialog */}
      {showRemoveConfirm && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-[60]" onClick={() => setShowRemoveConfirm(false)}>
          <div className="bg-[#242424] rounded-lg p-6 max-w-md mx-4" onClick={(e) => e.stopPropagation()}>
            <h3 className="text-xl font-bold text-white mb-2">Remove from Library?</h3>
            <p className="text-slate-300 mb-4">
              This will permanently remove "{media.title}" from your library and add it to the blacklist to prevent re-importing.
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setShowRemoveConfirm(false)}
                disabled={removing}
                className="px-4 py-2 bg-slate-600 hover:bg-slate-500 text-white rounded transition-colors disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={handleRemoveAndBlacklist}
                disabled={removing}
                className="px-4 py-2 bg-red-600 hover:bg-red-500 text-white rounded transition-colors disabled:opacity-50 flex items-center gap-2"
              >
                {removing ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Removing...
                  </>
                ) : (
                  <>
                    <Trash2 className="w-4 h-4" />
                    Remove & Blacklist
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
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

  // Fetch settings to control stream card display behavior
  const { data: settingsData } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => {
      const res = await streamarrApi.getSettings();
      return res.data as any;
    },
  });
  const showFullStreamNames = !!settingsData?.show_full_stream_name;

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
                <StreamCard 
                  key={index} 
                  stream={stream} 
                  compact 
                  forceFullName={showFullStreamNames}
                />
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
function StreamCard({ stream, compact = false, forceFullName = false }: { stream: any; compact?: boolean; forceFullName?: boolean }) {
  const [showFullName, setShowFullName] = useState(forceFullName || false);
  useEffect(() => {
    setShowFullName(forceFullName || false);
  }, [forceFullName]);
  
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
            className={`text-left text-slate-300 ${compact ? 'text-xs' : 'text-sm'} ${showFullName ? 'whitespace-normal break-words w-full' : 'line-clamp-1'} hover:text-white transition-colors cursor-pointer block`}
            title={showFullName ? "Click to collapse" : fileName}
          >
            {fileName}
          </button>
        )}
        
        <div className="flex items-center gap-3 mt-1.5 text-xs text-slate-500">
          {(() => {
            const sizeBytes = stream.size || stream.behaviorHints?.videoSize;
            const sizeGB = typeof stream.size_gb === 'number' ? stream.size_gb : (sizeBytes ? (sizeBytes / (1024 * 1024 * 1024)) : undefined);
            return sizeGB && sizeGB > 0 ? (
              <span className="flex items-center gap-1">
                <span>📁</span> {sizeGB.toFixed(2)} GB
              </span>
            ) : null;
          })()}
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
    </div>
  );
}

// Collection Card Component
function CollectionCard({ 
  collection, 
  onClick 
}: { 
  collection: Collection; 
  onClick: () => void;
}) {
  const progress = collection.total_movies 
    ? Math.round((collection.movies_in_library || 0) / collection.total_movies * 100) 
    : 0;
  
  return (
    <div 
      onClick={onClick}
      className="group cursor-pointer"
    >
      <div className="relative aspect-[2/3] rounded-lg overflow-hidden bg-slate-800 mb-2 
                     group-hover:ring-2 ring-white transition-all">
        {collection.poster_path ? (
          <img
            src={tmdbImageUrl(collection.poster_path, 'w342')}
            alt={collection.name}
            className="w-full h-full object-cover"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center bg-gradient-to-br from-slate-700 to-slate-800">
            <LibraryIcon className="w-12 h-12 text-slate-600" />
          </div>
        )}
        
        {/* Collection Badge */}
        <div className="absolute top-2 left-2">
          <span className="px-2 py-1 rounded text-xs font-bold bg-cyan-600 text-white">
            COLLECTION
          </span>
        </div>

        {/* Progress indicator */}
        <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent p-3">
          <div className="flex items-center justify-between text-xs text-white mb-1">
            <span>{collection.movies_in_library || 0}/{collection.total_movies || 0}</span>
            <span>{progress}%</span>
          </div>
          <div className="w-full bg-slate-700 rounded-full h-1.5">
            <div 
              className={`h-1.5 rounded-full transition-all ${
                progress === 100 ? 'bg-green-500' : 'bg-cyan-500'
              }`}
              style={{ width: `${progress}%` }}
            />
          </div>
        </div>
      </div>

      <h3 className="text-white text-sm font-medium line-clamp-2 group-hover:text-slate-300 transition-colors">
        {collection.name}
      </h3>
    </div>
  );
}

// Collection Detail Modal
function CollectionDetailModal({ 
  collection, 
  onClose,
  onMovieClick 
}: { 
  collection: Collection; 
  onClose: () => void;
  onMovieClick?: (movie: any) => void;
}) {
  const [syncing, setSyncing] = useState(false);

  // Fetch collection details with movies
  const { data: collectionData, isLoading, refetch } = useQuery({
    queryKey: ['collection', collection.id],
    queryFn: async () => {
      const response = await streamarrApi.getCollection(collection.id);
      return response.data;
    },
  });

  const handleSync = async () => {
    setSyncing(true);
    try {
      await streamarrApi.syncCollection(collection.id);
      await refetch();
      alert('Collection synced! Missing movies have been added to your library.');
    } catch (error) {
      console.error('Failed to sync collection:', error);
      alert('Failed to sync collection. Please try again.');
    } finally {
      setSyncing(false);
    }
  };

  // Close on escape
  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleEsc);
    return () => window.removeEventListener('keydown', handleEsc);
  }, [onClose]);

  const movies = collectionData?.movies || [];
  const progress = collection.total_movies 
    ? Math.round((collection.movies_in_library || 0) / collection.total_movies * 100) 
    : 0;

  return (
    <div className="fixed inset-0 z-50 bg-[#141414] overflow-y-auto">
      {/* Hero section */}
      <div className="relative min-h-[60vh] w-full">
        {/* Background image */}
        <div className="absolute inset-0">
          {collection.backdrop_path ? (
            <img
              src={tmdbImageUrl(collection.backdrop_path, 'original')}
              alt={collection.name}
              className="w-full h-full object-cover"
            />
          ) : collection.poster_path ? (
            <img
              src={tmdbImageUrl(collection.poster_path, 'original')}
              alt={collection.name}
              className="w-full h-full object-cover object-top"
            />
          ) : (
            <div className="w-full h-full bg-gradient-to-br from-slate-800 to-slate-950" />
          )}
          {/* Gradient overlays */}
          <div className="absolute inset-0 bg-gradient-to-r from-[#141414] via-[#141414]/60 to-transparent" />
          <div className="absolute inset-0 bg-gradient-to-t from-[#141414] via-transparent to-[#141414]/30" />
          <div className="absolute bottom-0 left-0 right-0 h-64 bg-gradient-to-t from-[#141414] to-transparent" />
        </div>

        {/* Back button */}
        <button
          onClick={onClose}
          className="absolute top-6 left-6 z-30 flex items-center gap-2 px-4 py-2 rounded-full bg-black/50 hover:bg-black/70 backdrop-blur-sm transition-all group"
        >
          <ArrowLeft className="w-5 h-5 text-white group-hover:scale-110 transition-transform" />
          <span className="text-white font-medium">Back</span>
        </button>

        {/* Content info */}
        <div className="absolute bottom-16 left-0 right-0 px-8 md:px-16 lg:px-20">
          <div className="max-w-3xl">
            {/* Type badge */}
            <div className="flex items-center gap-3 mb-4">
              <span className="px-3 py-1 rounded-md text-sm font-bold uppercase tracking-wide bg-cyan-600 text-white">
                Collection
              </span>
              <span className={`px-3 py-1 rounded-md text-sm font-bold ${
                progress === 100 ? 'bg-green-600' : 'bg-slate-600'
              } text-white`}>
                {progress}% Complete
              </span>
            </div>

            {/* Title */}
            <h1 className="text-4xl md:text-6xl lg:text-7xl font-black text-white mb-6 drop-shadow-2xl leading-tight">
              {collection.name}
            </h1>

            {/* Progress bar */}
            <div className="mb-6 max-w-md">
              <div className="flex items-center justify-between text-sm text-slate-300 mb-2">
                <span>{collection.movies_in_library || 0} of {collection.total_movies || 0} movies in library</span>
              </div>
              <div className="w-full bg-slate-700 rounded-full h-2">
                <div 
                  className={`h-2 rounded-full transition-all ${
                    progress === 100 ? 'bg-green-500' : 'bg-cyan-500'
                  }`}
                  style={{ width: `${progress}%` }}
                />
              </div>
            </div>

            {/* Overview */}
            {collection.overview && (
              <p className="text-slate-200 text-lg md:text-xl leading-relaxed mb-8 line-clamp-4">
                {collection.overview}
              </p>
            )}

            {/* Action buttons */}
            {progress < 100 && (
              <div className="flex items-center gap-4">
                <button 
                  onClick={handleSync}
                  disabled={syncing}
                  className="flex items-center gap-2 px-6 py-3 bg-cyan-600 hover:bg-cyan-500 text-white font-semibold rounded-lg transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed" 
                >
                  {syncing ? (
                    <>
                      <Loader2 className="w-5 h-5 animate-spin" />
                      Syncing...
                    </>
                  ) : (
                    <>
                      <RefreshCw className="w-5 h-5" />
                      Add Missing Movies
                    </>
                  )}
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Movies in collection */}
      <div className="relative z-10 px-8 md:px-16 lg:px-20 pb-20 -mt-8 bg-[#141414]">
        <h2 className="text-2xl font-bold text-white mb-6">Movies in Collection</h2>
        
        {isLoading ? (
          <div className="flex items-center justify-center py-16">
            <Loader2 className="w-10 h-10 animate-spin text-cyan-600" />
          </div>
        ) : movies.length === 0 ? (
          <div className="text-center py-16 text-slate-400">
            <Film className="w-16 h-16 mx-auto mb-4 text-slate-600" />
            <p className="text-lg">No movies found in this collection</p>
          </div>
        ) : (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-4">
            {movies.map((movie: any) => (
              <div 
                key={movie.id || movie.tmdb_id} 
                className={`group ${movie.in_library ? 'cursor-pointer' : 'cursor-default'}`}
                onClick={() => {
                  if (movie.in_library && onMovieClick) {
                    onMovieClick(movie);
                  }
                }}
              >
                <div className={`relative aspect-[2/3] rounded-lg overflow-hidden bg-slate-800 mb-2 transition-all duration-200 ${movie.in_library ? 'group-hover:ring-2 group-hover:ring-cyan-500 group-hover:scale-105' : ''}`}>
                  {movie.poster_path ? (
                    <img
                      src={tmdbImageUrl(movie.poster_path, 'w342')}
                      alt={movie.title}
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center">
                      <Film className="w-12 h-12 text-slate-600" />
                    </div>
                  )}
                  
                  {/* In Library badge */}
                  <div className="absolute top-2 right-2">
                    {movie.in_library ? (
                      <span className="px-2 py-1 rounded text-xs font-bold bg-green-600 text-white">
                        ✓ In Library
                      </span>
                    ) : (
                      <span className="px-2 py-1 rounded text-xs font-bold bg-slate-600 text-white">
                        Missing
                      </span>
                    )}
                  </div>

                  {/* Year */}
                  {movie.release_date && (
                    <div className="absolute bottom-2 left-2 bg-black/80 px-2 py-1 rounded backdrop-blur-sm">
                      <span className="text-white text-xs font-medium">
                        {new Date(movie.release_date).getFullYear()}
                      </span>
                    </div>
                  )}

                  {/* Hover overlay for clickable movies */}
                  {movie.in_library && (
                    <div className="absolute inset-0 bg-black/0 group-hover:bg-black/30 transition-all duration-200 flex items-center justify-center opacity-0 group-hover:opacity-100">
                      <span className="text-white text-sm font-semibold bg-black/50 px-3 py-1.5 rounded-full backdrop-blur-sm">
                        View Details
                      </span>
                    </div>
                  )}
                </div>

                <h3 className="text-white text-sm font-medium line-clamp-2">
                  {movie.title}
                </h3>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// Main Library Component
export default function Library() {
  const [selectedMedia, setSelectedMedia] = useState<MediaItem | null>(null);
  const [selectedCollection, setSelectedCollection] = useState<Collection | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [searchParams, setSearchParams] = useSearchParams();
  const [currentPage, setCurrentPage] = useState(1);
  const [sortBy, setSortBy] = useState('title');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');
  
  // Filter states
  const [selectedGenres, setSelectedGenres] = useState<string[]>([]);
  const [selectedYears, setSelectedYears] = useState<number[]>([]);
  
  // Dropdown states
  const [openDropdown, setOpenDropdown] = useState<string | null>(null);
  
  // Mass selection states
  const [selectionMode, setSelectionMode] = useState(false);
  const [selectedItems, setSelectedItems] = useState<Set<string>>(new Set()); // Format: "type-id"
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteProgress, setDeleteProgress] = useState({ current: 0, total: 0 });
  
  const ITEMS_PER_PAGE = 50;

  // Get current view from URL params (default to 'all')
  const currentView = searchParams.get('view') || 'all';
  
  // Get direct media ID from URL params (for linking from dashboard)
  const movieIdParam = searchParams.get('movie');
  const seriesIdParam = searchParams.get('series');

  // Fetch movies with sort params
  const { data: movies = [], isLoading: moviesLoading } = useQuery({
    queryKey: ['movies', 'library', sortBy, sortOrder],
    queryFn: () => streamarrApi.getMovies({ limit: 10000, sort: sortBy, order: sortOrder }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Fetch series with sort params
  const { data: series = [], isLoading: seriesLoading } = useQuery({
    queryKey: ['series', 'library', sortBy, sortOrder],
    queryFn: () => streamarrApi.getSeries({ limit: 10000, sort: sortBy, order: sortOrder }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Fetch collections
  const { data: collectionsData, isLoading: collectionsLoading } = useQuery({
    queryKey: ['collections'],
    queryFn: () => streamarrApi.getCollections({ limit: 1000 }).then(res => res.data),
    enabled: currentView === 'collections',
  });
  const collections: Collection[] = collectionsData?.collections || [];

  const isLoading = currentView === 'collections' ? collectionsLoading : (moviesLoading || seriesLoading);

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
      year: s.year || (s.first_air_date ? new Date(s.first_air_date).getFullYear() : undefined),
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

  // Extract available filter values
  const availableFilters = useMemo(() => {
    const genres = new Set<string>();
    const years = new Set<number>();

    allMedia.forEach(media => {
      // Extract genres from metadata if available
      if (media.metadata?.genres) {
        (media.metadata.genres as string[]).forEach(g => genres.add(g));
      }
      // Year is directly on the media item from API
      if (media.year) {
        years.add(media.year);
      }
    });

    return {
      genres: Array.from(genres).sort(),
      years: Array.from(years).sort((a, b) => b - a),
    };
  }, [allMedia]);

  // Filtered media based on current view and search
  const filteredMedia = useMemo(() => {
    let filtered = [...allMedia];

    // Apply search filter if search term exists
    if (searchTerm.trim()) {
      const term = searchTerm.toLowerCase();
      filtered = filtered.filter(m => m.title.toLowerCase().includes(term));
    } else {
      // Apply view filters only if not searching
      switch (currentView) {
        case 'recently-added-movies':
          filtered = filtered.filter(m => m.type === 'movie');
          break;
        case 'recently-added-series':
          filtered = filtered.filter(m => m.type === 'series');
          break;
        case 'movies':
          filtered = filtered.filter(m => m.type === 'movie');
          break;
        case 'series':
          filtered = filtered.filter(m => m.type === 'series');
          break;
        default:
          // Show all
          break;
      }
    }

    // Apply genre filter
    if (selectedGenres.length > 0) {
      filtered = filtered.filter(m => {
        const mediaGenres = m.metadata?.genres as string[] || [];
        return selectedGenres.some(g => mediaGenres.includes(g));
      });
    }

    // Apply year filter
    if (selectedYears.length > 0) {
      filtered = filtered.filter(m => m.year && selectedYears.includes(m.year));
    }

    // Apply sorting based on sortBy and sortOrder
    const ascending = sortOrder === 'asc';
    filtered.sort((a, b) => {
      switch (sortBy) {
        case 'title':
          return ascending 
            ? a.title.toLowerCase().localeCompare(b.title.toLowerCase())
            : b.title.toLowerCase().localeCompare(a.title.toLowerCase());

        case 'date_added':
          const aDate = new Date(a.added_at || 0).getTime();
          const bDate = new Date(b.added_at || 0).getTime();
          return ascending ? aDate - bDate : bDate - aDate;

        case 'release_date':
          const aRelease = new Date(a.release_date || 0).getTime();
          const bRelease = new Date(b.release_date || 0).getTime();
          return ascending ? aRelease - bRelease : bRelease - aRelease;

        case 'rating':
          const aRating = a.vote_average || 0;
          const bRating = b.vote_average || 0;
          return ascending ? aRating - bRating : bRating - aRating;

        case 'runtime':
          const aRuntime = a.metadata?.runtime || 0;
          const bRuntime = b.metadata?.runtime || 0;
          return ascending ? aRuntime - bRuntime : bRuntime - aRuntime;

        case 'monitored':
          if (a.monitored === b.monitored) {
            return a.title.toLowerCase().localeCompare(b.title.toLowerCase());
          }
          return ascending ? (a.monitored ? -1 : 1) : (a.monitored ? 1 : -1);

        case 'genre':
          const aGenre = a.metadata?.genres?.[0] || '';
          const bGenre = b.metadata?.genres?.[0] || '';
          return ascending 
            ? aGenre.localeCompare(bGenre)
            : bGenre.localeCompare(aGenre);

        case 'language':
          const aLang = a.metadata?.original_language || '';
          const bLang = b.metadata?.original_language || '';
          return ascending 
            ? aLang.localeCompare(bLang)
            : bLang.localeCompare(aLang);

        case 'country':
          const aCountries = a.metadata?.production_countries as any[] || [];
          const bCountries = b.metadata?.production_countries as any[] || [];
          const aCountry = aCountries.length > 0 && aCountries[0].iso_3166_1 ? aCountries[0].iso_3166_1 : '';
          const bCountry = bCountries.length > 0 && bCountries[0].iso_3166_1 ? bCountries[0].iso_3166_1 : '';
          return ascending 
            ? aCountry.localeCompare(bCountry)
            : bCountry.localeCompare(aCountry);

        default:
          return ascending 
            ? a.title.toLowerCase().localeCompare(b.title.toLowerCase())
            : b.title.toLowerCase().localeCompare(a.title.toLowerCase());
      }
    });

    return filtered;
  }, [allMedia, currentView, searchTerm, sortBy, sortOrder, selectedGenres, selectedYears]);

  // Pagination
  const totalPages = Math.ceil(filteredMedia.length / ITEMS_PER_PAGE);
  const startIndex = (currentPage - 1) * ITEMS_PER_PAGE;
  const endIndex = startIndex + ITEMS_PER_PAGE;
  const currentItems = filteredMedia.slice(startIndex, endIndex);

  // Reset page when view changes
  useEffect(() => {
    setCurrentPage(1);
  }, [currentView]);

  // Auto-open detail modal when movie/series ID is in URL (from dashboard links)
  useEffect(() => {
    if (!isLoading && allMedia.length > 0) {
      if (movieIdParam) {
        const movie = allMedia.find(m => m.type === 'movie' && m.id === parseInt(movieIdParam));
        if (movie) {
          setSelectedMedia(movie);
          // Clear the URL param after opening
          setSearchParams(prev => {
            prev.delete('movie');
            return prev;
          });
        }
      } else if (seriesIdParam) {
        const show = allMedia.find(m => m.type === 'series' && m.id === parseInt(seriesIdParam));
        if (show) {
          setSelectedMedia(show);
          // Clear the URL param after opening
          setSearchParams(prev => {
            prev.delete('series');
            return prev;
          });
        }
      }
    }
  }, [movieIdParam, seriesIdParam, allMedia, isLoading, setSearchParams]);

  // View titles
  const viewTitles: Record<string, string> = {
    'all': 'All Media',
    'recently-added-movies': 'Recently Added Movies',
    'recently-added-series': 'Recently Added Series',
    'movies': 'Movies',
    'series': 'TV Shows',
    'collections': 'Collections',
  };

  // Selection helpers
  const toggleItemSelection = (item: MediaItem) => {
    const key = `${item.type}-${item.id}`;
    setSelectedItems(prev => {
      const newSet = new Set(prev);
      if (newSet.has(key)) {
        newSet.delete(key);
      } else {
        newSet.add(key);
      }
      return newSet;
    });
  };

  const selectAll = () => {
    const allKeys = currentItems.map(item => `${item.type}-${item.id}`);
    setSelectedItems(new Set(allKeys));
  };

  const deselectAll = () => {
    setSelectedItems(new Set());
  };

  const cancelSelection = () => {
    setSelectionMode(false);
    setSelectedItems(new Set());
  };

  const handleBulkDelete = async () => {
    if (selectedItems.size === 0) return;
    
    const confirmed = window.confirm(
      `Are you sure you want to delete ${selectedItems.size} item(s) from your library and add them to the blacklist?`
    );
    
    if (!confirmed) return;

    setIsDeleting(true);
    setDeleteProgress({ current: 0, total: selectedItems.size });
    
    const itemsToDelete = Array.from(selectedItems);
    let successCount = 0;
    
    for (let i = 0; i < itemsToDelete.length; i++) {
      const [type, idStr] = itemsToDelete[i].split('-');
      const id = parseInt(idStr);
      
      try {
        await streamarrApi.removeAndBlacklist(type as 'movie' | 'series', id, 'Bulk deleted by user');
        successCount++;
      } catch (error) {
        console.error(`Failed to delete ${type} ${id}:`, error);
      }
      
      setDeleteProgress({ current: i + 1, total: itemsToDelete.length });
    }
    
    setIsDeleting(false);
    setSelectionMode(false);
    setSelectedItems(new Set());
    
    // Show result and reload
    alert(`Successfully deleted ${successCount} of ${itemsToDelete.length} items.`);
    window.location.reload();
  };

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
      {/* Filter Tabs with Search */}
      <div className="relative z-50 px-12 pt-6 mb-6">
        {/* Search Bar and Sort Controls */}
        <div className="mb-6 flex flex-col sm:flex-row gap-4 items-start sm:items-center">
          <div className="relative flex-1 max-w-md">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-5 h-5 text-slate-400" />
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder="Search library..."
              className="w-full pl-10 pr-4 py-2.5 bg-white/10 text-white placeholder-slate-400 rounded-lg border border-white/20 focus:border-white/40 focus:outline-none transition-colors"
            />
            {searchTerm && (
              <button
                onClick={() => setSearchTerm('')}
                className="absolute right-3 top-1/2 transform -translate-y-1/2 p-1 hover:bg-white/10 rounded transition-colors"
              >
                <X className="w-4 h-4 text-slate-400" />
              </button>
            )}
          </div>

          {/* Sort Controls */}
          <div className="flex gap-3 items-center flex-wrap overflow-visible">
            <select
              value={sortBy}
              onChange={(e) => {
                setSortBy(e.target.value);
                setCurrentPage(1);
              }}
              className="px-3 py-2.5 bg-white/10 text-white rounded-lg border border-white/20 focus:border-white/40 focus:outline-none transition-colors text-sm"
            >
              <option value="title">Sort by Title</option>
              <option value="date_added">Sort by Date Added</option>
              <option value="release_date">Sort by Release Date</option>
              <option value="rating">Sort by Rating</option>
              <option value="runtime">Sort by Runtime</option>
              <option value="monitored">Sort by Monitored</option>
              <option value="genre">Sort by Genre</option>
              <option value="language">Sort by Language</option>
              <option value="country">Sort by Country</option>
            </select>

            <button
              onClick={() => {
                setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
                setCurrentPage(1);
              }}
              className="px-4 py-2.5 bg-white/10 text-white rounded-lg border border-white/20 hover:bg-white/20 focus:border-white/40 focus:outline-none transition-colors text-sm font-medium"
              title={sortOrder === 'asc' ? 'Ascending' : 'Descending'}
            >
              {sortOrder === 'asc' ? '↑ Asc' : '↓ Desc'}
            </button>

            {/* Genres Dropdown */}
            <div className="relative">
              <button
                onClick={() => setOpenDropdown(openDropdown === 'genres' ? null : 'genres')}
                className="px-3 py-2.5 bg-white/10 text-white rounded-lg border border-white/20 hover:bg-white/20 focus:border-white/40 focus:outline-none transition-colors text-sm font-medium"
              >
                Genres {selectedGenres.length > 0 && `(${selectedGenres.length})`}
              </button>
              {openDropdown === 'genres' && (
                <>
                  <div className="fixed top-0 left-0 w-full h-full z-[998]" onClick={() => setOpenDropdown(null)} />
                  <div className="absolute top-full mt-2 left-0 bg-[#242424] border border-white/20 rounded-lg p-2 z-[9999] max-h-96 overflow-y-auto w-56 shadow-2xl">
                    {availableFilters.genres.map(g => (
                      <label key={g} className="flex items-center gap-2 px-2 py-1 hover:bg-white/10 rounded cursor-pointer text-white">
                        <input
                          type="checkbox"
                          checked={selectedGenres.includes(g)}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setSelectedGenres([...selectedGenres, g]);
                            } else {
                              setSelectedGenres(selectedGenres.filter(sg => sg !== g));
                            }
                            setCurrentPage(1);
                          }}
                          className="w-4 h-4 accent-white"
                        />
                        <span className="text-sm">{g}</span>
                      </label>
                    ))}
                  </div>
                </>
              )}
            </div>

            {/* Years Dropdown */}
            <div className="relative z-40">
              <button
                onClick={() => setOpenDropdown(openDropdown === 'years' ? null : 'years')}
                className="px-3 py-2.5 bg-white/10 text-white rounded-lg border border-white/20 hover:bg-white/20 focus:border-white/40 focus:outline-none transition-colors text-sm font-medium"
              >
                Years {selectedYears.length > 0 && `(${selectedYears.length})`}
              </button>
              {openDropdown === 'years' && (
                <>
                  <div className="fixed top-0 left-0 w-full h-full z-[998]" onClick={() => setOpenDropdown(null)} />
                  <div className="absolute top-full mt-2 left-0 bg-[#242424] border border-white/20 rounded-lg p-2 z-[9999] max-h-96 overflow-y-auto w-56 shadow-2xl">
                    {availableFilters.years.map(y => (
                      <label key={y} className="flex items-center gap-2 px-2 py-1 hover:bg-white/10 rounded cursor-pointer text-white">
                        <input
                          type="checkbox"
                          checked={selectedYears.includes(y)}
                          onChange={(e) => {
                            if (e.target.checked) {
                              setSelectedYears([...selectedYears, y]);
                            } else {
                              setSelectedYears(selectedYears.filter(sy => sy !== y));
                            }
                            setCurrentPage(1);
                          }}
                          className="w-4 h-4 accent-white"
                        />
                        <span className="text-sm">{y}</span>
                      </label>
                    ))}
                  </div>
                </>
              )}
            </div>

            {/* Clear Filters Button */}
            {(selectedGenres.length > 0 || selectedYears.length > 0) && (
              <button
                onClick={() => {
                  setSelectedGenres([]);
                  setSelectedYears([]);
                  setCurrentPage(1);
                }}
                className="px-3 py-2.5 bg-red-600/20 text-red-400 rounded-lg border border-red-600/30 hover:bg-red-600/30 transition-colors text-sm font-medium"
              >
                Clear Filters
              </button>
            )}

            {/* Selection Mode Toggle */}
            {!selectionMode ? (
              <button
                onClick={() => setSelectionMode(true)}
                className="px-3 py-2.5 bg-white/10 text-white rounded-lg border border-white/20 hover:bg-white/20 transition-colors text-sm font-medium flex items-center gap-2"
              >
                <CheckSquare className="w-4 h-4" />
                Select
              </button>
            ) : (
              <div className="flex items-center gap-2">
                <span className="text-white text-sm">
                  {selectedItems.size} selected
                </span>
                <button
                  onClick={selectAll}
                  className="px-3 py-2.5 bg-white/10 text-white rounded-lg border border-white/20 hover:bg-white/20 transition-colors text-sm font-medium"
                >
                  Select All
                </button>
                <button
                  onClick={deselectAll}
                  className="px-3 py-2.5 bg-white/10 text-white rounded-lg border border-white/20 hover:bg-white/20 transition-colors text-sm font-medium"
                  disabled={selectedItems.size === 0}
                >
                  Deselect All
                </button>
                <button
                  onClick={handleBulkDelete}
                  disabled={selectedItems.size === 0 || isDeleting}
                  className="px-3 py-2.5 bg-red-600 text-white rounded-lg border border-red-500 hover:bg-red-700 transition-colors text-sm font-medium flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <Trash2 className="w-4 h-4" />
                  Delete ({selectedItems.size})
                </button>
                <button
                  onClick={cancelSelection}
                  className="px-3 py-2.5 bg-white/10 text-white rounded-lg border border-white/20 hover:bg-white/20 transition-colors text-sm font-medium flex items-center gap-2"
                >
                  <XCircle className="w-4 h-4" />
                  Cancel
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-2 overflow-x-auto pb-2 scrollbar-hide">
          {[
            { key: 'all', label: 'All' },
            { key: 'recently-added-movies', label: 'Recent Movies' },
            { key: 'recently-added-series', label: 'Recent Series' },
            { key: 'movies', label: 'Movies' },
            { key: 'series', label: 'TV Shows' },
            { key: 'collections', label: '📦 Collections' },
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
      <div className="relative z-0 px-12 pb-12">
        <div className="mb-4">
          <h2 className="text-2xl font-bold text-white mb-2">{viewTitles[currentView]}</h2>
          {currentView === 'collections' ? (
            <p className="text-slate-400">
              {collections.length} collection{collections.length !== 1 ? 's' : ''}
            </p>
          ) : (
            <p className="text-slate-400">
              Showing {startIndex + 1}-{Math.min(endIndex, filteredMedia.length)} of {filteredMedia.length} items
            </p>
          )}
        </div>

        {/* Collections View */}
        {currentView === 'collections' ? (
          collections.length > 0 ? (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-4 mb-8">
              {collections.map((collection) => (
                <CollectionCard
                  key={collection.id}
                  collection={collection}
                  onClick={() => setSelectedCollection(collection)}
                />
              ))}
            </div>
          ) : (
            <div className="text-center py-20">
              <LibraryIcon className="w-16 h-16 mx-auto mb-4 text-slate-600" />
              <p className="text-slate-400 text-xl">No collections found</p>
              <p className="text-slate-500 text-sm mt-2">
                Collections are automatically created when you add movies that belong to a collection
              </p>
            </div>
          )
        ) : currentItems.length > 0 ? (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-4 mb-8">
              {currentItems.map((item) => (
                <div
                  key={`${item.type}-${item.id}`}
                  onClick={() => {
                    if (selectionMode) {
                      toggleItemSelection(item);
                    } else {
                      setSelectedMedia(item);
                    }
                  }}
                  className={`group cursor-pointer ${
                    selectionMode && selectedItems.has(`${item.type}-${item.id}`)
                      ? 'ring-2 ring-red-500 rounded-lg'
                      : ''
                  }`}
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

                    {/* Selection Checkbox */}
                    {selectionMode && (
                      <div className="absolute top-2 right-2 z-10">
                        {selectedItems.has(`${item.type}-${item.id}`) ? (
                          <CheckSquare className="w-6 h-6 text-red-500 bg-white rounded" />
                        ) : (
                          <Square className="w-6 h-6 text-white bg-black/50 rounded" />
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

      {/* Collection Detail Modal */}
      {selectedCollection && (
        <CollectionDetailModal 
          collection={selectedCollection} 
          onClose={() => setSelectedCollection(null)}
          onMovieClick={(movie) => {
            // Find the movie in allMedia by tmdb_id and open detail modal
            const libraryMovie = allMedia.find(m => m.type === 'movie' && m.tmdb_id === movie.tmdb_id);
            if (libraryMovie) {
              setSelectedCollection(null);
              setSelectedMedia(libraryMovie);
            }
          }}
        />
      )}

      {/* Deletion Progress Overlay */}
      {isDeleting && (
        <div className="fixed inset-0 bg-black/80 z-[9999] flex items-center justify-center">
          <div className="bg-[#242424] rounded-xl p-8 max-w-md w-full mx-4 text-center">
            <Loader2 className="w-12 h-12 animate-spin text-red-600 mx-auto mb-4" />
            <h3 className="text-white text-xl font-bold mb-2">Deleting Items</h3>
            <p className="text-slate-400 mb-4">
              Deleting {deleteProgress.current} of {deleteProgress.total} items...
            </p>
            <div className="w-full bg-slate-700 rounded-full h-2">
              <div 
                className="bg-red-600 h-2 rounded-full transition-all duration-300"
                style={{ width: `${(deleteProgress.current / deleteProgress.total) * 100}%` }}
              />
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
