import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { streamarrApi } from '../services/api';
import { 
  Search, Radio, Filter, Play, ChevronLeft, ChevronRight, 
  Loader2, ExternalLink, Tv, X, Grid3X3, Calendar
} from 'lucide-react';
import type { Channel } from '../types';
import TVGuide from '../components/TVGuide';

const CHANNELS_PER_PAGE = 50;

type ViewMode = 'grid' | 'guide';

export default function LiveTV() {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [currentPage, setCurrentPage] = useState(1);
  const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null);
  const [viewMode, setViewMode] = useState<ViewMode>('grid');

  const { data: channels = [], isLoading } = useQuery({
    queryKey: ['channels', selectedCategory],
    queryFn: () => {
      const category = selectedCategory === 'all' ? undefined : selectedCategory;
      return streamarrApi.getChannels({ category }).then(res => res.data || []);
    },
  });

  // Fetch categories from the API
  const { data: categoriesData } = useQuery({
    queryKey: ['channelCategories'],
    queryFn: () => streamarrApi.getChannelCategories().then(res => res.data),
  });

  const categories = ['all', ...(categoriesData?.categories || [])];

  const filteredChannels = channels.filter((ch: Channel) =>
    ch.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const totalPages = Math.ceil(filteredChannels.length / CHANNELS_PER_PAGE);
  
  const paginatedChannels = useMemo(() => {
    const startIndex = (currentPage - 1) * CHANNELS_PER_PAGE;
    return filteredChannels.slice(startIndex, startIndex + CHANNELS_PER_PAGE);
  }, [filteredChannels, currentPage]);

  const handleSearchChange = (value: string) => {
    setSearchQuery(value);
    setCurrentPage(1);
  };

  const handleCategoryChange = (value: string) => {
    setSelectedCategory(value);
    setCurrentPage(1);
  };

  const handleChannelClick = (channelOrId: Channel | string) => {
    if (typeof channelOrId === 'string') {
      // Coming from TV Guide - find the channel by ID (compare as strings)
      const channel = channels.find((ch: Channel) => String(ch.id) === channelOrId);
      if (channel) {
        setSelectedChannel(channel);
      }
    } else {
      setSelectedChannel(channelOrId);
    }
  };

  // Group channels by category for display
  const channelsByCategory = useMemo(() => {
    if (searchQuery || selectedCategory !== 'all') {
      return { [selectedCategory === 'all' ? 'Search Results' : selectedCategory]: paginatedChannels };
    }
    
    const grouped: Record<string, Channel[]> = {};
    channels.forEach((ch: Channel) => {
      const cat = ch.category || 'Other';
      if (!grouped[cat]) grouped[cat] = [];
      if (grouped[cat].length < 20) grouped[cat].push(ch);
    });
    return grouped;
  }, [channels, paginatedChannels, searchQuery, selectedCategory]);

  return (
    <div className="min-h-screen bg-[#141414] -m-6">
      {/* Hero Section */}
      <div className="relative h-[35vh] min-h-[250px] flex items-end pb-8 px-8">
        <div className="absolute inset-0 bg-gradient-to-b from-purple-900/30 via-[#141414]/50 to-[#141414]" />
        <div className="absolute inset-0 bg-gradient-to-r from-[#141414] via-transparent to-[#141414]" />
        
        <div className="relative z-10 w-full">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-3 rounded-full bg-purple-600">
              <Radio className="w-8 h-8 text-white" />
            </div>
            <div>
              <h1 className="text-4xl font-black text-white">Live TV</h1>
              <p className="text-slate-400 mt-1">
                {filteredChannels.length} channels available
                <span className="ml-3 inline-flex items-center gap-1">
                  <span className="relative flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-purple-400 opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-purple-500"></span>
                  </span>
                  <span className="text-purple-400">{channels.filter((ch: Channel) => ch.active).length} Live</span>
                </span>
              </p>
            </div>
          </div>

          {/* Search, Filter, and View Toggle */}
          <div className="flex gap-4 max-w-4xl">
            <div className="flex-1 relative">
              <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-400" />
              <input
                type="text"
                placeholder="Search channels..."
                value={searchQuery}
                onChange={(e) => handleSearchChange(e.target.value)}
                className="w-full pl-12 pr-4 py-3 bg-[#333] border-2 border-transparent rounded-lg text-white 
                           placeholder-slate-500 focus:outline-none focus:border-white/30 transition-all"
              />
            </div>

            <div className="relative">
              <select
                value={selectedCategory}
                onChange={(e) => handleCategoryChange(e.target.value)}
                className="appearance-none px-4 py-3 pr-10 bg-[#333] text-white rounded-lg border-2 border-transparent
                           focus:outline-none focus:border-white/30 cursor-pointer min-w-[150px]"
              >
                {categories.map(cat => (
                  <option key={cat} value={cat}>
                    {cat === 'all' ? 'All Categories' : cat}
                  </option>
                ))}
              </select>
              <Filter className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400 pointer-events-none" />
            </div>

            {/* View Mode Toggle */}
            <div className="flex rounded-lg overflow-hidden border-2 border-transparent bg-[#333]">
              <button
                onClick={() => setViewMode('grid')}
                className={`px-4 py-3 flex items-center gap-2 transition-colors ${
                  viewMode === 'grid' 
                    ? 'bg-purple-600 text-white' 
                    : 'text-slate-400 hover:text-white hover:bg-white/10'
                }`}
              >
                <Grid3X3 className="w-4 h-4" />
                <span className="hidden sm:inline">Grid</span>
              </button>
              <button
                onClick={() => setViewMode('guide')}
                className={`px-4 py-3 flex items-center gap-2 transition-colors ${
                  viewMode === 'guide' 
                    ? 'bg-purple-600 text-white' 
                    : 'text-slate-400 hover:text-white hover:bg-white/10'
                }`}
              >
                <Calendar className="w-4 h-4" />
                <span className="hidden sm:inline">Guide</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="px-8 pb-12">
        {isLoading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="w-10 h-10 animate-spin text-purple-600" />
          </div>
        ) : filteredChannels.length === 0 ? (
          <div className="text-center py-20">
            <Radio className="w-20 h-20 text-slate-600 mx-auto mb-4" />
            <h3 className="text-2xl font-bold text-white mb-2">
              {searchQuery ? 'No channels found' : 'No channels available'}
            </h3>
            <p className="text-slate-400">
              {searchQuery ? 'Try a different search term' : 'Configure your IPTV sources in Settings'}
            </p>
          </div>
        ) : viewMode === 'guide' ? (
          /* TV Guide View */
          <TVGuide 
            category={selectedCategory} 
            onChannelClick={handleChannelClick}
          />
        ) : (
          /* Grid View */
          <>
            {Object.entries(channelsByCategory).map(([category, categoryChannels]) => (
              <ChannelRow
                key={category}
                title={category}
                channels={categoryChannels}
                onChannelClick={handleChannelClick}
              />
            ))}

            {/* Pagination */}
            {(searchQuery || selectedCategory !== 'all') && totalPages > 1 && (
              <div className="flex items-center justify-center gap-4 mt-8">
                <button
                  onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                  className="p-2 rounded-lg bg-[#333] text-white disabled:opacity-50 hover:bg-[#444] transition-colors"
                >
                  <ChevronLeft className="w-5 h-5" />
                </button>
                <span className="text-white">
                  Page {currentPage} of {totalPages}
                </span>
                <button
                  onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                  disabled={currentPage === totalPages}
                  className="p-2 rounded-lg bg-[#333] text-white disabled:opacity-50 hover:bg-[#444] transition-colors"
                >
                  <ChevronRight className="w-5 h-5" />
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {/* Channel Detail Modal */}
      {selectedChannel && (
        <ChannelModal 
          channel={selectedChannel} 
          onClose={() => setSelectedChannel(null)}
        />
      )}
    </div>
  );
}

// Channel Row Component
function ChannelRow({ 
  title, 
  channels, 
  onChannelClick 
}: { 
  title: string; 
  channels: Channel[]; 
  onChannelClick: (channel: Channel) => void;
}) {
  if (channels.length === 0) return null;

  return (
    <div className="mb-8">
      <h2 className="text-xl font-bold text-white mb-4">{title}</h2>
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 gap-4">
        {channels.map((channel) => (
          <ChannelCard 
            key={channel.id} 
            channel={channel} 
            onClick={() => onChannelClick(channel)} 
          />
        ))}
      </div>
    </div>
  );
}

// Channel Card Component
function ChannelCard({ channel, onClick }: { channel: Channel; onClick: () => void }) {
  return (
    <div
      onClick={onClick}
      className="bg-[#1e1e1e] rounded-xl overflow-hidden cursor-pointer group hover:bg-[#282828] 
                 hover:scale-105 transition-all duration-200"
    >
      <div className="relative aspect-video bg-gradient-to-br from-slate-700 to-slate-900 flex items-center justify-center">
        {channel.logo ? (
          <img
            src={channel.logo}
            alt={channel.name}
            className="w-full h-full object-contain p-4"
            onError={(e) => {
              (e.target as HTMLImageElement).style.display = 'none';
            }}
          />
        ) : (
          <Tv className="w-12 h-12 text-slate-600" />
        )}
        
        {/* Play overlay */}
        <div className="absolute inset-0 bg-black/60 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity">
          <div className="p-3 rounded-full bg-purple-600">
            <Play className="w-6 h-6 text-white fill-white" />
          </div>
        </div>

        {/* Live indicator */}
        {channel.active && (
          <div className="absolute top-2 right-2 flex items-center gap-1 px-2 py-0.5 bg-purple-600 rounded text-xs font-bold text-white">
            <span className="w-1.5 h-1.5 rounded-full bg-white animate-pulse" />
            LIVE
          </div>
        )}
      </div>

      <div className="p-3">
        <h3 className="text-white font-medium text-sm truncate">{channel.name}</h3>
        <p className="text-slate-500 text-xs truncate">{channel.category || 'Uncategorized'}</p>
      </div>
    </div>
  );
}

// Channel Modal Component
function ChannelModal({ channel, onClose }: { channel: Channel; onClose: () => void }) {
  const handlePlay = () => {
    if (channel.stream_url) {
      window.open(channel.stream_url, '_blank');
    }
    onClose();
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4" onClick={onClose}>
      <div 
        className="bg-[#181818] rounded-xl w-full max-w-lg overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="relative aspect-video bg-gradient-to-br from-slate-700 to-slate-900 flex items-center justify-center">
          {channel.logo ? (
            <img
              src={channel.logo}
              alt={channel.name}
              className="w-full h-full object-contain p-8"
            />
          ) : (
            <Tv className="w-24 h-24 text-slate-600" />
          )}
          
          <button
            onClick={onClose}
            className="absolute top-4 right-4 p-2 rounded-full bg-black/50 hover:bg-black/70 transition-colors"
          >
            <X className="w-5 h-5 text-white" />
          </button>

          {channel.active && (
            <div className="absolute top-4 left-4 flex items-center gap-1 px-3 py-1 bg-purple-600 rounded text-sm font-bold text-white">
              <span className="w-2 h-2 rounded-full bg-white animate-pulse" />
              LIVE
            </div>
          )}
        </div>

        {/* Content */}
        <div className="p-6">
          <h2 className="text-2xl font-bold text-white mb-2">{channel.name}</h2>
          <p className="text-slate-400 mb-6">{channel.category || 'Uncategorized'}</p>

          <div className="flex gap-3">
            <button
              onClick={handlePlay}
              className="flex-1 flex items-center justify-center gap-2 px-6 py-3 bg-purple-600 hover:bg-purple-700 
                         text-white font-bold rounded-lg transition-colors"
            >
              <Play className="w-5 h-5 fill-white" />
              Watch Now
            </button>
            <button
              onClick={() => channel.stream_url && navigator.clipboard.writeText(channel.stream_url)}
              className="px-4 py-3 bg-[#333] hover:bg-[#444] text-white rounded-lg transition-colors"
              title="Copy stream URL"
            >
              <ExternalLink className="w-5 h-5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
