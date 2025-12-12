import { useQuery } from '@tanstack/react-query';
import { streamarrApi } from '../services/api';
import { Film, Tv, Layers, Radio } from 'lucide-react';

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

  const dashboardStats = {
    totalMovies: stats?.total_movies || 0,
    monitoredMovies: stats?.monitored_movies || 0,
    availableMovies: stats?.available_movies || 0,
    totalSeries: stats?.total_series || 0,
    monitoredSeries: stats?.monitored_series || 0,
    totalEpisodes: stats?.total_episodes || 0,
    totalChannels: stats?.total_channels || 0,
    activeChannels: stats?.active_channels || 0,
    totalCollections: stats?.total_collections || 0,
  };

  const statCards = [
    {
      label: 'Total Movies',
      value: dashboardStats.totalMovies,
      icon: Film,
      color: 'bg-blue-500',
      subtitle: `${dashboardStats.monitoredMovies} monitored`,
    },
    {
      label: 'TV Series',
      value: dashboardStats.totalSeries,
      icon: Tv,
      color: 'bg-purple-500',
      subtitle: `${dashboardStats.monitoredSeries} monitored`,
    },
    {
      label: 'Live Channels',
      value: dashboardStats.totalChannels,
      icon: Radio,
      color: 'bg-red-500',
      subtitle: `${dashboardStats.activeChannels} active`,
    },
    {
      label: 'Collections',
      value: dashboardStats.totalCollections,
      icon: Layers,
      color: 'bg-cyan-500',
      subtitle: 'Movie collections',
    },
  ];

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-white mb-2">Dashboard</h1>
        <p className="text-slate-400">Overview of your media library</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        {statCards.map((stat) => (
          <div key={stat.label} className="card p-6">
            <div className="flex items-center justify-between mb-4">
              <div className={`${stat.color} p-3 rounded-lg`}>
                <stat.icon className="w-6 h-6 text-white" />
              </div>
            </div>
            <div className="text-3xl font-bold text-white mb-1">
              {stat.value.toLocaleString()}
            </div>
            <div className="text-slate-400 text-sm">{stat.label}</div>
            {stat.subtitle && (
              <div className="text-slate-500 text-xs mt-1">{stat.subtitle}</div>
            )}
          </div>
        ))}
      </div>

      {/* Quick Stats */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="card p-6">
          <h2 className="text-xl font-semibold text-white mb-4">Quick Stats</h2>
          
          <div className="space-y-4">
            <div>
              <div className="flex justify-between text-sm mb-2">
                <span className="text-slate-400">Monitoring Rate</span>
                <span className="text-white">
                  {dashboardStats.totalMovies > 0 
                    ? Math.round((dashboardStats.monitoredMovies / dashboardStats.totalMovies) * 100)
                    : 0}%
                </span>
              </div>
              <div className="h-2 bg-slate-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-green-500 transition-all"
                  style={{
                    width: `${dashboardStats.totalMovies > 0 
                      ? (dashboardStats.monitoredMovies / dashboardStats.totalMovies) * 100
                      : 0}%`
                  }}
                />
              </div>
            </div>

            <div>
              <div className="flex justify-between text-sm mb-2">
                <span className="text-slate-400">Series Coverage</span>
                <span className="text-white">
                  {dashboardStats.totalSeries > 0 
                    ? Math.round((dashboardStats.monitoredSeries / dashboardStats.totalSeries) * 100)
                    : 0}%
                </span>
              </div>
              <div className="h-2 bg-slate-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-purple-500 transition-all"
                  style={{
                    width: `${dashboardStats.totalSeries > 0 
                      ? (dashboardStats.monitoredSeries / dashboardStats.totalSeries) * 100
                      : 0}%`
                  }}
                />
              </div>
            </div>

            <div className="pt-4 border-t border-slate-700">
              <div className="grid grid-cols-2 gap-4 text-center">
                <div>
                  <div className="text-2xl font-bold text-white">{dashboardStats.totalMovies + dashboardStats.totalSeries}</div>
                  <div className="text-sm text-slate-400">Total Items</div>
                </div>
                <div>
                  <div className="text-2xl font-bold text-cyan-400">{dashboardStats.totalCollections}</div>
                  <div className="text-sm text-slate-400">Collections</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
