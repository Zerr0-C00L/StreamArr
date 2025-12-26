import type { Channel } from '../types';
import { Play, Radio } from 'lucide-react';

interface ChannelCardProps {
  channel: Channel;
  onClick: () => void;
}

export default function ChannelCard({ channel, onClick }: ChannelCardProps) {
  return (
    <div
      className="card p-4 cursor-pointer hover:ring-2 hover:ring-primary-500 transition-all"
      onClick={onClick}
    >
      <div className="flex items-center gap-4">
        {/* Channel Logo */}
        <div className="w-16 h-16 bg-slate-700 rounded-lg flex items-center justify-center flex-shrink-0 overflow-hidden">
          {channel.logo ? (
            <img 
              src={channel.logo} 
              alt={channel.name}
              className="w-full h-full object-contain"
            />
          ) : (
            <Radio className="w-8 h-8 text-slate-400" />
          )}
        </div>

        {/* Channel Info */}
        <div className="flex-1 min-w-0">
          <h3 className="text-white font-semibold text-lg truncate mb-1">
            {channel.name}
          </h3>
          
          <div className="flex items-center gap-2 text-sm text-slate-400">
            <span className="px-2 py-0.5 bg-slate-700 rounded text-xs">
              {channel.category}
            </span>
            {channel.language && (
              <span className="text-xs">{channel.language}</span>
            )}
            {channel.country && (
              <>
                <span>•</span>
                <span className="text-xs">{channel.country}</span>
              </>
            )}
          </div>
        </div>

        {/* Play Button */}
        <button className="btn btn-primary flex items-center gap-2">
          <Play className="w-4 h-4" />
          Watch
        </button>
      </div>

      {/* Live Indicator */}
      {channel.active && (
        <div className="mt-3 flex items-center gap-2 text-sm">
          <div className="flex items-center gap-1 text-red-500">
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-2 w-2 bg-red-500"></span>
            </span>
            <span className="font-medium">LIVE</span>
          </div>
        </div>
      )}
    </div>
  );
}
