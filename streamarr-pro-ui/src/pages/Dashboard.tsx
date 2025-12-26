import { useQuery } from '@tanstack/react-query';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import { Film, Tv, Layers, Radio, TrendingUp, Clock, Star, ChevronRight, Calendar } from 'lucide-react';
import { Link } from 'react-router-dom';
import type { Movie, Series, CalendarEntry } from '../types';
import { useState } from 'react';

interface DashboardStats {
  total_movies: number;
  monitored_movies: number;
  available_movies: number;
  total_series: number;
  monitored_series: number;
  total_episodes: number;
  total_channels: number;
  active_channels: number;
  total_collections: number;
}

export default function Dashboard() {
  const { data: stats } = useQuery<DashboardStats>({
    queryKey: ['stats'],
    queryFn: () => streamarrApi.getStats().then(res => res.data),
  });

  // Get recent movies
  const { data: recentMovies = [] } = useQuery({
    queryKey: ['movies', 'recent-dashboard'],
    queryFn: () => streamarrApi.getMovies({ limit: 10 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Get recent series
  const { data: recentSeries = [] } = useQuery({
    queryKey: ['series', 'recent-dashboard'],
    queryFn: () => streamarrApi.getSeries({ limit: 10 }).then(res => Array.isArray(res.data) ? res.data : []),
  });

  // Get upcoming
  const today = new Date();
  const nextWeek = new Date(today);
  nextWeek.setDate(nextWeek.getDate() + 30); // Look 30 days ahead for upcoming
  const tomorrow = new Date(today);
  tomorrow.setDate(tomorrow.getDate() + 1); // Start from tomorrow for "coming soon"

  const { data: upcoming = [] } = useQuery({
    queryKey: ['calendar', 'dashboard'],
    queryFn: () => streamarrApi.getCalendar(
      tomorrow.toISOString().split('T')[0],
      nextWeek.toISOString().split('T')[0]
    ).then(res => Array.isArray(res.data) ? res.data.slice(0, 5) : []),
  });

  const dashboardStats = {
    totalMovies: stats?.total_movies || 0,
    monitoredMovies: stats?.monitored_movies || 0,
    totalSeries: stats?.total_series || 0,
    monitoredSeries: stats?.monitored_series || 0,
    totalChannels: stats?.total_channels || 0,
    activeChannels: stats?.active_channels || 0,
    totalCollections: stats?.total_collections || 0,
  };

  return (
    <div className="min-h-screen bg-[#141414] -m-6 p-8">
      {/* Welcome Section */}
      <div className="mb-10">
        <h1 className="text-4xl font-black text-white mb-2">Welcome Back</h1>
        <p className="text-slate-400 text-lg">Here's what's happening in your library</p>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-10">
        <StatCard
          icon={Film}
          label="Movies"
          value={dashboardStats.totalMovies}
          subtitle={`${dashboardStats.monitoredMovies} monitored`}
          color="purple"
          link="/library"
        />
        <StatCard
          icon={Tv}
          label="TV Shows"
          value={dashboardStats.totalSeries}
          subtitle={`${dashboardStats.monitoredSeries} monitored`}
          color="green"
          link="/library"
        />
        <StatCard
          icon={Radio}
          label="Live Channels"
          value={dashboardStats.totalChannels}
          subtitle={`${dashboardStats.activeChannels} active`}
          color="red"
          link="/livetv"
        />
        <StatCard
          icon={Layers}
          label="Collections"
          value={dashboardStats.totalCollections}
          subtitle="Movie collections"
          color="cyan"
        />
      </div>

      {/* Content Sections */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Recent Movies */}
        <div className="lg:col-span-2">
          <ContentSection
            title="Recently Added Movies"
            icon={<Film className="w-5 h-5 text-purple-500" />}
            items={recentMovies}
            type="movie"
            link="/library"
          />
        </div>

        {/* Upcoming */}
        <div>
          <UpcomingSection entries={upcoming} />
        </div>
      </div>

      {/* Recent Series */}
      <div className="mt-6">
        <ContentSection
          title="Recently Added Series"
          icon={<Tv className="w-5 h-5 text-green-500" />}
          items={recentSeries}
          type="series"
          link="/library"
        />
      </div>

      {/* Quick Actions */}
      <div className="mt-10 grid grid-cols-2 md:grid-cols-4 gap-4">
        <QuickAction
          icon={<TrendingUp className="w-6 h-6" />}
          label="Discover"
          description="Find new content"
          link="/search"
          color="cyan"
        />
        <QuickAction
          icon={<Radio className="w-6 h-6" />}
          label="Live TV"
          description="Watch live channels"
          link="/livetv"
          color="red"
        />
        <QuickAction
          icon={<Film className="w-6 h-6" />}
          label="Library"
          description="Browse your collection"
          link="/library"
          color="purple"
        />
        <QuickAction
          icon={<Star className="w-6 h-6" />}
          label="Settings"
          description="Configure your app"
          link="/settings"
          color="yellow"
        />
      </div>
    </div>
  );
}

// Stat Card Component
function StatCard({ 
  icon: Icon, 
  label, 
  value, 
  subtitle, 
  color, 
  link 
}: { 
  icon: any; 
  label: string; 
  value: number; 
  subtitle: string; 
  color: string;
  link?: string;
}) {
  const colorClasses: Record<string, string> = {
    purple: 'from-purple-600 to-purple-800',
    green: 'from-green-600 to-green-800',
    red: 'from-red-600 to-red-800',
    cyan: 'from-cyan-600 to-cyan-800',
  };

  const content = (
    <>
      <div className={`w-12 h-12 rounded-lg bg-gradient-to-br ${colorClasses[color]} flex items-center justify-center mb-4 group-hover:scale-110 transition-transform`}>
        <Icon className="w-6 h-6 text-white" />
      </div>
      <div className="text-3xl font-black text-white mb-1">{value.toLocaleString()}</div>
      <div className="text-white font-medium">{label}</div>
      <div className="text-slate-500 text-sm mt-1">{subtitle}</div>
    </>
  );

  if (!link) {
    return (
      <div className="bg-[#1e1e1e] rounded-xl p-5">
        {content}
      </div>
    );
  }

  return (
    <Link
      to={link}
      className="bg-[#1e1e1e] rounded-xl p-5 hover:bg-[#282828] transition-all group"
    >
      {content}
    </Link>
  );
}

// Content Section Component
function ContentSection({ 
  title, 
  icon, 
  items, 
  type, 
  link 
}: { 
  title: string; 
  icon: React.ReactNode; 
  items: (Movie | Series)[]; 
  type: 'movie' | 'series';
  link: string;
}) {
  const [hoveredItem, setHoveredItem] = useState<number | null>(null);

  if (items.length === 0) return null;

  return (
    <div className="bg-[#1e1e1e] rounded-xl p-5">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-bold text-white flex items-center gap-2">
          {icon}
          {title}
        </h2>
        <Link to={link} className="text-slate-400 hover:text-white text-sm flex items-center gap-1">
          View All <ChevronRight className="w-4 h-4" />
        </Link>
      </div>

      <div className="flex gap-3 overflow-x-auto pb-2 scrollbar-hide">
        {items.slice(0, 8).map((item) => (
          <Link
            key={item.id}
            to={`/library?${type === 'movie' ? 'movie' : 'series'}=${item.id}`}
            className="flex-shrink-0 w-36 group relative"
            onMouseEnter={() => setHoveredItem(item.id)}
            onMouseLeave={() => setHoveredItem(null)}
          >
            <div className="aspect-[2/3] rounded-lg overflow-hidden bg-slate-800 mb-2 group-hover:ring-2 ring-red-600 transition-all relative">
              {item.poster_path ? (
                <img
                  src={tmdbImageUrl(item.poster_path, 'w300')}
                  alt={item.title}
                  className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
                />
              ) : (
                <div className="w-full h-full flex items-center justify-center">
                  {type === 'movie' ? <Film className="w-8 h-8 text-slate-600" /> : <Tv className="w-8 h-8 text-slate-600" />}
                </div>
              )}
              
              {/* Rating badge */}
              {item.vote_average && item.vote_average > 0 && (
                <div className="absolute top-2 left-2 bg-black/80 backdrop-blur-sm px-1.5 py-0.5 rounded flex items-center gap-1">
                  <Star className="w-3 h-3 text-yellow-400 fill-yellow-400" />
                  <span className="text-white text-xs font-bold">{item.vote_average.toFixed(1)}</span>
                </div>
              )}

              {/* Year badge */}
              {(() => {
                const year = type === 'movie' 
                  ? (item as Movie).year || (item as Movie).release_date?.split('-')[0]
                  : (item as Series).first_air_date?.split('-')[0];
                return year ? (
                  <div className="absolute top-2 right-2 bg-black/80 backdrop-blur-sm px-1.5 py-0.5 rounded">
                    <span className="text-white text-xs font-medium">{year}</span>
                  </div>
                ) : null;
              })()}

              {/* Hover overlay with info */}
              <div className={`absolute inset-0 bg-gradient-to-t from-black via-black/50 to-transparent flex flex-col justify-end p-3 transition-opacity duration-200 ${hoveredItem === item.id ? 'opacity-100' : 'opacity-0'}`}>
                {item.overview && (
                  <p className="text-white/80 text-xs line-clamp-4 text-center">{item.overview}</p>
                )}
              </div>
            </div>
            
            <p className="text-white text-sm font-medium truncate">{item.title}</p>
            
            {/* Genre/Type subtitle */}
            <p className="text-slate-500 text-xs truncate">
              {type === 'movie' ? 'Movie' : 'TV Series'}
              {(() => {
                const year = type === 'movie' 
                  ? (item as Movie).year || (item as Movie).release_date?.split('-')[0]
                  : (item as Series).first_air_date?.split('-')[0];
                return year ? ` • ${year}` : '';
              })()}
            </p>
          </Link>
        ))}
      </div>
    </div>
  );
}

// Upcoming Section Component
function UpcomingSection({ entries }: { entries: CalendarEntry[] }) {
  return (
    <div className="bg-[#1e1e1e] rounded-xl p-5 h-full">
      <h2 className="text-lg font-bold text-white flex items-center gap-2 mb-4">
        <Clock className="w-5 h-5 text-yellow-500" />
        Coming Soon
      </h2>

      {entries.length === 0 ? (
        <div className="text-center py-8 text-slate-500">
          <Clock className="w-10 h-10 mx-auto mb-2 opacity-50" />
          <p>No upcoming releases</p>
        </div>
      ) : (
        <div className="space-y-3">
          {entries.map((entry, index) => {
            const releaseDate = new Date(entry.date);
            const today = new Date();
            const daysUntil = Math.ceil((releaseDate.getTime() - today.getTime()) / (1000 * 60 * 60 * 24));
            
            return (
              <div key={index} className="flex items-center gap-3 p-2 rounded-lg hover:bg-[#282828] transition-colors group">
                <div className="w-14 h-20 rounded-lg overflow-hidden bg-slate-800 flex-shrink-0 relative group-hover:ring-2 ring-red-600 transition-all">
                  {entry.poster_path ? (
                    <img
                      src={tmdbImageUrl(entry.poster_path, 'w200')}
                      alt={entry.title}
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center">
                      {entry.type === 'movie' ? <Film className="w-5 h-5 text-slate-600" /> : <Tv className="w-5 h-5 text-slate-600" />}
                    </div>
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-white text-sm font-medium truncate group-hover:text-red-400 transition-colors">{entry.title}</p>
                  <div className="flex items-center gap-2 mt-1">
                    <Calendar className="w-3 h-3 text-slate-500" />
                    <p className="text-slate-400 text-xs">
                      {releaseDate.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                    </p>
                    {daysUntil > 0 && daysUntil <= 7 && (
                      <span className="text-xs text-yellow-400">
                        {daysUntil === 1 ? 'Tomorrow' : `in ${daysUntil} days`}
                      </span>
                    )}
                  </div>
                  <span className={`inline-block mt-1 text-xs px-2 py-0.5 rounded-full font-medium ${
                    entry.type === 'movie' 
                      ? 'bg-purple-600/30 text-purple-400 border border-purple-500/30' 
                      : 'bg-green-600/30 text-green-400 border border-green-500/30'
                  }`}>
                    {entry.type === 'movie' ? 'Movie' : `S${entry.season_number}E${entry.episode_number}`}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

// Quick Action Component
function QuickAction({ 
  icon, 
  label, 
  description, 
  link, 
  color 
}: { 
  icon: React.ReactNode; 
  label: string; 
  description: string; 
  link: string;
  color: string;
}) {
  const colorClasses: Record<string, string> = {
    cyan: 'group-hover:bg-cyan-600',
    red: 'group-hover:bg-red-600',
    purple: 'group-hover:bg-purple-600',
    yellow: 'group-hover:bg-yellow-600',
  };

  return (
    <Link
      to={link}
      className="bg-[#1e1e1e] rounded-xl p-5 hover:bg-[#282828] transition-all group flex items-center gap-4"
    >
      <div className={`w-12 h-12 rounded-lg bg-slate-700 ${colorClasses[color]} flex items-center justify-center transition-colors`}>
        {icon}
      </div>
      <div>
        <div className="text-white font-semibold">{label}</div>
        <div className="text-slate-500 text-sm">{description}</div>
      </div>
    </Link>
  );
}
