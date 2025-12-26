import { useState, useRef, useEffect, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { streamarrApi } from '../services/api';
import { 
  Play, ChevronLeft, ChevronRight, Tv, Clock, 
  Loader2, RefreshCw
} from 'lucide-react';
import type { GuideChannel, GuideProgram, NowPlaying } from '../types';

interface TVGuideProps {
  category?: string;
  onChannelClick?: (channelId: string) => void;
}

export default function TVGuide({ category, onChannelClick }: TVGuideProps) {
  const gridRef = useRef<HTMLDivElement>(null);
  const [scrollPosition, setScrollPosition] = useState(0);

  const { data: guideData, isLoading, refetch } = useQuery({
    queryKey: ['tvguide', category],
    queryFn: () => streamarrApi.getTVGuide({ 
      category: category === 'all' ? undefined : category, 
      limit: 30, 
      hours: 6 
    }).then(res => res.data),
    refetchInterval: 60000, // Refresh every minute
  });

  const timeSlots = useMemo(() => {
    if (!guideData?.time_slots) return [];
    return guideData.time_slots.map(slot => new Date(slot));
  }, [guideData?.time_slots]);

  const startTime = guideData?.start_time ? new Date(guideData.start_time) : new Date();
  const endTime = guideData?.end_time ? new Date(guideData.end_time) : new Date();
  const totalMinutes = (endTime.getTime() - startTime.getTime()) / (1000 * 60);
  const pixelsPerMinute = 4; // 4px per minute = 120px per 30 minutes

  // Auto-scroll to current time on load
  useEffect(() => {
    if (gridRef.current && guideData) {
      const now = new Date();
      const minutesFromStart = (now.getTime() - startTime.getTime()) / (1000 * 60);
      const scrollTo = Math.max(0, (minutesFromStart * pixelsPerMinute) - 200);
      gridRef.current.scrollLeft = scrollTo;
      setScrollPosition(scrollTo);
    }
  }, [guideData, startTime]);

  const handleScroll = (direction: 'left' | 'right') => {
    if (gridRef.current) {
      const scrollAmount = 400;
      const newPosition = direction === 'left' 
        ? Math.max(0, scrollPosition - scrollAmount)
        : scrollPosition + scrollAmount;
      gridRef.current.scrollTo({ left: newPosition, behavior: 'smooth' });
      setScrollPosition(newPosition);
    }
  };

  const formatTime = (date: Date) => {
    return date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="w-10 h-10 animate-spin text-purple-500" />
      </div>
    );
  }

  if (!guideData || guideData.channels.length === 0) {
    return (
      <div className="text-center py-20">
        <Tv className="w-20 h-20 text-slate-600 mx-auto mb-4" />
        <h3 className="text-2xl font-bold text-white mb-2">No EPG Data Available</h3>
        <p className="text-slate-400">EPG data is not available for the selected channels</p>
      </div>
    );
  }

  return (
    <div className="bg-[#1a1625] rounded-2xl overflow-hidden">
      {/* Now Playing Hero */}
      {guideData.now_playing && (
        <NowPlayingHero 
          nowPlaying={guideData.now_playing} 
          onPlay={() => onChannelClick?.(guideData.now_playing!.channel_id)}
        />
      )}

      {/* TV Guide Header */}
      <div className="flex items-center justify-between px-6 py-4 border-b border-white/10">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-bold text-white">TV Guide</h2>
          <button 
            onClick={() => refetch()}
            className="p-2 rounded-lg hover:bg-white/10 transition-colors"
          >
            <RefreshCw className="w-4 h-4 text-slate-400" />
          </button>
        </div>
        
        <div className="flex items-center gap-2">
          <button
            onClick={() => handleScroll('left')}
            className="p-2 rounded-lg bg-white/10 hover:bg-white/20 transition-colors"
          >
            <ChevronLeft className="w-5 h-5 text-white" />
          </button>
          <button
            onClick={() => handleScroll('right')}
            className="p-2 rounded-lg bg-white/10 hover:bg-white/20 transition-colors"
          >
            <ChevronRight className="w-5 h-5 text-white" />
          </button>
        </div>
      </div>

      {/* Guide Grid */}
      <div className="relative">
        {/* Time Header */}
        <div className="flex sticky top-0 z-20 bg-[#1a1625]">
          {/* Channel column header */}
          <div className="w-[120px] flex-shrink-0 bg-[#1a1625] border-r border-white/10 px-3 py-2">
            <span className="text-xs text-slate-500 uppercase">Channel</span>
          </div>
          
          {/* Time slots header */}
          <div 
            ref={gridRef}
            className="flex-1 overflow-x-auto scrollbar-hide"
            onScroll={(e) => setScrollPosition((e.target as HTMLDivElement).scrollLeft)}
          >
            <div 
              className="flex border-b border-white/10"
              style={{ width: `${totalMinutes * pixelsPerMinute}px` }}
            >
              {timeSlots.map((slot, idx) => (
                <div 
                  key={idx}
                  className="flex-shrink-0 px-2 py-2 border-r border-white/5 text-center"
                  style={{ width: `${30 * pixelsPerMinute}px` }}
                >
                  <span className="text-xs text-slate-400">{formatTime(slot)}</span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Channel Rows */}
        <div className="max-h-[500px] overflow-y-auto">
          {guideData.channels.map((channel) => (
            <ChannelRow
              key={channel.id}
              channel={channel}
              startTime={startTime}
              pixelsPerMinute={pixelsPerMinute}
              totalMinutes={totalMinutes}
              scrollPosition={scrollPosition}
              onChannelClick={() => onChannelClick?.(channel.id)}
              onProgramClick={(program) => {
                if (program.is_live) {
                  onChannelClick?.(channel.id);
                }
              }}
            />
          ))}
        </div>

        {/* Current Time Indicator */}
        <CurrentTimeIndicator 
          startTime={startTime} 
          pixelsPerMinute={pixelsPerMinute}
          channelColumnWidth={120}
        />
      </div>
    </div>
  );
}

// Now Playing Hero Component
function NowPlayingHero({ nowPlaying, onPlay }: { nowPlaying: NowPlaying; onPlay: () => void }) {
  const startTime = new Date(nowPlaying.start_time);
  const endTime = new Date(nowPlaying.end_time);

  const getTimeRemaining = () => {
    const now = new Date();
    const diffMs = endTime.getTime() - now.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    if (diffMins < 1) return 'Ending soon';
    return `${diffMins}mins remaining`;
  };

  return (
    <div className="relative bg-gradient-to-r from-purple-900/50 to-indigo-900/50 p-6">
      <div className="flex items-start gap-6">
        {/* Channel Logo */}
        <div className="w-20 h-20 rounded-xl bg-white/10 flex items-center justify-center overflow-hidden flex-shrink-0">
          {nowPlaying.channel_logo ? (
            <img 
              src={nowPlaying.channel_logo} 
              alt={nowPlaying.channel_name}
              className="w-full h-full object-contain p-2"
            />
          ) : (
            <Tv className="w-10 h-10 text-slate-400" />
          )}
        </div>

        {/* Info */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <span className="px-2 py-0.5 bg-red-500 rounded text-xs font-bold text-white">LIVE</span>
            <span className="text-sm text-slate-400">{nowPlaying.channel_name}</span>
          </div>
          <h2 className="text-2xl font-bold text-white mb-2 truncate">{nowPlaying.title}</h2>
          <p className="text-slate-300 text-sm line-clamp-2 mb-3">{nowPlaying.description}</p>
          <div className="flex items-center gap-4 text-sm text-slate-400">
            <span className="flex items-center gap-1">
              <Clock className="w-4 h-4" />
              {startTime.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })} - 
              {endTime.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })}
            </span>
            <span>• {getTimeRemaining()}</span>
          </div>
          
          {/* Progress Bar */}
          <div className="mt-3 h-1 bg-white/20 rounded-full overflow-hidden">
            <div 
              className="h-full bg-purple-500 rounded-full transition-all"
              style={{ width: `${nowPlaying.progress}%` }}
            />
          </div>
        </div>

        {/* Play Button */}
        <button
          onClick={onPlay}
          className="flex-shrink-0 p-4 bg-purple-600 hover:bg-purple-700 rounded-full transition-colors"
        >
          <Play className="w-8 h-8 text-white fill-white" />
        </button>
      </div>
    </div>
  );
}

