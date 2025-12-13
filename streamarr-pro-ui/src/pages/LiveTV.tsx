import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { streamarrApi } from '../services/api';
import ChannelCard from '../components/ChannelCard';
import { Search, Radio, Filter, ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, Clock, Tv, Calendar as CalendarIcon } from 'lucide-react';
import type { Channel, EPGProgram } from '../types';

const CHANNELS_PER_PAGE = 50;

export default function LiveTV() {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [currentPage, setCurrentPage] = useState(1);
  const [viewMode, setViewMode] = useState<'list' | 'grid' | 'epg'>('grid');
  const [selectedChannel, setSelectedChannel] = useState<Channel | null>(null);

  const { data: channels, isLoading } = useQuery({
    queryKey: ['channels', selectedCategory],
    queryFn: () => {
      const category = selectedCategory === 'all' ? undefined : selectedCategory;
      return streamarrApi.getChannels({ category }).then(res => res.data);
    },
  });

  const { data: epgData } = useQuery({
    queryKey: ['epg', selectedChannel?.id],
    queryFn: () => streamarrApi.getEPG(selectedChannel!.id).then(res => res.data),
    enabled: !!selectedChannel && viewMode === 'epg',
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

  const getCurrentProgram = (channelId: number): EPGProgram | undefined => {
    if (!epgData) return undefined;
    const now = new Date().toISOString();
    return (epgData as EPGProgram[]).find(
      program => program.channel_id === channelId &&
        program.start_time <= now &&
        program.end_time >= now
    );
  };

  const getUpcomingPrograms = (channelId: number): EPGProgram[] => {
    if (!epgData) return [];
    const now = new Date().toISOString();
    return (epgData as EPGProgram[])
      .filter(program => program.channel_id === channelId && program.start_time > now)
      .slice(0, 5);
  };

  const formatTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
  };

  const handleChannelClick = (id: number) => {
    // Stream URL can be accessed via /api/v1/channels/{id}/stream
    const channel = channels?.find(ch => ch.id === id);
    if (channel) {
      console.log('Channel stream URL:', channel.stream_url);
    }
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
        
        <div className="flex items-center gap-4">
          {/* View Mode Toggle */}
          <div className="flex items-center gap-2 bg-slate-800 rounded-lg p-1">
            <button
              onClick={() => setViewMode('grid')}
              className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                viewMode === 'grid'
                  ? 'bg-blue-600 text-white'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              Grid
            </button>
            <button
              onClick={() => setViewMode('list')}
              className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                viewMode === 'list'
                  ? 'bg-blue-600 text-white'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              List
            </button>
            <button
              onClick={() => setViewMode('epg')}
              className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                viewMode === 'epg'
                  ? 'bg-blue-600 text-white'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              EPG Guide
            </button>
          </div>

          <div className="flex items-center gap-2 text-sm">
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
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
          {paginatedChannels.map((channel) => {
            const currentProgram = getCurrentProgram(channel.id);
            return (
              <div
                key={channel.id}
                onClick={() => handleChannelClick(channel.id)}
                className="group bg-slate-800 rounded-lg overflow-hidden cursor-pointer hover:ring-2 hover:ring-blue-500 transition-all"
              >
                <div className="aspect-video bg-slate-900 relative overflow-hidden">
                  {channel.logo ? (
                    <img
                      src={channel.logo}
                      alt={channel.name}
                      className="w-full h-full object-contain p-4 group-hover:scale-110 transition-transform"
                    />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center">
                      <Tv className="w-16 h-16 text-slate-600" />
                    </div>
                  )}
                  {channel.active && (
                    <div className="absolute top-2 right-2 bg-red-500 text-white text-xs font-bold px-2 py-1 rounded">
                      LIVE
                    </div>
                  )}
                </div>
                <div className="p-3">
                  <h3 className="text-white font-medium mb-1 truncate">{channel.name}</h3>
                  {currentProgram ? (
                    <div className="text-xs text-slate-400">
                      <div className="flex items-center gap-1 mb-1">
                        <Clock className="w-3 h-3" />
                        <span>{formatTime(currentProgram.start_time)} - {formatTime(currentProgram.end_time)}</span>
                      </div>
                      <div className="truncate">{currentProgram.title}</div>
                    </div>
                  ) : (
                    <div className="text-xs text-slate-500">No program info</div>
                  )}
                </div>
              </div>
            );
          })}
        : (
        /* EPG Guide View */
        <div className="bg-slate-800 rounded-lg overflow-hidden">
          <div className="p-6 border-b border-slate-700">
            <h2 className="text-xl font-semibold text-white mb-4 flex items-center gap-2">
              <CalendarIcon className="w-6 h-6 text-blue-500" />
              Program Guide
            </h2>
            <p className="text-slate-400 text-sm">
              Select a channel to view its program schedule
            </p>
          </div>
          
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 p-6">
            {/* Channel List */}
            <div className="lg:col-span-1 space-y-2 max-h-[600px] overflow-y-auto">
              {paginatedChannels.map((channel) => (
                <div
                  key={channel.id}
                  onClick={() => setSelectedChannel(channel)}
                  className={`p-3 rounded-lg cursor-pointer transition-colors ${
                    selectedChannel?.id === channel.id
                      ? 'bg-blue-600 text-white'
                      : 'bg-slate-700 hover:bg-slate-600'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <div className="w-12 h-9 bg-slate-900 rounded flex-shrink-0 overflow-hidden">
                      {channel.logo ? (
                        <img
                          src={channel.logo}
                          alt={channel.name}
                          className="w-full h-full object-contain"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center">
                          <Tv className="w-6 h-6 text-slate-600" />
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="font-medium truncate">{channel.name}</div>
                      <div className="text-xs opacity-75">{channel.category}</div>
                    </div>
                    {channel.active && (
                      <div className="flex-shrink-0">
                        <span className="inline-block w-2 h-2 bg-red-500 rounded-full"></span>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>

            {/* Program Schedule */}
            <div className="lg:col-span-2">
              {selectedChannel ? (
                <div className="space-y-4">
                  <h3 className="text-lg font-semibold text-white mb-4">{selectedChannel.name}</h3>

                  {/* Current Program */}
                  {getCurrentProgram(selectedChannel.id) && (
                    <div className="bg-blue-900/30 border border-blue-700 rounded-lg p-4">
                      <div className="flex items-center gap-2 text-blue-400 text-sm font-medium mb-2">
                        <span className="relative flex h-2 w-2">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-blue-500"></span>
                        </span>
                        NOW PLAYING
                      </div>
                      <h4 className="text-white font-semibold text-lg mb-2">
                        {getCurrentProgram(selectedChannel.id)!.title}
                      </h4>
                      <div className="flex items-center gap-2 text-slate-400 text-sm mb-3">
                        <Clock className="w-4 h-4" />
                        <span>
                          {formatTime(getCurrentProgram(selectedChannel.id)!.start_time)} -{' '}
                          {formatTime(getCurrentProgram(selectedChannel.id)!.end_time)}
                        </span>
                      </div>
                      <p className="text-slate-300 text-sm">
                        {getCurrentProgram(selectedChannel.id)!.description || 'No description available'}
                      </p>
                    </div>
                  )}

                  {/* Upcoming Programs */}
                  <div>
                    <h4 className="text-white font-medium mb-3">Upcoming Programs</h4>
                    <div className="space-y-3">
                      {getUpcomingPrograms(selectedChannel.id).length > 0 ? (
                        getUpcomingPrograms(selectedChannel.id).map((program) => (
                          <div
                            key={program.id}
                            className="bg-slate-700 rounded-lg p-4 hover:bg-slate-600 transition-colors"
                          >
                            <div className="flex items-start justify-between mb-2">
                              <h5 className="text-white font-medium">{program.title}</h5>
                              <div className="text-slate-400 text-sm whitespace-nowrap ml-4">
                                {formatTime(program.start_time)}
                              </div>
                            </div>
                            <p className="text-slate-400 text-sm line-clamp-2">
                              {program.description || 'No description available'}
                            </p>
                          </div>
                        ))
                      ) : (
                        <div className="text-center py-8 text-slate-400">
                          <CalendarIcon className="w-12 h-12 mx-auto mb-2 opacity-50" />
                          <p>No upcoming programs available</p>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="text-center py-20 text-slate-400">
                  <Tv className="w-16 h-16 mx-auto mb-4 opacity-50" />
                  <p>Select a channel to view program schedule</p>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Pagination Controls */}
      {filteredChannels.length > CHANNELS_PER_PAGE && viewMode !== 'epg' && (
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
