import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import { ChevronLeft, ChevronRight, Film, Tv, Calendar as CalendarIcon } from 'lucide-react';
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
    <div className="p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          <CalendarIcon className="w-8 h-8 text-blue-400" />
          <div>
            <h1 className="text-2xl font-bold text-white">Calendar</h1>
            <p className="text-gray-400">Upcoming releases and episodes</p>
          </div>
        </div>
        
        <div className="flex items-center gap-2">
          <button
            onClick={() => navigateMonth(-1)}
            className="p-2 rounded-lg bg-gray-700 hover:bg-gray-600 text-white"
          >
            <ChevronLeft className="w-5 h-5" />
          </button>
          
          <button
            onClick={goToToday}
            className="px-4 py-2 rounded-lg bg-blue-600 hover:bg-blue-500 text-white font-medium"
          >
            Today
          </button>
          
          <button
            onClick={() => navigateMonth(1)}
            className="p-2 rounded-lg bg-gray-700 hover:bg-gray-600 text-white"
          >
            <ChevronRight className="w-5 h-5" />
          </button>
        </div>
      </div>

      {/* Month/Year Display */}
      <div className="text-center mb-4">
        <h2 className="text-xl font-semibold text-white">
          {MONTHS[currentDate.getMonth()]} {currentDate.getFullYear()}
        </h2>
      </div>

      <div className="flex gap-6">
        {/* Calendar Grid */}
        <div className="flex-1">
          {/* Day Headers */}
          <div className="grid grid-cols-7 gap-1 mb-2">
            {DAYS.map(day => (
              <div key={day} className="text-center text-gray-400 text-sm font-medium py-2">
                {day}
              </div>
            ))}
          </div>

          {/* Calendar Days */}
          {isLoading ? (
            <div className="flex items-center justify-center h-96">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
            </div>
          ) : (
            <div className="grid grid-cols-7 gap-1">
              {calendarDays.map((date, idx) => {
                const dateKey = formatDateKey(date);
                const dayEntries = entriesByDate[dateKey] || [];
                const isSelected = selectedDate?.toDateString() === date.toDateString();
                
                return (
                  <button
                    key={idx}
                    onClick={() => setSelectedDate(date)}
                    className={`
                      min-h-[100px] p-2 rounded-lg text-left transition-colors
                      ${isCurrentMonth(date) ? 'bg-gray-800' : 'bg-gray-900/50'}
                      ${isSelected ? 'ring-2 ring-blue-500' : ''}
                      ${isToday(date) ? 'border border-blue-500' : ''}
                      hover:bg-gray-700
                    `}
                  >
                    <div className={`
                      text-sm font-medium mb-1
                      ${isToday(date) ? 'text-blue-400' : isCurrentMonth(date) ? 'text-white' : 'text-gray-500'}
                    `}>
                      {date.getDate()}
                    </div>
                    
                    {/* Show up to 3 entries, then "+N more" */}
                    <div className="space-y-1">
                      {dayEntries.slice(0, 3).map((entry, i) => (
                        <div
                          key={i}
                          className={`
                            text-xs px-1.5 py-0.5 rounded truncate
                            ${entry.type === 'movie' ? 'bg-purple-900/50 text-purple-300' : 'bg-green-900/50 text-green-300'}
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
                        <div className="text-xs text-gray-400 px-1">
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
        <div className="w-80 bg-gray-800 rounded-lg p-4">
          <h3 className="text-lg font-semibold text-white mb-4">
            {selectedDate 
              ? selectedDate.toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' })
              : 'Select a date'
            }
          </h3>
          
          {selectedDate && selectedEntries.length === 0 && (
            <p className="text-gray-400 text-sm">No releases on this date</p>
          )}
          
          <div className="space-y-3">
            {selectedEntries.map((entry, idx) => (
              <div key={idx} className="flex gap-3 p-2 bg-gray-700 rounded-lg">
                <img
                  src={tmdbImageUrl(entry.poster_path, 'w200')}
                  alt={entry.title}
                  className="w-12 h-18 rounded object-cover"
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    {entry.type === 'movie' ? (
                      <Film className="w-4 h-4 text-purple-400" />
                    ) : (
                      <Tv className="w-4 h-4 text-green-400" />
                    )}
                    <span className={`text-xs px-1.5 py-0.5 rounded ${
                      entry.type === 'movie' ? 'bg-purple-900/50 text-purple-300' : 'bg-green-900/50 text-green-300'
                    }`}>
                      {entry.type === 'movie' ? 'Movie' : 'Episode'}
                    </span>
                  </div>
                  
                  <h4 className="text-white font-medium text-sm truncate mt-1">
                    {entry.type === 'episode' ? entry.series_title : entry.title}
                  </h4>
                  
                  {entry.type === 'episode' && (
                    <p className="text-gray-400 text-xs">
                      S{String(entry.season_number).padStart(2, '0')}E{String(entry.episode_number).padStart(2, '0')} - {entry.title}
                    </p>
                  )}
                  
                  {entry.overview && (
                    <p className="text-gray-400 text-xs mt-1 line-clamp-2">
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
      <div className="mt-6 flex items-center gap-6 text-sm">
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-purple-900/50 rounded"></div>
          <span className="text-gray-400">Movie</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-green-900/50 rounded"></div>
          <span className="text-gray-400">Episode</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 border border-blue-500 rounded"></div>
          <span className="text-gray-400">Today</span>
        </div>
      </div>
    </div>
  );
}
