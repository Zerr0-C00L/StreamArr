import { useState, useEffect, type ReactNode } from 'react';
import { Link, useLocation, Outlet } from 'react-router-dom';
import { Settings, Home, Compass, Radio, Library, LogOut, Menu, X, User, ChevronDown, RotateCw } from 'lucide-react';
import axios from 'axios';

const api = axios.create({
  baseURL: '/api/v1',
});

api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('auth_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

interface LayoutProps {
  children?: ReactNode;
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation();
  const [scrolled, setScrolled] = useState(false);
  const [showUserMenu, setShowUserMenu] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [apiOnline, setApiOnline] = useState(false);
  const [versionInfo, setVersionInfo] = useState<{ current_version: string; current_commit: string } | null>(null);
  const [profilePicture, setProfilePicture] = useState<string | null>(null);
  const username = localStorage.getItem('username') || 'User';

  const handleLogout = () => {
    localStorage.removeItem('auth_token');
    localStorage.removeItem('username');
    localStorage.removeItem('is_admin');
    localStorage.removeItem('profile_picture');
    window.location.href = '/login';
  };

  const handleRestart = async () => {
    if (!window.confirm('Are you sure you want to restart the server? This will briefly interrupt service.')) {
      return;
    }
    
    try {
      setShowUserMenu(false);
      await axios.post('/api/v1/admin/restart');
      // Show success message
      alert('Server restarting...');
    } catch (error) {
      console.error('Restart failed:', error);
      alert('Failed to restart server');
    }
  };

  useEffect(() => {
    api.get('/version')
      .then((res) => {
        setApiOnline(true);
        if (res?.data) {
          setVersionInfo({
            current_version: res.data.current_version,
            current_commit: res.data.current_commit,
          });
        }
      })
      .catch(() => setApiOnline(false));
    
    // Fetch user profile to get profile picture
    api.get('/auth/profile')
      .then((res) => {
        if (res.data.profile_picture) {
          setProfilePicture(res.data.profile_picture);
          localStorage.setItem('profile_picture', res.data.profile_picture);
        }
      })
      .catch(() => {
        // Try to get from localStorage if API fails
        const cached = localStorage.getItem('profile_picture');
        if (cached) setProfilePicture(cached);
      });
  }, []);

  useEffect(() => {
    const handleScroll = () => {
      setScrolled(window.scrollY > 20);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const navItems = [
    { path: '/', icon: Home, label: 'Home' },
    { path: '/library', icon: Library, label: 'Library' },
    { path: '/livetv', icon: Radio, label: 'Live TV' },
    { path: '/search', icon: Compass, label: 'Discover' },
  ];

  const isActive = (path: string) => location.pathname === path;

  return (
    <div className="min-h-screen bg-[#141414]">
      {/* Netflix-style Header */}
      <header 
        className={`fixed top-0 left-0 right-0 z-50 transition-all duration-300 ${
          scrolled ? 'bg-[#141414]' : 'bg-gradient-to-b from-black/80 to-transparent'
        }`}
      >
        <div className="flex items-center justify-between px-4 md:px-8 py-3">
          {/* Left side - Logo and Nav */}
          <div className="flex items-center gap-8">
            {/* Logo */}
            <Link to="/" className="flex items-center gap-2">
              <img src="/logo.png" alt="StreamArr" className="w-8 h-8" />
              <span className="text-xl font-bold text-red-600 hidden sm:block">StreamArr</span>
            </Link>

            {/* Desktop Navigation */}
            <nav className="hidden md:flex items-center gap-1">
              {navItems.map((item) => (
                <Link
                  key={item.path}
                  to={item.path}
                  className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                    isActive(item.path)
                      ? 'text-white bg-white/10'
                      : 'text-slate-300 hover:text-white hover:bg-white/5'
                  }`}
                >
                  {item.label}
                </Link>
              ))}
            </nav>
          </div>

          {/* Right side - User menu */}
          <div className="flex items-center gap-4">
            {/* API Status indicator */}
            <div className={`w-2 h-2 rounded-full ${apiOnline ? 'bg-green-500' : 'bg-red-500'}`} 
                 title={apiOnline ? 'API Online' : 'API Offline'} />

            {/* Settings */}
            <Link
              to="/settings"
              className={`p-2 rounded-md transition-colors ${
                isActive('/settings') ? 'bg-white/10 text-white' : 'text-slate-400 hover:text-white'
              }`}
            >
              <Settings className="w-5 h-5" />
            </Link>

            {/* User dropdown */}
            <div className="relative">
              <button
                onClick={() => setShowUserMenu(!showUserMenu)}
                className="flex items-center gap-2 p-1.5 rounded-md hover:bg-white/10 transition-colors"
              >
                {profilePicture ? (
                  <img 
                    src={profilePicture} 
                    alt={username}
                    className="w-8 h-8 rounded object-cover"
                    onError={(e) => {
                      e.currentTarget.style.display = 'none';
                      e.currentTarget.nextElementSibling?.classList.remove('hidden');
                    }}
                  />
                ) : null}
                <div className={`w-8 h-8 rounded bg-gradient-to-br from-red-600 to-red-800 flex items-center justify-center ${profilePicture ? 'hidden' : ''}`}>
                  <User className="w-5 h-5 text-white" />
                </div>
                <ChevronDown className={`w-4 h-4 text-white transition-transform ${showUserMenu ? 'rotate-180' : ''}`} />
              </button>

              {showUserMenu && (
                <div className="absolute right-0 top-full mt-2 w-56 bg-[#1a1a1a] rounded-lg shadow-xl border border-white/10 py-2 z-50">
                  <div className="px-4 py-3 border-b border-white/10 flex items-center gap-3">
                    {profilePicture ? (
                      <img 
                        src={profilePicture} 
                        alt={username}
                        className="w-10 h-10 rounded-full object-cover"
                      />
                    ) : (
                      <div className="w-10 h-10 rounded-full bg-gradient-to-br from-red-600 to-red-800 flex items-center justify-center">
                        <User className="w-6 h-6 text-white" />
                      </div>
                    )}
                    <div>
                      <p className="text-white font-medium">{username}</p>
                      <p className="text-slate-500 text-xs">Account</p>
                    </div>
                  </div>
                  <Link
                    to="/settings"
                    className="flex items-center gap-3 px-4 py-2 text-slate-300 hover:bg-white/10 transition-colors"
                    onClick={() => setShowUserMenu(false)}
                  >
                    <Settings className="w-4 h-4" />
                    Settings
                  </Link>
                  <button
                    onClick={handleRestart}
                    className="flex items-center gap-3 px-4 py-2 text-slate-300 hover:bg-white/10 transition-colors w-full"
                  >
                    <RotateCw className="w-4 h-4" />
                    Restart
                  </button>
                  <button
                    onClick={handleLogout}
                    className="flex items-center gap-3 px-4 py-2 text-slate-300 hover:bg-white/10 transition-colors w-full"
                  >
                    <LogOut className="w-4 h-4" />
                    Sign Out
                  </button>
                </div>
              )}
            </div>

            {/* Mobile menu button */}
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="md:hidden p-2 rounded-md text-white hover:bg-white/10 transition-colors"
            >
              {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
            </button>
          </div>
        </div>

        {/* Mobile Navigation */}
        {mobileMenuOpen && (
          <nav className="md:hidden bg-[#141414] border-t border-white/10 px-4 py-3">
            {navItems.map((item) => (
              <Link
                key={item.path}
                to={item.path}
                onClick={() => setMobileMenuOpen(false)}
                className={`flex items-center gap-3 px-4 py-3 rounded-md text-base font-medium transition-colors ${
                  isActive(item.path)
                    ? 'text-white bg-white/10'
                    : 'text-slate-300 hover:text-white hover:bg-white/5'
                }`}
              >
                <item.icon className="w-5 h-5" />
                {item.label}
              </Link>
            ))}
          </nav>
        )}
      </header>

      {/* Click outside to close menus */}
      {(showUserMenu || mobileMenuOpen) && (
        <div 
          className="fixed inset-0 z-40" 
          onClick={() => { setShowUserMenu(false); setMobileMenuOpen(false); }}
        />
      )}

      {/* Main Content */}
      <main className="pt-16 min-h-screen">
        <div className="p-6">
          {children || <Outlet />}
        </div>
      </main>

      {/* Tiny footer with version & commit */}
      {versionInfo && (
        <div className="fixed bottom-2 right-3 z-40 text-xs text-slate-400 bg-black/40 border border-white/10 rounded px-2 py-1 backdrop-blur-sm">
          <span className="mr-2">v{versionInfo.current_version}</span>
          <span title="Commit">{versionInfo.current_commit}</span>
        </div>
      )}
    </div>
  );
}
