import { useQuery } from '@tanstack/react-query';
import { streamarrApi, tmdbImageUrl } from '../services/api';
import { Film, Tv, Layers, Radio, TrendingUp, Clock, Star, ChevronRight } from 'lucide-react';
import { Link } from 'react-router-dom';
import type { Movie, Series, CalendarEntry } from '../types';

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
  nextWeek.setDate(nextWeek.getDate() + 7);

  const { data: upcoming = [] } = useQuery({
    queryKey: ['calendar', 'dashboard'],
    queryFn: () => streamarrApi.getCalendar(
      today.toISOString().split('T')[0],
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
            to={link}
            className="flex-shrink-0 w-28 group"
          >
            <div className="aspect-[2/3] rounded-lg overflow-hidden bg-slate-800 mb-2 group-hover:ring-2 ring-white transition-all">
              {item.poster_path ? (
                <img
                  src={tmdbImageUrl(item.poster_path, 'w200')}
                  alt={item.title}
                  className="w-full h-full object-cover"
                />
              ) : (
                <div className="w-full h-full flex items-center justify-center">
                  {type === 'movie' ? <Film className="w-8 h-8 text-slate-600" /> : <Tv className="w-8 h-8 text-slate-600" />}
                </div>
              )}
            </div>
            <p className="text-white text-xs font-medium truncate">{item.title}</p>
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
          {entries.map((entry, index) => (
            <div key={index} className="flex items-center gap-3 p-2 rounded-lg hover:bg-[#282828] transition-colors">
              <div className="w-12 h-16 rounded overflow-hidden bg-slate-800 flex-shrink-0">
                {entry.poster_path ? (
                  <img
                    src={tmdbImageUrl(entry.poster_path, 'w200')}
                    alt={entry.title}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <div className="w-full h-full flex items-center justify-center">
                    {entry.type === 'movie' ? <Film className="w-4 h-4 text-slate-600" /> : <Tv className="w-4 h-4 text-slate-600" />}
                  </div>
                )}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-white text-sm font-medium truncate">{entry.title}</p>
                <p className="text-slate-500 text-xs">
                  {new Date(entry.date).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                </p>
                <span className={`text-xs px-1.5 py-0.5 rounded ${
                  entry.type === 'movie' ? 'bg-purple-600/30 text-purple-400' : 'bg-green-600/30 text-green-400'
                }`}>
                  {entry.type === 'movie' ? 'Movie' : `S${entry.season_number}E${entry.episode_number}`}
                </span>
              </div>
            </div>
          ))}
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
