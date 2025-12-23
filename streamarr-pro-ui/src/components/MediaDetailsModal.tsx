import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { tmdbImageUrl } from '../services/api';
import { ArrowLeft, Star, Calendar, Plus, Check, Loader2, Film, Tv } from 'lucide-react';
import type { SearchResult } from '../types';
import type { TrendingItem } from '../services/api';

interface MediaDetailsModalProps {
  item: SearchResult | TrendingItem;
  mediaType: string;
  onClose: () => void;
  onAdd: (item: SearchResult | TrendingItem, type: string) => void;
  isAdding: boolean;
  isAdded: boolean;
}

export default function MediaDetailsModal({
  item,
  mediaType,
  onClose,
  onAdd,
  isAdding,
  isAdded
}: MediaDetailsModalProps) {
  const isMovie = mediaType === 'movie';
  const tmdbId = ('tmdb_id' in item && item.tmdb_id) ? item.tmdb_id : item.id;

  // Fetch full details from TMDB
  const { data: details, isLoading } = useQuery({
    queryKey: ['tmdb-details', mediaType, tmdbId],
    queryFn: async () => {
      const endpoint = isMovie ? 'movie' : 'tv';
      const response = await fetch(
        `https://api.themoviedb.org/3/${endpoint}/${tmdbId}?api_key=${import.meta.env.VITE_TMDB_API_KEY || ''}&append_to_response=credits,videos`
      );
      if (!response.ok) throw new Error('Failed to fetch details');
      return response.json();
    },
    enabled: !!tmdbId,
  });

  // Close on escape
  useEffect(() => {
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleEsc);
    return () => window.removeEventListener('keydown', handleEsc);
  }, [onClose]);

  const title = item.title || item.name || 'Unknown';
  const overview = item.overview || details?.overview || 'No overview available.';
  const backdropPath = ('backdrop_path' in item ? item.backdrop_path : null) || details?.backdrop_path;
  const posterPath = item.poster_path || details?.poster_path;
  const year = item.release_date?.substring(0, 4) || item.first_air_date?.substring(0, 4) || details?.release_date?.substring(0, 4) || details?.first_air_date?.substring(0, 4);
  const rating = item.vote_average || details?.vote_average;
  const genres = details?.genres?.map((g: any) => g.name).join(', ') || '';
  const runtime = details?.runtime;
  const seasons = details?.number_of_seasons;

  return (
    <div className="fixed inset-0 z-50 bg-black/95 overflow-y-auto" onClick={onClose}>
      <div className="min-h-screen" onClick={(e) => e.stopPropagation()}>
        {/* Hero Section */}
        <div className="relative min-h-[70vh] w-full">
          {/* Background Image */}
          <div className="absolute inset-0">
            {backdropPath ? (
              <img
                src={tmdbImageUrl(backdropPath, 'original')}
                alt={title}
                className="w-full h-full object-cover"
              />
            ) : posterPath ? (
              <img
                src={tmdbImageUrl(posterPath, 'original')}
                alt={title}
                className="w-full h-full object-cover object-top"
              />
            ) : (
              <div className="w-full h-full bg-gradient-to-br from-slate-800 to-slate-950" />
            )}
            {/* Gradient Overlays */}
            <div className="absolute inset-0 bg-gradient-to-r from-[#141414] via-[#141414]/60 to-transparent" />
            <div className="absolute inset-0 bg-gradient-to-t from-[#141414] via-transparent to-[#141414]/30" />
            <div className="absolute bottom-0 left-0 right-0 h-64 bg-gradient-to-t from-[#141414] to-transparent" />
          </div>

          {/* Close Button */}
          <button
            onClick={onClose}
            className="absolute top-6 left-6 z-30 flex items-center gap-2 px-4 py-2 rounded-full bg-black/50 hover:bg-black/70 backdrop-blur-sm transition-all group"
          >
            <ArrowLeft className="w-5 h-5 text-white group-hover:scale-110 transition-transform" />
            <span className="text-white font-medium">Back</span>
          </button>

          {/* Content Info */}
          <div className="absolute bottom-16 left-0 right-0 px-8 md:px-16 lg:px-20">
            <div className="max-w-3xl">
              {/* Type Badge */}
              <div className="flex items-center gap-3 mb-4">
                <span className={`px-3 py-1 rounded-md text-sm font-bold uppercase tracking-wide ${
                  isMovie ? 'bg-purple-600' : 'bg-green-600'
                } text-white`}>
                  {isMovie ? 'Movie' : 'Series'}
                </span>
              </div>

              {/* Title */}
              <h1 className="text-4xl md:text-6xl lg:text-7xl font-black text-white mb-6 drop-shadow-2xl leading-tight">
                {title}
              </h1>

              {/* Meta Info */}
              <div className="flex flex-wrap items-center gap-4 mb-6">
                {rating && rating > 0 && (
                  <div className="flex items-center gap-1.5">
                    <Star className="w-5 h-5 text-yellow-400 fill-yellow-400" />
                    <span className="text-white font-bold text-lg">{rating.toFixed(1)}</span>
                  </div>
                )}
                {year && (
                  <div className="flex items-center gap-1.5">
                    <Calendar className="w-5 h-5 text-slate-400" />
                    <span className="text-slate-300 text-lg">{year}</span>
                  </div>
                )}
                {runtime && isMovie && (
                  <span className="text-slate-300 text-lg">{runtime} min</span>
                )}
                {seasons && !isMovie && (
                  <div className="flex items-center gap-1.5">
                    <Tv className="w-5 h-5 text-slate-400" />
                    <span className="text-slate-300 text-lg">{seasons} Season{seasons !== 1 ? 's' : ''}</span>
                  </div>
                )}
              </div>

              {/* Overview */}
              <p className="text-slate-200 text-lg md:text-xl leading-relaxed mb-8 line-clamp-4">
                {overview}
              </p>

              {/* Action Button */}
              <div className="flex items-center gap-4">
                {isAdded ? (
                  <button className="flex items-center gap-2 px-6 py-3 bg-green-600 text-white font-semibold rounded-lg cursor-default">
                    <Check className="w-5 h-5" />
                    Added to Library
                  </button>
                ) : (
                  <button
                    onClick={() => onAdd(item, mediaType)}
                    disabled={isAdding}
                    className="flex items-center gap-2 px-6 py-3 bg-white text-black font-semibold rounded-lg hover:bg-white/90 disabled:bg-slate-600 disabled:text-slate-400 transition-all hover:scale-105"
                  >
                    {isAdding ? (
                      <>
                        <Loader2 className="w-5 h-5 animate-spin" />
                        Adding...
                      </>
                    ) : (
                      <>
                        <Plus className="w-5 h-5" />
                        Add to Library
                      </>
                    )}
                  </button>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Additional Details Section */}
        <div className="relative z-10 px-8 md:px-16 lg:px-20 pb-20 bg-[#141414]">
          {isLoading ? (
            <div className="flex items-center justify-center py-16">
              <Loader2 className="w-10 h-10 animate-spin text-red-600" />
            </div>
          ) : (
            <div className="max-w-5xl space-y-8">
              {/* Genres */}
              {genres && (
                <div>
                  <h3 className="text-lg font-semibold text-white mb-3">Genres</h3>
                  <p className="text-slate-300">{genres}</p>
                </div>
              )}

              {/* Cast */}
              {details?.credits?.cast && details.credits.cast.length > 0 && (
                <div>
                  <h3 className="text-lg font-semibold text-white mb-4">Cast</h3>
                  <div className="flex gap-4 overflow-x-auto pb-4 scrollbar-hide">
                    {details.credits.cast.slice(0, 10).map((person: any) => (
                      <div key={person.id} className="flex-shrink-0 w-32">
                        <div className="aspect-[2/3] rounded-lg overflow-hidden bg-slate-800 mb-2">
                          {person.profile_path ? (
                            <img
                              src={tmdbImageUrl(person.profile_path, 'w200')}
                              alt={person.name}
                              className="w-full h-full object-cover"
                            />
                          ) : (
                            <div className="w-full h-full flex items-center justify-center">
                              <Film className="w-8 h-8 text-slate-600" />
                            </div>
                          )}
                        </div>
                        <p className="text-white text-sm font-medium truncate">{person.name}</p>
                        <p className="text-slate-400 text-xs truncate">{person.character}</p>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
