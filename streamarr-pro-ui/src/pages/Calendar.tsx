import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import { ChevronLeft, ChevronRight, Film, Tv, Calendar as CalendarIcon, Play } from 'lucide-react';
import type { CalendarEntry } from '../types';

const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 
                'July', 'August', 'September', 'October', 'November', 'December'];

export default function Calendar() {
  const [currentDate, setCurrentDate] = useState(new Date());
  const [selectedDate, setSelectedDate] = useState<Date | null>(null);

  // Calculate start and end of current month view (including surrounding weeks)
  const { start, end } = useMemo(() => {
    const firstDay = new Date(currentDate.getFullYear(), currentDate.getMonth(), 1);
    const lastDay = new Date(currentDate.getFullYear(), currentDate.getMonth() + 1, 0);
    
    // Extend to full weeks
    const start = new Date(firstDay);
    start.setDate(start.getDate() - start.getDay());
    
    const end = new Date(lastDay);
    end.setDate(end.getDate() + (6 - end.getDay()));
    
    return {
      start: start.toISOString().split('T')[0],
      end: end.toISOString().split('T')[0],
    };
  }, [currentDate]);

  const { data: entries = [], isLoading } = useQuery({
    queryKey: ['calendar', start, end],
    queryFn: () => streamarrApi.getCalendar(start, end).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Group entries by date
  const entriesByDate = useMemo(() => {
    const grouped: Record<string, CalendarEntry[]> = {};
    const safeEntries = Array.isArray(entries) ? entries : [];
    safeEntries.forEach(entry => {
      if (entry.date) {
        const date = entry.date.split('T')[0];
        if (!grouped[date]) grouped[date] = [];
        grouped[date].push(entry);
      }
    });
    return grouped;
  }, [entries]);

  // Generate calendar days
  const calendarDays = useMemo(() => {
    const days: Date[] = [];
    const startDate = new Date(start);
    const endDate = new Date(end);
    
    for (let d = new Date(startDate); d <= endDate; d.setDate(d.getDate() + 1)) {
      days.push(new Date(d));
    }
    return days;
  }, [start, end]);

  const navigateMonth = (direction: number) => {
    setCurrentDate(prev => {
      const newDate = new Date(prev);
      newDate.setMonth(newDate.getMonth() + direction);
      return newDate;
    });
    setSelectedDate(null);
  };

  const goToToday = () => {
    setCurrentDate(new Date());
    setSelectedDate(new Date());
  };

  const isToday = (date: Date) => {
    const today = new Date();
    return date.toDateString() === today.toDateString();
  };

  const isCurrentMonth = (date: Date) => {
    return date.getMonth() === currentDate.getMonth();
  };

  const formatDateKey = (date: Date) => {
    return date.toISOString().split('T')[0];
  };

  const selectedEntries = selectedDate 
    ? entriesByDate[formatDateKey(selectedDate)] || []
    : [];

  return (
    <div className="min-h-screen bg-[#141414] -m-6 p-8">
      {/* Header */}
      <div className="max-w-7xl mx-auto">
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-4">
            <div className="w-14 h-14 rounded-xl bg-gradient-to-br from-red-600 to-red-800 flex items-center justify-center">
              <CalendarIcon className="w-7 h-7 text-white" />
            </div>
            <div>
              <h1 className="text-4xl font-black text-white">Calendar</h1>
              <p className="text-slate-400 mt-1">Upcoming releases and episodes</p>
            </div>
          </div>
          
          <div className="flex items-center gap-3">
            <button
              onClick={() => navigateMonth(-1)}
              className="w-12 h-12 rounded-xl bg-[#2a2a2a] hover:bg-white/10 text-white flex items-center justify-center transition-colors"
            >
              <ChevronLeft className="w-5 h-5" />
            </button>
            
            <button
              onClick={goToToday}
              className="px-6 py-3 rounded-xl bg-red-600 hover:bg-red-700 text-white font-bold transition-colors"
            >
              Today
            </button>
            
            <button
              onClick={() => navigateMonth(1)}
              className="w-12 h-12 rounded-xl bg-[#2a2a2a] hover:bg-white/10 text-white flex items-center justify-center transition-colors"
            >
              <ChevronRight className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Month/Year Display */}
        <div className="text-center mb-6">
          <h2 className="text-2xl font-bold text-white">
            {MONTHS[currentDate.getMonth()]} {currentDate.getFullYear()}
          </h2>
        </div>

        <div className="flex gap-6">
          {/* Calendar Grid */}
          <div className="flex-1">
            {/* Day Headers */}
            <div className="grid grid-cols-7 gap-2 mb-3">
              {DAYS.map(day => (
                <div key={day} className="text-center text-slate-400 text-sm font-semibold py-3">
                  {day}
                </div>
              ))}
            </div>

            {/* Calendar Days */}
            {isLoading ? (
              <div className="flex items-center justify-center h-96">
                <div className="flex flex-col items-center gap-4">
                  <div className="w-12 h-12 border-4 border-red-600 border-t-transparent rounded-full animate-spin" />
                  <span className="text-slate-400">Loading calendar...</span>
                </div>
              </div>
            ) : (
              <div className="grid grid-cols-7 gap-2">
                {calendarDays.map((date, idx) => {
                  const dateKey = formatDateKey(date);
                  const dayEntries = entriesByDate[dateKey] || [];
                  const isSelected = selectedDate?.toDateString() === date.toDateString();
                  
                  return (
                    <button
                      key={idx}
                      onClick={() => setSelectedDate(date)}
                      className={`
                        min-h-[110px] p-3 rounded-xl text-left transition-all
                        ${isCurrentMonth(date) ? 'bg-[#1e1e1e]' : 'bg-[#1e1e1e]/40'}
                        ${isSelected ? 'ring-2 ring-red-500 bg-red-500/10' : ''}
                        ${isToday(date) ? 'ring-2 ring-red-600' : ''}
                        hover:bg-white/5 border border-white/5
                      `}
                    >
                      <div className={`
                        text-sm font-bold mb-2
                        ${isToday(date) ? 'text-red-500' : isCurrentMonth(date) ? 'text-white' : 'text-slate-600'}
                      `}>
                        {date.getDate()}
                      </div>
                      
                      {/* Show up to 3 entries, then "+N more" */}
                      <div className="space-y-1">
                        {dayEntries.slice(0, 3).map((entry, i) => (
                          <div
                            key={i}
                            className={`
                              text-xs px-2 py-1 rounded-lg truncate font-medium
                              ${entry.type === 'movie' 
                                ? 'bg-purple-500/20 text-purple-300 border border-purple-500/20' 
                                : 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/20'}
                            `}
                            title={entry.type === 'episode' 
                              ? `${entry.series_title} S${entry.season_number}E${entry.episode_number}`
                              : entry.title
                            }
                          >
                            {entry.type === 'movie' ? (
                              <span className="flex items-center gap-1">
                                <Film className="w-3 h-3 flex-shrink-0" />
                                <span className="truncate">{entry.title}</span>
                              </span>
                            ) : (
                              <span className="flex items-center gap-1">
                                <Tv className="w-3 h-3 flex-shrink-0" />
                                <span className="truncate">{entry.series_title}</span>
                              </span>
                            )}
                          </div>
                        ))}
                        {dayEntries.length > 3 && (
                          <div className="text-xs text-slate-400 px-2 font-medium">
                            +{dayEntries.length - 3} more
                          </div>
                        )}
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          {/* Selected Day Details */}
          <div className="w-96 bg-[#1e1e1e] rounded-xl p-6 border border-white/10 h-fit sticky top-24">
            <h3 className="text-xl font-bold text-white mb-4 flex items-center gap-2">
              <CalendarIcon className="w-5 h-5 text-red-500" />
              {selectedDate 
                ? selectedDate.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' })
                : 'Select a date'
              }
            </h3>
            
            {selectedDate && selectedEntries.length === 0 && (
              <div className="text-center py-12">
                <div className="w-16 h-16 rounded-full bg-white/5 flex items-center justify-center mx-auto mb-4">
                  <CalendarIcon className="w-8 h-8 text-slate-500" />
                </div>
                <p className="text-slate-400">No releases on this date</p>
              </div>
            )}
            
            <div className="space-y-4">
              {selectedEntries.map((entry, idx) => (
                <div key={idx} className="flex gap-4 p-3 bg-[#2a2a2a] rounded-xl border border-white/5 group hover:border-white/10 transition-colors">
                  <div className="relative">
                    <img
                      src={tmdbImageUrl(entry.poster_path, 'w200')}
                      alt={entry.title}
                      className="w-16 h-24 rounded-lg object-cover"
                    />
                    <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity rounded-lg flex items-center justify-center">
                      <Play className="w-6 h-6 text-white fill-white" />
                    </div>
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                      {entry.type === 'movie' ? (
                        <Film className="w-4 h-4 text-purple-400" />
                      ) : (
                        <Tv className="w-4 h-4 text-emerald-400" />
                      )}
                      <span className={`text-xs px-2 py-0.5 rounded-full font-bold ${
                        entry.type === 'movie' 
                          ? 'bg-purple-500/20 text-purple-300' 
                          : 'bg-emerald-500/20 text-emerald-300'
                      }`}>
                        {entry.type === 'movie' ? 'Movie' : 'Episode'}
                      </span>
                    </div>
                    
                    <h4 className="text-white font-bold text-sm truncate">
                      {entry.type === 'episode' ? entry.series_title : entry.title}
                    </h4>
                    
                    {entry.type === 'episode' && (
                      <p className="text-slate-400 text-xs mt-1">
                        S{String(entry.season_number).padStart(2, '0')}E{String(entry.episode_number).padStart(2, '0')} - {entry.title}
                      </p>
                    )}
                    
                    {entry.overview && (
                      <p className="text-slate-500 text-xs mt-2 line-clamp-2">
                        {entry.overview}
                      </p>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Legend */}
        <div className="mt-8 flex items-center justify-center gap-8 text-sm">
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 bg-purple-500/20 border border-purple-500/30 rounded"></div>
            <span className="text-slate-400 font-medium">Movie</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 bg-emerald-500/20 border border-emerald-500/30 rounded"></div>
            <span className="text-slate-400 font-medium">Episode</span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 ring-2 ring-red-600 rounded"></div>
            <span className="text-slate-400 font-medium">Today</span>
          </div>
        </div>
      </div>
    </div>
  );
}
