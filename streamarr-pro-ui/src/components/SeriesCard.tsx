import type { Series } from '../types';
import { tmdbImageUrl } from '../services/api';
import { Play, Check, Clock, Star, Tv } from 'lucide-react';

interface SeriesCardProps {
  series: Series;
  onClick: () => void;
}

export default function SeriesCard({ series, onClick }: SeriesCardProps) {
  const year = series.first_air_date ? new Date(series.first_air_date).getFullYear() : 'N/A';
  const rating = series.metadata?.vote_average ? Number(series.metadata.vote_average).toFixed(1) : 'N/A';

  return (
    <div
      className="poster-card cursor-pointer group"
      onClick={onClick}
    >
      <div className="relative aspect-[2/3] bg-slate-800 rounded-lg overflow-hidden">
        <img
          src={tmdbImageUrl(series.poster_path, 'w500')}
          alt={series.title}
          className="w-full h-full object-cover"
          loading="lazy"
        />
        
        {/* Overlay */}
        <div className="absolute inset-0 bg-gradient-to-t from-black/90 via-black/50 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300">
          <div className="absolute bottom-0 left-0 right-0 p-4">
            <h3 className="text-white font-semibold text-lg mb-2 line-clamp-2">
              {series.title}
            </h3>
            
            <div className="flex items-center gap-2 text-sm text-slate-300 mb-2">
              <span>{year}</span>
              <span>•</span>
              <div className="flex items-center gap-1">
                <Star className="w-4 h-4 fill-yellow-400 text-yellow-400" />
                <span>{rating}</span>
              </div>
            </div>

            <div className="flex items-center gap-2 text-sm text-slate-300 mb-3">
              <Tv className="w-4 h-4" />
              <span>{series.seasons} Season{series.seasons !== 1 ? 's' : ''}</span>
              <span>•</span>
              <span>{series.total_episodes} Episodes</span>
            </div>

            <div className="flex items-center gap-2">
              <button className="btn btn-primary text-sm flex items-center gap-1 flex-1">
                <Play className="w-4 h-4" />
                View
              </button>
              
              {series.monitored && (
                <div className="bg-green-500/20 text-green-400 px-2 py-1 rounded text-xs flex items-center gap-1">
                  <Check className="w-3 h-3" />
                  Monitored
                </div>
              )}
              
              {!series.monitored && (
                <div className="bg-slate-500/20 text-slate-400 px-2 py-1 rounded text-xs flex items-center gap-1">
                  <Clock className="w-3 h-3" />
                  Unmonitored
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Status Badges */}
        <div className="absolute top-2 right-2 flex flex-col gap-1">
          {series.status && (
            <div className="bg-black/70 backdrop-blur-sm px-2 py-0.5 rounded text-xs text-white font-medium">
              {series.status}
            </div>
          )}
          {series.quality_profile && (
            <div className="bg-black/70 backdrop-blur-sm px-2 py-0.5 rounded text-xs text-white font-medium">
              {series.quality_profile}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
