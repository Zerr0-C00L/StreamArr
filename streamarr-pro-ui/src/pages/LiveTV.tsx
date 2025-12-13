import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { streamarrApi } from '../services/api';
import ChannelCard from '../components/ChannelCard';
import { Search, Radio, Filter, ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react';

const CHANNELS_PER_PAGE = 50;

export default function LiveTV() {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [currentPage, setCurrentPage] = useState(1);

  const { data: channels, isLoading } = useQuery({
    queryKey: ['channels', selectedCategory],
    queryFn: () => {
      const category = selectedCategory === 'all' ? undefined : selectedCategory;
      return streamarrApi.getChannels({ category }).then(res => res.data);
    },
  });

  const filteredChannels = channels?.filter(ch =>
    ch.name.toLowerCase().includes(searchQuery.toLowerCase())
  ) || [];

  // Pagination calculations
  const totalPages = Math.ceil(filteredChannels.length / CHANNELS_PER_PAGE);
  const paginatedChannels = useMemo(() => {
    const startIndex = (currentPage - 1) * CHANNELS_PER_PAGE;
    return filteredChannels.slice(startIndex, startIndex + CHANNELS_PER_PAGE);
  }, [filteredChannels, currentPage]);

  // Reset to page 1 when search or category changes
  const handleSearchChange = (value: string) => {
    setSearchQuery(value);
    setCurrentPage(1);
  };

  const handleCategoryChange = (value: string) => {
    setSelectedCategory(value);
    setCurrentPage(1);
  };

  // Get unique categories
  const categories = ['all', ...new Set(channels?.map(ch => ch.category) || [])];

  const handleChannelClick = (id: number) => {
    // TODO: Open video player modal
    console.log('Channel clicked:', id);
  };

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-white mb-2 flex items-center gap-3">
            <Radio className="w-8 h-8 text-red-500" />
            Live TV
          </h1>
          <p className="text-slate-400">
            {filteredChannels.length} channels available
          </p>
        </div>
        
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-2 text-sm">
            <span className="relative flex h-3 w-3">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-3 w-3 bg-red-500"></span>
            </span>
            <span className="text-red-500 font-medium">
              {channels?.filter(ch => ch.active).length || 0} LIVE
            </span>
          </div>
        </div>
      </div>

      {/* Search and Category Filter */}
      <div className="flex gap-4 mb-6">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-slate-400 w-5 h-5" />
          <input
            type="text"
            placeholder="Search channels..."
            value={searchQuery}
            onChange={(e) => handleSearchChange(e.target.value)}
            className="input w-full pl-10"
          />
        </div>

        <div className="flex items-center gap-2">
          <Filter className="w-5 h-5 text-slate-400" />
          <select
            value={selectedCategory}
            onChange={(e) => handleCategoryChange(e.target.value)}
            className="input"
          >
            {categories.map(cat => (
              <option key={cat} value={cat}>
                {cat === 'all' ? 'All Categories' : cat}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Channels List */}
      {isLoading ? (
        <div className="text-center py-20">
          <div className="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-primary-500"></div>
          <p className="text-slate-400 mt-4">Loading channels...</p>
        </div>
      ) : filteredChannels.length === 0 ? (
        <div className="text-center py-20">
          <Radio className="w-24 h-24 text-slate-600 mx-auto mb-4" />
          <h3 className="text-xl font-semibold text-white mb-2">
            {searchQuery ? 'No channels found' : 'No channels available'}
          </h3>
          <p className="text-slate-400 mb-6">
            {searchQuery
              ? 'Try adjusting your search or category filter'
              : 'Configure your IPTV sources to start watching live TV'}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {paginatedChannels.map((channel) => (
            <ChannelCard
              key={channel.id}
              channel={channel}
              onClick={() => handleChannelClick(channel.id)}
            />
          ))}
        </div>
      )}

      {/* Pagination Controls */}
      {filteredChannels.length > CHANNELS_PER_PAGE && (
        <div className="mt-6 flex items-center justify-between">
          <div className="text-sm text-slate-400">
            Showing {((currentPage - 1) * CHANNELS_PER_PAGE) + 1} to{' '}
            {Math.min(currentPage * CHANNELS_PER_PAGE, filteredChannels.length)} of{' '}
            {filteredChannels.length} channels
          </div>
          
          <div className="flex items-center gap-2">
            <button
              onClick={() => setCurrentPage(1)}
              disabled={currentPage === 1}
              className="p-2 rounded-lg bg-slate-700/50 text-slate-300 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              title="First page"
            >
              <ChevronsLeft className="w-5 h-5" />
            </button>
            
            <button
              onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
              disabled={currentPage === 1}
              className="p-2 rounded-lg bg-slate-700/50 text-slate-300 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              title="Previous page"
            >
              <ChevronLeft className="w-5 h-5" />
            </button>
            
            <div className="flex items-center gap-2">
              {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                let pageNum;
                if (totalPages <= 5) {
                  pageNum = i + 1;
                } else if (currentPage <= 3) {
                  pageNum = i + 1;
                } else if (currentPage >= totalPages - 2) {
                  pageNum = totalPages - 4 + i;
                } else {
                  pageNum = currentPage - 2 + i;
                }
                
                return (
                  <button
                    key={pageNum}
                    onClick={() => setCurrentPage(pageNum)}
                    className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                      currentPage === pageNum
                        ? 'bg-blue-600 text-white'
                        : 'bg-slate-700/50 text-slate-300 hover:bg-slate-700'
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
              className="p-2 rounded-lg bg-slate-700/50 text-slate-300 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              title="Next page"
            >
              <ChevronRight className="w-5 h-5" />
            </button>
            
            <button
              onClick={() => setCurrentPage(totalPages)}
              disabled={currentPage === totalPages}
              className="p-2 rounded-lg bg-slate-700/50 text-slate-300 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              title="Last page"
            >
              <ChevronsRight className="w-5 h-5" />
            </button>
          </div>
        </div>
      )}

      {/* Stats Footer */}
      {filteredChannels.length > 0 && (
        <div className="mt-8 flex items-center justify-between text-sm text-slate-400">
          <div>
            Showing {filteredChannels.length} of {channels?.length || 0} channels
          </div>
          <div className="flex gap-4">
            <span>Categories: {categories.length - 1}</span>
            <span>Active: {channels?.filter(ch => ch.active).length || 0}</span>
          </div>
        </div>
      )}
    </div>
  );
}
