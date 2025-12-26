import { useState, useMemo } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useSearchParams, Link } from 'react-router-dom';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import { 
  Layers, ChevronLeft, ChevronRight, Plus, Check, Loader2, 
  Search as SearchIcon, ArrowLeft, CheckCircle, AlertCircle, X, SlidersHorizontal
} from 'lucide-react';
import type { Collection } from '../types';

export default function BrowseCollections() {
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();
  
  const page = parseInt(searchParams.get('page') || '1');
  const searchQuery = searchParams.get('query') || '';
  const [searchInput, setSearchInput] = useState(searchQuery);
  const [addingCollectionId, setAddingCollectionId] = useState<number | null>(null);
  const [newlyAddedIds, setNewlyAddedIds] = useState<Set<number>>(new Set());
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [sortBy, setSortBy] = useState<'name' | 'name-desc' | 'recent' | 'in-library'>('name');

  // Fetch collections with pagination
  const { data: collectionsData, isLoading } = useQuery({
    queryKey: ['browse-collections', page, searchQuery],
    queryFn: () => streamarrApi.browseCollections(page, searchQuery || undefined).then(res => res.data),
    staleTime: 5 * 60 * 1000,
  });

  // Fetch library collections to show which are already added
  const { data: libraryCollections = [] } = useQuery({
    queryKey: ['library-collections'],
    queryFn: () => streamarrApi.getCollections({ limit: 10000 }).then(res => {
      // Response is { collections: [...], total: ... }
      const data = res.data;
      if (Array.isArray(data)) return data;
      if (data?.collections && Array.isArray(data.collections)) return data.collections;
      return [];
    }),
    staleTime: 5 * 60 * 1000,
  });

  const libraryCollectionIds = useMemo(() => {
    const ids = new Set<number>();
    libraryCollections.forEach((c: any) => c.tmdb_id && ids.add(c.tmdb_id));
    return ids;
  }, [libraryCollections]);

  // Check if collection is in library (includes newly added)
  const isInLibrary = (tmdbId: number) => libraryCollectionIds.has(tmdbId) || newlyAddedIds.has(tmdbId);

  // Add collection mutation
  const addCollectionMutation = useMutation({
    mutationFn: (tmdbId: number) => streamarrApi.addCollection(tmdbId),
    onSuccess: (_data, tmdbId) => {
      // Immediately mark as added
      setNewlyAddedIds(prev => new Set([...prev, tmdbId]));
      queryClient.invalidateQueries({ queryKey: ['library-collections'] });
    },
  });

  const handleAddCollection = async (collection: Collection) => {
    if (isInLibrary(collection.tmdb_id)) return;
    setAddingCollectionId(collection.tmdb_id);
    setErrorMessage(null);
    try {
      await addCollectionMutation.mutateAsync(collection.tmdb_id);
    } catch (error: any) {
      const message = error.response?.data?.error || 'Failed to add collection';
      setErrorMessage(`${collection.name}: ${message}`);
      // Auto-clear error after 5 seconds
      setTimeout(() => setErrorMessage(null), 5000);
    } finally {
      setAddingCollectionId(null);
    }
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setSearchParams({ page: '1', query: searchInput });
  };

  const handlePageChange = (newPage: number) => {
    const params: Record<string, string> = { page: String(newPage) };
    if (searchQuery) params.query = searchQuery;
    setSearchParams(params);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const collections = collectionsData?.collections || [];
  const totalPages = collectionsData?.totalPages || 1;

  // Sort collections
  const sortedCollections = useMemo(() => {
    const sorted = [...collections];
    switch (sortBy) {
      case 'name':
        return sorted.sort((a, b) => a.name.localeCompare(b.name));
      case 'name-desc':
        return sorted.sort((a, b) => b.name.localeCompare(a.name));
      case 'in-library':
        return sorted.sort((a, b) => {
          const aInLib = isInLibrary(a.tmdb_id) ? 1 : 0;
          const bInLib = isInLibrary(b.tmdb_id) ? 1 : 0;
          return bInLib - aInLib; // In library first
        });
      case 'recent':
      default:
        return sorted; // Keep original order (most recent/popular from TMDB)
    }
  }, [collections, sortBy, libraryCollectionIds, newlyAddedIds]);

  return (
    <div className="min-h-screen bg-[#141414] text-white">
      {/* Header */}
      <div className="bg-gradient-to-b from-slate-900 to-[#141414] px-8 pt-8 pb-6">
        <div className="flex items-center gap-4 mb-6">
          <Link 
            to="/search" 
            className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </Link>
          <div className="flex items-center gap-3">
            <Layers className="w-8 h-8 text-cyan-500" />
            <h1 className="text-3xl font-bold">Browse Collections</h1>
          </div>
        </div>

        {/* Search Bar */}
        <form onSubmit={handleSearch} className="max-w-xl">
          <div className="relative">
            <SearchIcon className="absolute left-4 top-1/2 transform -translate-y-1/2 w-5 h-5 text-slate-400" />
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search collections..."
              className="w-full pl-12 pr-4 py-3 bg-slate-800 border border-slate-700 rounded-lg 
                       text-white placeholder-slate-400 focus:outline-none focus:border-cyan-500
                       transition-colors"
            />
            <button
              type="submit"
              className="absolute right-2 top-1/2 transform -translate-y-1/2 px-4 py-1.5 
                       bg-cyan-600 hover:bg-cyan-500 rounded-md text-sm font-medium transition-colors"
            >
              Search
            </button>
          </div>
        </form>

        {/* Sort Dropdown */}
        <div className="flex items-center gap-3 mt-4">
          <SlidersHorizontal className="w-4 h-4 text-slate-400" />
          <span className="text-sm text-slate-400">Sort by:</span>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as any)}
            className="bg-slate-800 border border-slate-700 rounded-lg px-3 py-1.5 text-sm text-white
                     focus:outline-none focus:border-cyan-500 cursor-pointer"
          >
            <option value="name">Name (A-Z)</option>
            <option value="name-desc">Name (Z-A)</option>
            <option value="in-library">In Library First</option>
            <option value="recent">Default</option>
          </select>
        </div>
      </div>

      {/* Error Toast */}
      {errorMessage && (
        <div className="fixed top-20 right-4 z-50 flex items-center gap-3 bg-red-900/90 border border-red-700 
                       text-white px-4 py-3 rounded-lg shadow-lg max-w-md animate-in slide-in-from-right">
          <AlertCircle className="w-5 h-5 text-red-400 flex-shrink-0" />
          <p className="text-sm">{errorMessage}</p>
          <button onClick={() => setErrorMessage(null)} className="text-red-400 hover:text-white">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Collections Grid */}
      <div className="px-8 pb-8">
        {isLoading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="w-8 h-8 animate-spin text-cyan-500" />
          </div>
        ) : collections.length === 0 ? (
          <div className="text-center py-20 text-slate-400">
            <Layers className="w-16 h-16 mx-auto mb-4 opacity-50" />
            <p className="text-lg">No collections found</p>
            {searchQuery && (
              <button 
                onClick={() => { setSearchInput(''); setSearchParams({ page: '1' }); }}
                className="mt-4 text-cyan-500 hover:text-cyan-400"
              >
                Clear search
              </button>
            )}
          </div>
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-6">
              {sortedCollections.map((collection: Collection) => {
                const inLibrary = isInLibrary(collection.tmdb_id);
                const isAdding = addingCollectionId === collection.tmdb_id;
                
                return (
                  <div key={collection.tmdb_id} className="group">
                    <div className="relative aspect-[2/3] rounded-lg overflow-hidden bg-slate-800 mb-2">
                      {collection.poster_path ? (
                        <img
                          src={tmdbImageUrl(collection.poster_path, 'w342')}
                          alt={collection.name}
                          className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center bg-gradient-to-br from-cyan-900 to-slate-900">
                          <Layers className="w-12 h-12 text-cyan-500/50" />
                        </div>
                      )}
                      
                      {/* Gradient overlay */}
                      <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-transparent to-transparent" />
                      
                      {/* Collection badge */}
                      <div className="absolute top-2 left-2">
                        <span className="px-1.5 py-0.5 rounded text-[10px] font-bold bg-cyan-600 text-white">
                          COLLECTION
                        </span>
                      </div>

                      {/* In Library badge */}
                      {inLibrary && (
                        <div className="absolute top-2 right-2">
                          <span className="px-1.5 py-0.5 rounded text-[10px] font-bold bg-green-600 text-white flex items-center gap-0.5">
                            <Check className="w-3 h-3" />
                            ADDED
                          </span>
                        </div>
                      )}

                      {/* Add button overlay */}
                      <div className="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity bg-black/40">
                        {inLibrary ? (
                          <span className="flex items-center gap-2 text-green-400 bg-black/50 px-4 py-2 rounded-full">
                            <CheckCircle className="w-5 h-5" />
                            In Library
                          </span>
                        ) : (
                          <button
                            onClick={() => handleAddCollection(collection)}
                            disabled={isAdding}
                            className="flex items-center gap-2 bg-cyan-600 hover:bg-cyan-500 px-4 py-2 
                                     rounded-full font-medium transition-colors disabled:opacity-50"
                          >
                            {isAdding ? (
                              <Loader2 className="w-5 h-5 animate-spin" />
                            ) : (
                              <Plus className="w-5 h-5" />
                            )}
                            Add to Library
                          </button>
                        )}
                      </div>
                    </div>

                    <h3 className="text-white text-sm font-medium line-clamp-2">{collection.name}</h3>
                  </div>
                );
              })}
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex items-center justify-center gap-4 mt-8">
                <button
                  onClick={() => handlePageChange(page - 1)}
                  disabled={page <= 1}
                  className="flex items-center gap-2 px-4 py-2 bg-slate-800 hover:bg-slate-700 
                           rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  <ChevronLeft className="w-5 h-5" />
                  Previous
                </button>
                
                <div className="flex items-center gap-2">
                  {Array.from({ length: Math.min(totalPages, 10) }, (_, i) => {
                    let pageNum: number;
                    if (totalPages <= 10) {
                      pageNum = i + 1;
                    } else if (page <= 5) {
                      pageNum = i + 1;
                    } else if (page >= totalPages - 4) {
                      pageNum = totalPages - 9 + i;
                    } else {
                      pageNum = page - 4 + i;
                    }
                    
                    return (
                      <button
                        key={pageNum}
                        onClick={() => handlePageChange(pageNum)}
                        className={`w-10 h-10 rounded-lg font-medium transition-colors
                          ${page === pageNum 
                            ? 'bg-cyan-600 text-white' 
                            : 'bg-slate-800 hover:bg-slate-700 text-slate-300'}`}
                      >
                        {pageNum}
                      </button>
                    );
                  })}
                </div>

                <button
                  onClick={() => handlePageChange(page + 1)}
                  disabled={page >= totalPages}
                  className="flex items-center gap-2 px-4 py-2 bg-slate-800 hover:bg-slate-700 
                           rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Next
                  <ChevronRight className="w-5 h-5" />
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
