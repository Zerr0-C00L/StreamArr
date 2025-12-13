import { useState, useEffect, type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Film, Calendar, Settings, Activity, Compass, Radio, Library } from 'lucide-react';

interface LayoutProps {
  children: ReactNode;
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation();
  const [version, setVersion] = useState('...');
  const [apiOnline, setApiOnline] = useState(false);

  useEffect(() => {
    fetch('/api/v1/version')
      .then(res => res.json())
      .then(data => {
        setVersion(data.current_version || 'unknown');
        setApiOnline(true);
      })
      .catch(() => {
        setVersion('offline');
        setApiOnline(false);
      });
  }, []);

  const navItems = [
    { path: '/', icon: Activity, label: 'Dashboard' },
    { path: '/library', icon: Library, label: 'Library' },
    { path: '/livetv', icon: Radio, label: 'Live TV' },
    { path: '/calendar', icon: Calendar, label: 'Calendar' },
    { path: '/search', icon: Compass, label: 'Discovery' },
    { path: '/settings', icon: Settings, label: 'Settings' },
  ];

  const isActive = (path: string) => location.pathname === path;

  return (
    <div className="flex h-screen bg-slate-900">
      {/* Sidebar */}
      <aside className="w-64 bg-slate-800 border-r border-slate-700 flex flex-col">
        <div className="p-6">
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Film className="w-8 h-8 text-primary-500" />
            StreamArr
          </h1>
          <p className="text-slate-400 text-sm mt-1">Media Management</p>
        </div>

        <nav className="flex-1 px-3">
          {navItems.map((item) => (
            <Link
              key={item.path}
              to={item.path}
              className={`flex items-center gap-3 px-3 py-2.5 rounded-lg mb-1 transition-colors ${
                isActive(item.path)
                  ? 'bg-primary-600 text-white'
                  : 'text-slate-300 hover:bg-slate-700'
              }`}
            >
              <item.icon className="w-5 h-5" />
              <span className="font-medium">{item.label}</span>
            </Link>
          ))}
        </nav>

        <div className="p-4 border-t border-slate-700">
          <div className="text-xs text-slate-400">
            <div className="flex justify-between mb-1">
              <span>API Status</span>
              <span className={apiOnline ? "text-green-400" : "text-red-400"}>● {apiOnline ? 'Online' : 'Offline'}</span>
            </div>
            <div className="flex justify-between">
              <span>Version</span>
              <span>{version}</span>
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 overflow-auto">
        {children}
      </main>
    </div>
  );
}
