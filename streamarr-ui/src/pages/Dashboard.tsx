import { useQuery } from '@tanstack/react-query';
import { streamarrApi } from '../services/api';
import { Film, Tv, Check, Radio } from 'lucide-react';

export default function Dashboard() {
  const { data: movies } = useQuery({
    queryKey: ['movies'],
    queryFn: () => streamarrApi.getMovies({ limit: 1000 }).then(res => res.data),
  });

  const { data: series } = useQuery({
    queryKey: ['series'],
    queryFn: () => streamarrApi.getSeries({ limit: 1000 }).then(res => res.data),
  });

  const { data: channels } = useQuery({
    queryKey: ['channels'],
    queryFn: () => streamarrApi.getChannels().then(res => res.data),
  });

  const stats = {
    totalMovies: movies?.length || 0,
    monitoredMovies: movies?.filter(m => m.monitored).length || 0,
    availableMovies: movies?.filter(m => m.available).length || 0,
    totalSeries: series?.length || 0,
    monitoredSeries: series?.filter(s => s.monitored).length || 0,
    totalEpisodes: series?.reduce((sum, s) => sum + s.total_episodes, 0) || 0,
    totalChannels: channels?.length || 0,
    liveChannels: channels?.filter(ch => ch.active).length || 0,
    recentlyAdded: movies?.slice(0, 10) || [],
  };

  const statCards = [
    {
      label: 'Total Movies',
      value: stats.totalMovies,
      icon: Film,
      color: 'bg-blue-500',
      subtitle: `${stats.monitoredMovies} monitored`,
    },
    {
      label: 'TV Series',
      value: stats.totalSeries,
      icon: Tv,
      color: 'bg-purple-500',
      subtitle: `${stats.totalEpisodes} episodes`,
    },
    {
      label: 'Live Channels',
      value: stats.totalChannels,
      icon: Radio,
      color: 'bg-red-500',
      subtitle: `${stats.liveChannels} active`,
    },
    {
      label: 'Available',
      value: stats.availableMovies,
      icon: Check,
      color: 'bg-green-500',
      subtitle: 'Ready to watch',
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
                  {stats.totalMovies > 0 
                    ? Math.round((stats.monitoredMovies / stats.totalMovies) * 100)
                    : 0}%
                </span>
              </div>
              <div className="h-2 bg-slate-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-green-500 transition-all"
                  style={{
                    width: `${stats.totalMovies > 0 
                      ? (stats.monitoredMovies / stats.totalMovies) * 100
                      : 0}%`
                  }}
                />
              </div>
            </div>

            <div>
              <div className="flex justify-between text-sm mb-2">
                <span className="text-slate-400">Availability Rate</span>
                <span className="text-white">
                  {stats.totalMovies > 0 
                    ? Math.round((stats.availableMovies / stats.totalMovies) * 100)
                    : 0}%
                </span>
              </div>
              <div className="h-2 bg-slate-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-purple-500 transition-all"
                  style={{
                    width: `${stats.totalMovies > 0 
                      ? (stats.availableMovies / stats.totalMovies) * 100
                      : 0}%`
                  }}
                />
              </div>
            </div>

            <div className="pt-4 border-t border-slate-700">
              <div className="grid grid-cols-2 gap-4 text-center">
                <div>
                  <div className="text-2xl font-bold text-white">{stats.totalMovies}</div>
                  <div className="text-sm text-slate-400">Total Items</div>
                </div>
                <div>
                  <div className="text-2xl font-bold text-green-400">{stats.availableMovies}</div>
                  <div className="text-sm text-slate-400">Ready to Watch</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
