import type { Movie } from '../types';
import { tmdbImageUrl } from '../services/api';
import { Play, Check, Clock, Star, Layers } from 'lucide-react';

interface MovieCardProps {
  movie: Movie;
  onClick: () => void;
}

export default function MovieCard({ movie, onClick }: MovieCardProps) {
  const year = movie.release_date ? new Date(movie.release_date).getFullYear() : 'N/A';
  const rating = movie.metadata?.vote_average ? Number(movie.metadata.vote_average).toFixed(1) : 'N/A';

  return (
    <div
      className="poster-card cursor-pointer group"
      onClick={onClick}
    >
      <div className="relative aspect-[2/3] bg-slate-800 rounded-lg overflow-hidden">
        <img
          src={tmdbImageUrl(movie.poster_path, 'w500')}
          alt={movie.title}
          className="w-full h-full object-cover"
          loading="lazy"
        />
        
        {/* Overlay */}
        <div className="absolute inset-0 bg-gradient-to-t from-black/90 via-black/50 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300">
          <div className="absolute bottom-0 left-0 right-0 p-4">
            <h3 className="text-white font-semibold text-lg mb-2 line-clamp-2">
              {movie.title}
            </h3>
            
            <div className="flex items-center gap-2 text-sm text-slate-300 mb-3">
              <span>{year}</span>
              <span>•</span>
              <div className="flex items-center gap-1">
                <Star className="w-4 h-4 fill-yellow-400 text-yellow-400" />
                <span>{rating}</span>
              </div>
              {movie.runtime > 0 && (
                <>
                  <span>•</span>
                  <span>{movie.runtime}m</span>
                </>
              )}
            </div>

            <div className="flex items-center gap-2">
              {movie.available && (
                <button className="btn btn-primary text-sm flex items-center gap-1 flex-1">
                  <Play className="w-4 h-4" />
                  Play
                </button>
              )}
              
              {movie.monitored && (
                <div className="bg-green-500/20 text-green-400 px-2 py-1 rounded text-xs flex items-center gap-1">
                  <Check className="w-3 h-3" />
                  Monitored
                </div>
              )}
              
              {!movie.available && movie.monitored && (
                <div className="bg-yellow-500/20 text-yellow-400 px-2 py-1 rounded text-xs flex items-center gap-1">
                  <Clock className="w-3 h-3" />
                  Searching
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Status Badges (Top Right) */}
        <div className="absolute top-2 right-2 flex flex-col gap-1">
          {movie.quality_profile && (
            <div className="bg-black/70 backdrop-blur-sm px-2 py-0.5 rounded text-xs text-white font-medium">
              {movie.quality_profile}
            </div>
          )}
        </div>

        {/* Collection Badge (Top Left) */}
        {movie.collection && (
          <div className="absolute top-2 left-2">
            <div 
              className="bg-purple-600/90 backdrop-blur-sm px-2 py-1 rounded text-xs text-white font-medium flex items-center gap-1"
              title={movie.collection.name}
            >
              <Layers className="w-3 h-3" />
              <span className="max-w-[80px] truncate">{movie.collection.name}</span>
              {movie.collection.movies_in_library && movie.collection.total_movies && (
                <span className="text-purple-200 text-[10px]">
                  {movie.collection.movies_in_library}/{movie.collection.total_movies}
                </span>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