// Channel Row Component
function ChannelRow({ 
  channel, 
  startTime, 
  pixelsPerMinute, 
  totalMinutes,
  scrollPosition,
  onChannelClick,
  onProgramClick 
}: { 
  channel: GuideChannel;
  startTime: Date;
  pixelsPerMinute: number;
  totalMinutes: number;
  scrollPosition: number;
  onChannelClick: () => void;
  onProgramClick: (program: GuideProgram) => void;
}) {
  const rowRef = useRef<HTMLDivElement>(null);

  // Sync scroll with header
  useEffect(() => {
    if (rowRef.current) {
      rowRef.current.scrollLeft = scrollPosition;
    }
  }, [scrollPosition]);

  return (
    <div className="flex border-b border-white/5 hover:bg-white/5 transition-colors">
      {/* Channel Info */}
      <div 
        className="w-[120px] flex-shrink-0 p-2 border-r border-white/10 cursor-pointer"
        onClick={onChannelClick}
      >
        <div className="flex items-center gap-2">
          <div className="w-12 h-12 rounded-lg bg-white/10 flex items-center justify-center overflow-hidden flex-shrink-0">
            {channel.logo ? (
              <img 
                src={channel.logo} 
                alt={channel.name}
                className="w-full h-full object-contain p-1"
                onError={(e) => {
                  (e.target as HTMLImageElement).style.display = 'none';
                }}
              />
            ) : (
              <Tv className="w-6 h-6 text-slate-500" />
            )}
          </div>
        </div>
      </div>

      {/* Programs */}
      <div 
        ref={rowRef}
        className="flex-1 overflow-x-auto scrollbar-hide"
        style={{ pointerEvents: 'none' }}
      >
        <div 
          className="relative h-16"
          style={{ width: `${totalMinutes * pixelsPerMinute}px` }}
        >
          {channel.programs.map((program, idx) => {
            const programStart = new Date(program.start_time);
            const programEnd = new Date(program.end_time);
            const startOffset = (programStart.getTime() - startTime.getTime()) / (1000 * 60);
            const duration = (programEnd.getTime() - programStart.getTime()) / (1000 * 60);
            
            return (
              <div
                key={idx}
                className={`absolute top-1 bottom-1 rounded-lg px-2 py-1 overflow-hidden cursor-pointer transition-colors
                  ${program.is_live 
                    ? 'bg-purple-600/80 hover:bg-purple-500/80 border-2 border-purple-400' 
                    : 'bg-slate-700/80 hover:bg-slate-600/80'
                  }`}
                style={{
                  left: `${startOffset * pixelsPerMinute}px`,
                  width: `${Math.max(duration * pixelsPerMinute - 4, 40)}px`,
                  pointerEvents: 'auto',
                }}
                onClick={() => onProgramClick(program)}
                title={`${program.title}\n${new Date(program.start_time).toLocaleTimeString()} - ${new Date(program.end_time).toLocaleTimeString()}`}
              >
                <div className="text-xs font-medium text-white truncate">{program.title}</div>
                <div className="text-xs text-white/60 truncate">
                  {new Date(program.start_time).toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })} - 
                  {new Date(program.end_time).toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit' })}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// Current Time Indicator
function CurrentTimeIndicator({ 
  startTime, 
  pixelsPerMinute,
  channelColumnWidth 
}: { 
  startTime: Date; 
  pixelsPerMinute: number;
  channelColumnWidth: number;
}) {
  const [position, setPosition] = useState(0);

  useEffect(() => {
    const updatePosition = () => {
      const now = new Date();
      const minutesFromStart = (now.getTime() - startTime.getTime()) / (1000 * 60);
      setPosition(channelColumnWidth + (minutesFromStart * pixelsPerMinute));
    };
    
    updatePosition();
    const interval = setInterval(updatePosition, 60000); // Update every minute
    return () => clearInterval(interval);
  }, [startTime, pixelsPerMinute, channelColumnWidth]);

  return (
    <div 
      className="absolute top-0 bottom-0 w-0.5 bg-red-500 z-30 pointer-events-none"
      style={{ left: `${position}px` }}
    >
      <div className="absolute -top-1 left-1/2 -translate-x-1/2 w-3 h-3 bg-red-500 rounded-full" />
    </div>
  );
}
