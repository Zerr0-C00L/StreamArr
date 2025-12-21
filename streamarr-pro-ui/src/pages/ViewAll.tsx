import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams, Link } from 'react-router-dom';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import { ArrowLeft, ChevronLeft, ChevronRight, Film, Tv, Loader2 } from 'lucide-react';
import type { Movie, Series } from '../types';

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
  metadata?: Record<string, any>;
};

const ITEMS_PER_PAGE = 50;

export default function ViewAll() {
  const [searchParams] = useSearchParams();
  const category = searchParams.get('category') || 'all';
  const [currentPage, setCurrentPage] = useState(1);

  // Fetch movies
  const { data: movies = [], isLoading: moviesLoading } = useQuery({
    queryKey: ['movies', 'viewall'],
    queryFn: () => streamarrApi.getMovies({ limit: 10000 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Fetch series
  const { data: series = [], isLoading: seriesLoading } = useQuery({
    queryKey: ['series', 'viewall'],
    queryFn: () => streamarrApi.getSeries({ limit: 10000 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  const isLoading = moviesLoading || seriesLoading;

  // Transform to MediaItems
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
      metadata: m.metadata,
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
    }));

    return [...movieItems, ...seriesItems];
  }, [movies, series]);

  // Filter and sort based on category
  const filteredMedia = useMemo(() => {
    let filtered = [...allMedia];

    switch (category) {
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
  }, [allMedia, category]);

  // Pagination
  const totalPages = Math.ceil(filteredMedia.length / ITEMS_PER_PAGE);
  const startIndex = (currentPage - 1) * ITEMS_PER_PAGE;
  const endIndex = startIndex + ITEMS_PER_PAGE;
  const currentItems = filteredMedia.slice(startIndex, endIndex);

  // Category title
  const categoryTitle = {
    'recently-added-movies': 'Recently Added Movies',
    'recently-added-series': 'Recently Added Series',
    'movies': 'Movies',
    'series': 'TV Shows',
    'all': 'All Media',
  }[category] || 'All Media';

  if (isLoading) {
    return (
      <div className="min-h-screen bg-[#141414] flex items-center justify-center">
        <Loader2 className="w-12 h-12 animate-spin text-red-600" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#141414] py-8 px-8">
      {/* Header */}
      <div className="mb-8">
        <Link
          to="/"
          className="inline-flex items-center gap-2 text-slate-400 hover:text-white mb-4 transition-colors"
        >
          <ArrowLeft className="w-5 h-5" />
          Back to Home
        </Link>
        <h1 className="text-3xl font-bold text-white mb-2">{categoryTitle}</h1>
        <p className="text-slate-400">
          Showing {startIndex + 1}-{Math.min(endIndex, filteredMedia.length)} of {filteredMedia.length} items
        </p>
      </div>

      {/* Grid */}
      {currentItems.length > 0 ? (
        <>
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-4 mb-8">
            {currentItems.map((item) => (
              <Link
                key={`${item.type}-${item.id}`}
                to={`/?media=${item.type}-${item.tmdb_id}`}
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
              </Link>
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
          <p className="text-slate-400 text-xl">No items found in this category</p>
        </div>
      )}
    </div>
  );
}
