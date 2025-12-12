import { useState, useEffect } from 'react';
import { Save, Key, Layers, Settings as SettingsIcon, List, Bell, Code, Plus, X, Tv, Server, Activity, Play, Clock, RefreshCw, Filter } from 'lucide-react';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

interface SettingsData {
  tmdb_api_key: string;
  realdebrid_api_key: string;
  premiumize_api_key: string;
  mdblist_api_key: string;
  user_create_playlist: boolean;
  total_pages: number;
  language: string;
  movies_origin_country: string;
  series_origin_country: string;
  max_resolution: number;
  max_file_size: number;
  use_realdebrid: boolean;
  use_premiumize: boolean;
  stream_providers: string;
  torrentio_providers: string;
  comet_indexers: string;
  enable_quality_variants: boolean;
  show_full_stream_name: boolean;
  auto_add_collections: boolean;
  include_live_tv: boolean;
  include_adult_vod: boolean;
  min_year: number;
  min_runtime: number;
  enable_notifications: boolean;
  discord_webhook_url: string;
  telegram_bot_token: string;
  telegram_chat_id: string;
  debug: boolean;
  server_port: number;
  host: string;
  mdblist_lists: string;
  // Proxy settings
  http_proxy: string;
  use_http_proxy: boolean;
  // HeadlessVidX settings
  headless_vidx_address: string;
  headless_vidx_max_threads: number;
  // Cache settings
  auto_cache_interval_hours: number;
  // Release filter settings
  excluded_release_groups: string;
  excluded_language_tags: string;
  excluded_qualities: string;
  custom_exclude_patterns: string;
  enable_release_filters: boolean;
  // Stream sorting settings
  stream_sort_order: string;
  stream_sort_prefer: string;
  // Live TV settings
  livetv_enable_plutotv: boolean;
}

interface MDBListEntry {
  url: string;
  name?: string;
  enabled: boolean;
}

interface M3USource {
  name: string;
  url: string;
  enabled: boolean;
}

interface ChannelStats {
  total_channels: number;
  categories: Array<{name: string; count: number}>;
  sources: Array<{name: string; count: number}>;
}

interface ServiceStatus {
  name: string;
  description: string;
  enabled: boolean;
  running: boolean;
  interval: string;
  last_run: string;
  next_run: string;
  last_error?: string;
  run_count: number;
}

type TabType = 'api' | 'providers' | 'quality' | 'playlist' | 'livetv' | 'filters' | 'services' | 'xtream' | 'notifications' | 'advanced';

export default function Settings() {
  const [settings, setSettings] = useState<SettingsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');
  const [activeTab, setActiveTab] = useState<TabType>('api');
  const [newListUrl, setNewListUrl] = useState('');
  const [mdbLists, setMdbLists] = useState<MDBListEntry[]>([]);
  const [userLists, setUserLists] = useState<Array<{id: number; name: string; slug: string; items: number; user_name?: string}>>([]);
  const [mdbUsername, setMdbUsername] = useState('');
  const [fetchingUserLists, setFetchingUserLists] = useState(false);
  const [m3uSources, setM3uSources] = useState<M3USource[]>([]);
  const [newM3uName, setNewM3uName] = useState('');
  const [newM3uUrl, setNewM3uUrl] = useState('');
  const [channelStats, setChannelStats] = useState<ChannelStats | null>(null);
  const [enabledCategories, setEnabledCategories] = useState<Set<string>>(new Set());
  const [enabledSources, setEnabledSources] = useState<Set<string>>(new Set());
  const [services, setServices] = useState<ServiceStatus[]>([]);
  const [triggeringService, setTriggeringService] = useState<string | null>(null);

  useEffect(() => {
    fetchSettings();
    fetchChannelStats();
    fetchServices();
  }, []);

  // Poll services status when on services tab
  useEffect(() => {
    if (activeTab === 'services') {
      const interval = setInterval(fetchServices, 5000);
      return () => clearInterval(interval);
    }
  }, [activeTab]);

  const fetchServices = async () => {
    try {
      const response = await fetch(`${API_BASE_URL}/services`);
      const data = await response.json();
      setServices(data.services || []);
    } catch (error) {
      console.error('Failed to fetch services:', error);
    }
  };

  const triggerService = async (serviceName: string) => {
    setTriggeringService(serviceName);
    try {
      const response = await fetch(`${API_BASE_URL}/services/${serviceName}/trigger?name=${serviceName}`, {
        method: 'POST',
      });
      if (response.ok) {
        setMessage(`✅ Service "${serviceName}" triggered successfully`);
        setTimeout(() => setMessage(''), 3000);
        // Refresh services after a short delay
        setTimeout(fetchServices, 500);
      } else {
        const data = await response.json();
        setMessage(`❌ Failed to trigger service: ${data.error}`);
        setTimeout(() => setMessage(''), 5000);
      }
    } catch (error) {
      setMessage(`❌ Failed to trigger service: ${error}`);
      setTimeout(() => setMessage(''), 5000);
    }
    setTriggeringService(null);
  };

  const toggleServiceEnabled = async (serviceName: string, enabled: boolean) => {
    try {
      const response = await fetch(`${API_BASE_URL}/services/${serviceName}?name=${serviceName}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
      });
      if (response.ok) {
        fetchServices();
      }
    } catch (error) {
      console.error('Failed to toggle service:', error);
    }
  };

  const formatServiceName = (name: string) => {
    return name.split('_').map(word => word.charAt(0).toUpperCase() + word.slice(1)).join(' ');
  };

  const formatDateTime = (dateStr: string) => {
    if (!dateStr || dateStr === '0001-01-01T00:00:00Z') return 'Never';
    const date = new Date(dateStr);
    return date.toLocaleString();
  };

  const fetchChannelStats = async () => {
    try {
      const response = await fetch(`${API_BASE_URL}/channels/stats`);
      const data = await response.json();
      setChannelStats(data);
      
      // Initialize enabled sets from settings or default to all
      if (data.categories) {
        setEnabledCategories(new Set(data.categories.map((c: {name: string}) => c.name)));
      }
      if (data.sources) {
        setEnabledSources(new Set(data.sources.map((s: {name: string}) => s.name)));
      }
    } catch (error) {
      console.error('Failed to fetch channel stats:', error);
    }
  };

  const fetchSettings = async () => {
    try {
      const response = await fetch(`${API_BASE_URL}/settings`);
      const data = await response.json();
      setSettings(data);
      
      if (data.mdblist_lists) {
        try {
          const lists = JSON.parse(data.mdblist_lists);
          setMdbLists(Array.isArray(lists) ? lists : []);
        } catch {
          setMdbLists([]);
        }
      }
      
      // Load M3U sources
      if (data.m3u_sources && Array.isArray(data.m3u_sources)) {
        setM3uSources(data.m3u_sources);
      }
    } catch (error) {
      console.error('Failed to fetch settings:', error);
      setMessage('Failed to load settings');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    if (!settings) return;
    
    setSaving(true);
    setMessage('');
    
    try {
      const settingsToSave = {
        ...settings,
        mdblist_lists: JSON.stringify(mdbLists),
        m3u_sources: m3uSources,
      };
      
      const response = await fetch(`${API_BASE_URL}/settings`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settingsToSave),
      });
      
      if (response.ok) {
        setMessage('✅ Settings saved successfully!');
        setTimeout(() => setMessage(''), 3000);
      } else {
        setMessage('❌ Failed to save settings');
      }
    } catch (error) {
      console.error('Failed to save settings:', error);
      setMessage('❌ Error saving settings');
    } finally {
      setSaving(false);
    }
  };

  const updateSetting = (key: keyof SettingsData, value: any) => {
    if (!settings) return;
    setSettings({ ...settings, [key]: value });
  };

  const addMDBList = () => {
    if (!newListUrl.trim()) return;
    if (!newListUrl.includes('mdblist.com/lists/')) {
      setMessage('❌ Invalid MDBList URL format');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    const match = newListUrl.match(/\/lists\/[^\/]+\/([^\/\?]+)/);
    const name = match ? match[1].replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase()) : 'MDBList';
    
    setMdbLists([...mdbLists, { url: newListUrl, name, enabled: true }]);
    setNewListUrl('');
  };

  const removeMDBList = (index: number) => {
    setMdbLists(mdbLists.filter((_, i) => i !== index));
  };

  const toggleMDBList = (index: number) => {
    setMdbLists(mdbLists.map((list, i) => 
      i === index ? { ...list, enabled: !list.enabled } : list
    ));
  };

  const fetchUserMDBLists = async () => {
    if (!settings?.mdblist_api_key) {
      setMessage('❌ Please enter your MDBList API key first');
      setTimeout(() => setMessage(''), 3000);
      return;
    }

    setFetchingUserLists(true);
    try {
      const response = await fetch(`${API_BASE_URL}/mdblist/user-lists?apiKey=${encodeURIComponent(settings.mdblist_api_key)}`);
      const data = await response.json();
      
      if (data.success && data.lists) {
        // Deduplicate lists by name (keep the one with more items)
        const byName = new Map<string, {id: number; name: string; slug: string; items: number}>();
        for (const list of data.lists) {
          const existing = byName.get(list.name);
          if (!existing || list.items > existing.items) {
            byName.set(list.name, list);
          }
        }
        const uniqueLists = Array.from(byName.values());
        setUserLists(uniqueLists);
        if (data.username) {
          setMdbUsername(data.username);
        }
        if (uniqueLists.length === 0) {
          setMessage('No lists found in your MDBList account');
          setTimeout(() => setMessage(''), 3000);
        }
      } else {
        setMessage('❌ ' + (data.error || 'Failed to fetch lists'));
        setTimeout(() => setMessage(''), 3000);
      }
    } catch (error) {
      console.error('Failed to fetch user lists:', error);
      setMessage('❌ Failed to fetch user lists');
      setTimeout(() => setMessage(''), 3000);
    } finally {
      setFetchingUserLists(false);
    }
  };

  const addUserList = (list: {id: number; name: string; slug: string; items: number; user_name?: string}) => {
    // Use username from list or from fetched username
    const username = list.user_name || mdbUsername;
    const url = `https://mdblist.com/lists/${username}/${list.slug}`;
    
    // Check if already added
    if (mdbLists.some(l => l.url.includes(list.slug))) {
      setMessage('⚠️ This list is already added');
      setTimeout(() => setMessage(''), 2000);
      return;
    }
    
    setMdbLists([...mdbLists, { url, name: list.name, enabled: true }]);
  };

  // M3U Source management functions
  const addM3uSource = () => {
    if (!newM3uName.trim() || !newM3uUrl.trim()) {
      setMessage('❌ Please enter both a name and URL for the M3U source');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    // Validate URL
    if (!newM3uUrl.startsWith('http://') && !newM3uUrl.startsWith('https://')) {
      setMessage('❌ M3U URL must start with http:// or https://');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    // Check for duplicates
    if (m3uSources.some(s => s.url === newM3uUrl || s.name === newM3uName.trim())) {
      setMessage('⚠️ A source with this name or URL already exists');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    setM3uSources([...m3uSources, { name: newM3uName.trim(), url: newM3uUrl.trim(), enabled: true }]);
    setNewM3uName('');
    setNewM3uUrl('');
    setMessage('✅ M3U source added. Click Save to apply.');
    setTimeout(() => setMessage(''), 3000);
  };

  const removeM3uSource = (index: number) => {
    setM3uSources(m3uSources.filter((_, i) => i !== index));
  };

  const toggleM3uSource = (index: number) => {
    setM3uSources(m3uSources.map((source, i) => 
      i === index ? { ...source, enabled: !source.enabled } : source
    ));
  };

  if (loading) {
    return (
      <div className="p-8 text-white">
        <div className="animate-pulse">Loading settings...</div>
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="p-8 text-white">
        <div className="text-red-500">Failed to load settings</div>
      </div>
    );
  }

  const tabs = [
    { id: 'api' as TabType, label: 'API Keys', icon: Key },
    { id: 'providers' as TabType, label: 'Providers', icon: Layers },
    { id: 'quality' as TabType, label: 'Quality', icon: SettingsIcon },
    { id: 'playlist' as TabType, label: 'Playlist', icon: List },
    { id: 'livetv' as TabType, label: 'Live TV', icon: Tv },
    { id: 'filters' as TabType, label: 'Filters', icon: Filter },
    { id: 'services' as TabType, label: 'Services', icon: Activity },
    { id: 'xtream' as TabType, label: 'Xtream', icon: Server },
    { id: 'notifications' as TabType, label: 'Notifications', icon: Bell },
    { id: 'advanced' as TabType, label: 'Advanced', icon: Code },
  ];

  return (
    <div className="p-8">
      <div className="max-w-6xl">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-3xl font-bold text-white">Settings</h1>
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed"
          >
            <Save className="w-4 h-4" />
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>

        {message && (
          <div className="mb-4 p-4 bg-gray-800 border border-gray-700 rounded-lg text-white">
            {message}
          </div>
        )}

        {/* Tabs */}
        <div className="mb-6 border-b border-gray-800">
          <div className="flex gap-4 overflow-x-auto">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 px-4 py-3 border-b-2 transition-colors whitespace-nowrap ${
                  activeTab === tab.id
                    ? 'border-blue-500 text-blue-500'
                    : 'border-transparent text-gray-400 hover:text-gray-300'
                }`}
              >
                <tab.icon className="w-4 h-4" />
                {tab.label}
              </button>
            ))}
          </div>
        </div>

        {/* API Keys Tab */}
        {activeTab === 'api' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  TMDB API Key <span className="text-red-400">*</span>
                </label>
                <input
                  type="text"
                  value={settings.tmdb_api_key || ''}
                  onChange={(e) => updateSetting('tmdb_api_key', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="Your TMDB API key"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Required. Used to fetch movie/series metadata, posters, and discover content.{' '}
                  <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:underline">
                    Get one free
                  </a>
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Real-Debrid API Key
                </label>
                <input
                  type="text"
                  value={settings.realdebrid_api_key || ''}
                  onChange={(e) => updateSetting('realdebrid_api_key', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="Your Real-Debrid API key"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Debrid service for fast, cached torrent streams. Enables higher quality sources.{' '}
                  <a href="https://real-debrid.com/apitoken" target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:underline">
                    Get API token
                  </a>
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Premiumize API Key
                </label>
                <input
                  type="text"
                  value={settings.premiumize_api_key || ''}
                  onChange={(e) => updateSetting('premiumize_api_key', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="Your Premiumize API key"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Alternative debrid service. Use either Real-Debrid OR Premiumize (or both).
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  MDBList API Key
                </label>
                <input
                  type="text"
                  value={settings.mdblist_api_key || ''}
                  onChange={(e) => updateSetting('mdblist_api_key', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="Your MDBList API key (optional for public lists)"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Only required for private lists. Get yours from{' '}
                  <a href="https://mdblist.com/preferences/" target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:underline">
                    mdblist.com/preferences
                  </a>
                </p>
              </div>

              {/* MDBList Auto-Import */}
              <div className="mt-6 pt-6 border-t border-gray-700">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-lg font-medium text-gray-300">MDBList Auto-Import Lists</h3>
                  <button
                    onClick={fetchUserMDBLists}
                    disabled={fetchingUserLists || !settings.mdblist_api_key}
                    className="px-3 py-1.5 text-sm bg-gray-700 text-white rounded-lg hover:bg-gray-600 disabled:bg-gray-800 disabled:text-gray-500 disabled:cursor-not-allowed transition-colors"
                  >
                    {fetchingUserLists ? 'Loading...' : '📋 Fetch My Lists'}
                  </button>
                </div>
                <p className="text-sm text-gray-400 mb-4">
                  Automatically add movies/series from MDBList curated lists to your library (Movies/Series pages). 
                  The worker periodically syncs these lists. This is separate from playlist generation.
                </p>

                {/* User's MDBLists from API */}
                {userLists.length > 0 && (
                  <div className="mb-4 p-3 bg-gray-800 rounded-lg">
                    <p className="text-sm text-gray-300 mb-2">Your MDBLists (click to add):</p>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-1.5 max-h-48 overflow-y-auto">
                      {userLists.map((list) => (
                        <button
                          key={list.id}
                          onClick={() => addUserList(list)}
                          className="px-2 py-1.5 text-xs text-left bg-blue-600/20 text-blue-400 rounded hover:bg-blue-600/30 transition-colors border border-blue-600/30"
                        >
                          {list.name} <span className="text-gray-500">({list.items})</span>
                        </button>
                      ))}
                    </div>
                  </div>
                )}
                
                <div className="flex gap-2 mb-3">
                  <input
                    type="text"
                    value={newListUrl}
                    onChange={(e) => setNewListUrl(e.target.value)}
                    onKeyPress={(e) => e.key === 'Enter' && addMDBList()}
                    className="flex-1 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    placeholder="https://mdblist.com/lists/username/list-name"
                  />
                  <button
                    onClick={addMDBList}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                  </button>
                </div>

                {mdbLists.length > 0 && (
                  <div className="space-y-2 max-h-64 overflow-y-auto">
                    {mdbLists.map((list, index) => (
                      <div key={index} className="flex items-center gap-2 p-3 bg-gray-800 rounded-lg">
                        <input
                          type="checkbox"
                          checked={list.enabled}
                          onChange={() => toggleMDBList(index)}
                          className="w-4 h-4"
                        />
                        <div className="flex-1 min-w-0">
                          <div className="text-sm font-medium text-white truncate">{list.name}</div>
                          <div className="text-xs text-gray-400 truncate">{list.url}</div>
                        </div>
                        <button
                          onClick={() => removeMDBList(index)}
                          className="p-1 text-red-400 hover:text-red-300 transition-colors"
                        >
                          <X className="w-4 h-4" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}

                {mdbLists.length === 0 && (
                  <div className="text-center py-6 text-gray-500 bg-gray-800 rounded-lg">
                    <p className="text-sm">No lists configured yet</p>
                    <p className="text-xs mt-1">Add popular lists like "Top Watched Movies of the Week"</p>
                  </div>
                )}

                <div className="mt-3 text-xs text-gray-500">
                  <p className="mb-1">💡 Popular lists:</p>
                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={() => setNewListUrl('https://mdblist.com/lists/linaspuransen/top-watched-movies-of-the-week')}
                      className="text-blue-400 hover:underline"
                    >
                      Top Watched Movies
                    </button>
                    <span>•</span>
                    <button
                      onClick={() => setNewListUrl('https://mdblist.com/lists/linaspuransen/top-watched-tv-shows-of-the-week')}
                      className="text-blue-400 hover:underline"
                    >
                      Top Watched TV Shows
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Providers Tab */}
        {activeTab === 'providers' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              <div>
                <h3 className="text-md font-medium text-gray-300 mb-4">Debrid Services</h3>
                <p className="text-xs text-gray-500 mb-4">Premium services that cache torrents for instant high-speed streaming</p>
                <div className="space-y-3">
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="use_realdebrid"
                      checked={settings.use_realdebrid || false}
                      onChange={(e) => updateSetting('use_realdebrid', e.target.checked)}
                      className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                    />
                    <label htmlFor="use_realdebrid" className="text-sm text-gray-300">
                      Use Real-Debrid
                    </label>
                  </div>

                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="use_premiumize"
                      checked={settings.use_premiumize || false}
                      onChange={(e) => updateSetting('use_premiumize', e.target.checked)}
                      className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                    />
                    <label htmlFor="use_premiumize" className="text-sm text-gray-300">
                      Use Premiumize
                    </label>
                  </div>
                </div>
              </div>

              <div className="pt-6 border-t border-gray-700">
                <h3 className="text-md font-medium text-gray-300 mb-4">Stream Providers</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Stream Providers
                    </label>
                    <input
                      type="text"
                      value={settings.stream_providers || ''}
                      onChange={(e) => updateSetting('stream_providers', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="comet,mediafusion,torrentio"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Stremio addons to fetch streams from. Options: comet, mediafusion, torrentio, torbox
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Torrentio Providers
                    </label>
                    <input
                      type="text"
                      value={settings.torrentio_providers || ''}
                      onChange={(e) => updateSetting('torrentio_providers', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="yts,eztv,rarbg,1337x,thepiratebay"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Torrent indexers for Torrentio. Options: yts, eztv, rarbg, 1337x, thepiratebay, kickasstorrents, torrentgalaxy
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Comet Indexers
                    </label>
                    <input
                      type="text"
                      value={settings.comet_indexers || ''}
                      onChange={(e) => updateSetting('comet_indexers', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="bitsearch,eztv,thepiratebay,therarbg,yts"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Torrent indexers for Comet. Options: bitsearch, eztv, thepiratebay, therarbg, yts, nyaa
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Quality Tab */}
        {activeTab === 'quality' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Maximum Resolution
                </label>
                <select
                  value={settings.max_resolution || 1080}
                  onChange={(e) => updateSetting('max_resolution', Number(e.target.value))}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                >
                  <option value="720">720p (HD)</option>
                  <option value="1080">1080p (Full HD)</option>
                  <option value="2160">2160p (4K)</option>
                </select>
                <p className="text-xs text-gray-500 mt-1">Highest video quality to include in streams. Lower = smaller files, faster loading.</p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Max File Size (MB)
                </label>
                <input
                  type="number"
                  value={settings.max_file_size || 0}
                  onChange={(e) => updateSetting('max_file_size', Number(e.target.value))}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="0 (unlimited)"
                />
                <p className="text-xs text-gray-500 mt-1">Skip files larger than this. 0 = no limit. Useful for slow connections.</p>
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="quality_variants"
                    checked={settings.enable_quality_variants || false}
                    onChange={(e) => updateSetting('enable_quality_variants', e.target.checked)}
                    className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                  />
                  <label htmlFor="quality_variants" className="text-sm text-gray-300">
                    Enable Quality Variants
                  </label>
                </div>
                <p className="text-xs text-gray-500 mt-1 ml-6">Show multiple quality options (720p, 1080p, 4K) for each stream.</p>
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="full_stream_name"
                    checked={settings.show_full_stream_name || false}
                    onChange={(e) => updateSetting('show_full_stream_name', e.target.checked)}
                    className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                  />
                  <label htmlFor="full_stream_name" className="text-sm text-gray-300">
                    Show Full Stream Names
                  </label>
                </div>
                <p className="text-xs text-gray-500 mt-1 ml-6">Display detailed stream info (codec, size, etc.) in player.</p>
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="auto_add_collections"
                    checked={settings.auto_add_collections || false}
                    onChange={(e) => updateSetting('auto_add_collections', e.target.checked)}
                    className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                  />
                  <label htmlFor="auto_add_collections" className="text-sm text-gray-300">
                    Add Entire Collection
                  </label>
                </div>
                <p className="text-xs text-gray-500 mt-1 ml-6">When adding a movie that belongs to a collection (e.g., The Dark Knight Trilogy), automatically add all other movies from that collection.</p>
              </div>

              <div className="pt-6 border-t border-gray-700">
                <h3 className="text-md font-medium text-gray-300 mb-4">Content Filters</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Minimum Year
                    </label>
                    <input
                      type="number"
                      value={settings.min_year || 1900}
                      onChange={(e) => updateSetting('min_year', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="1900"
                    />
                    <p className="text-xs text-gray-500 mt-1">Exclude movies/series released before this year.</p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Minimum Runtime (minutes)
                    </label>
                    <input
                      type="number"
                      value={settings.min_runtime || 0}
                      onChange={(e) => updateSetting('min_runtime', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="0"
                    />
                    <p className="text-xs text-gray-500 mt-1">Exclude short content (trailers, clips). 60+ recommended for movies.</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Playlist Tab */}
        {activeTab === 'playlist' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              <div className="pb-4 border-b border-gray-700">
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="user_playlist"
                    checked={settings.user_create_playlist || false}
                    onChange={(e) => updateSetting('user_create_playlist', e.target.checked)}
                    className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                  />
                  <label htmlFor="user_playlist" className="text-sm font-medium text-gray-300">
                    Create Custom Playlist
                  </label>
                </div>
                <p className="text-xs text-gray-500 mt-2 ml-6">
                  Generate M3U8 playlists from YOUR library for IPTV players (VLC, TiviMate, etc.). 
                  If disabled, uses generic pre-made playlists from GitHub.
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Total Pages to Fetch
                </label>
                <input
                  type="number"
                  value={settings.total_pages || 5}
                  onChange={(e) => updateSetting('total_pages', Number(e.target.value))}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  min="1"
                  max="50"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Pages of TMDB discovery results to import. 5 pages ≈ 3,000 items, 10 pages ≈ 6,500 items. 
                  Higher = more content but slower.
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Language
                </label>
                <input
                  type="text"
                  value={settings.language || 'en-US'}
                  onChange={(e) => updateSetting('language', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="en-US"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Preferred language for movie/series metadata (titles, descriptions). Format: en-US, de-DE, fr-FR
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Movies Origin Country
                </label>
                <input
                  type="text"
                  value={settings.movies_origin_country || ''}
                  onChange={(e) => updateSetting('movies_origin_country', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="US"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Filter TMDB discovery to movies from this country. Leave empty for all countries.
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Series Origin Country
                </label>
                <input
                  type="text"
                  value={settings.series_origin_country || ''}
                  onChange={(e) => updateSetting('series_origin_country', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="US"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Filter TMDB discovery to series from this country. Leave empty for all countries.
                </p>
              </div>

              <div className="pt-4 border-t border-gray-700">
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="include_livetv"
                    checked={settings.include_live_tv || false}
                    onChange={(e) => updateSetting('include_live_tv', e.target.checked)}
                    className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                  />
                  <label htmlFor="include_livetv" className="text-sm font-medium text-gray-300">
                    Include Live TV
                  </label>
                </div>
                <p className="text-xs text-gray-500 mt-1 ml-6">
                  Add live TV channels to your M3U8 playlist. Requires Live TV sources configured.
                </p>
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="include_adult"
                    checked={settings.include_adult_vod || false}
                    onChange={(e) => updateSetting('include_adult_vod', e.target.checked)}
                    className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                  />
                  <label htmlFor="include_adult" className="text-sm font-medium text-gray-300">
                    Include Adult Content
                  </label>
                </div>
                <p className="text-xs text-gray-500 mt-1 ml-6">
                  Include adult-rated content (18+) in TMDB discovery and playlists.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Live TV Tab */}
        {activeTab === 'livetv' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-blue-900/30 border border-blue-800 rounded-lg">
                <h3 className="text-blue-400 font-medium mb-2">📺 Live TV Configuration</h3>
                <p className="text-sm text-gray-300">
                  Manage Live TV channels from M3U sources. Currently loaded: <span className="font-bold text-white">{channelStats?.total_channels || 0}</span> channels
                  from <span className="font-bold text-white">{channelStats?.sources?.length || 0}</span> sources across <span className="font-bold text-white">{channelStats?.categories?.length || 0}</span> categories.
                </p>
              </div>

              {/* Pluto TV Toggle */}
              <div className="p-4 bg-purple-900/20 border border-purple-800 rounded-lg">
                <div className="flex items-center justify-between">
                  <div>
                    <h4 className="text-white font-medium flex items-center gap-2">
                      <span>📺</span> Pluto TV (Built-in)
                    </h4>
                    <p className="text-sm text-gray-400 mt-1">
                      Enable free Pluto TV channels with full EPG support. Channels are automatically merged with other sources.
                    </p>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={settings.livetv_enable_plutotv !== false}
                      onChange={(e) => updateSetting('livetv_enable_plutotv', e.target.checked)}
                      className="sr-only peer"
                    />
                    <div className="w-11 h-6 bg-gray-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-purple-600"></div>
                  </label>
                </div>
              </div>

              {/* Channel Sources */}
              {channelStats && channelStats.sources && channelStats.sources.length > 0 && (
                <div>
                  <h3 className="text-lg font-medium text-white mb-4">📡 Channel Sources</h3>
                  <p className="text-sm text-gray-400 mb-3">Toggle sources on/off to filter which channels appear in Live TV.</p>
                  <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
                    {channelStats.sources.map((source) => (
                      <div
                        key={source.name}
                        className={`flex items-center gap-2 p-3 rounded-lg border cursor-pointer transition-colors ${
                          enabledSources.has(source.name)
                            ? 'bg-green-900/30 border-green-700 hover:bg-green-900/50'
                            : 'bg-gray-800/50 border-gray-700 opacity-60 hover:opacity-80'
                        }`}
                        onClick={() => {
                          const newSet = new Set(enabledSources);
                          if (newSet.has(source.name)) {
                            newSet.delete(source.name);
                          } else {
                            newSet.add(source.name);
                          }
                          setEnabledSources(newSet);
                        }}
                      >
                        <input
                          type="checkbox"
                          checked={enabledSources.has(source.name)}
                          onChange={() => {}}
                          className="w-4 h-4 bg-gray-800 border-gray-700 rounded pointer-events-none"
                        />
                        <div className="flex-1 min-w-0">
                          <div className="text-sm font-medium text-white truncate">{source.name}</div>
                          <div className="text-xs text-gray-500">{source.count} channels</div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Channel Categories */}
              {channelStats && channelStats.categories && channelStats.categories.length > 0 && (
                <div>
                  <h3 className="text-lg font-medium text-white mb-4">📁 Channel Categories</h3>
                  <p className="text-sm text-gray-400 mb-3">Toggle categories on/off to filter which channels appear in Live TV.</p>
                  <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
                    {channelStats.categories.map((category) => (
                      <div
                        key={category.name}
                        className={`flex items-center gap-2 p-3 rounded-lg border cursor-pointer transition-colors ${
                          enabledCategories.has(category.name)
                            ? 'bg-blue-900/30 border-blue-700 hover:bg-blue-900/50'
                            : 'bg-gray-800/50 border-gray-700 opacity-60 hover:opacity-80'
                        }`}
                        onClick={() => {
                          const newSet = new Set(enabledCategories);
                          if (newSet.has(category.name)) {
                            newSet.delete(category.name);
                          } else {
                            newSet.add(category.name);
                          }
                          setEnabledCategories(newSet);
                        }}
                      >
                        <input
                          type="checkbox"
                          checked={enabledCategories.has(category.name)}
                          onChange={() => {}}
                          className="w-4 h-4 bg-gray-800 border-gray-700 rounded pointer-events-none"
                        />
                        <div className="flex-1 min-w-0">
                          <div className="text-sm font-medium text-white truncate">{category.name}</div>
                          <div className="text-xs text-gray-500">{category.count} channels</div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <hr className="border-gray-700" />

              {/* Add New M3U Source */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">➕ Add M3U Source</h3>
                <div className="flex flex-col gap-3">
                  <div>
                    <label className="block text-sm text-gray-400 mb-1">Source Name</label>
                    <input
                      type="text"
                      value={newM3uName}
                      onChange={(e) => setNewM3uName(e.target.value)}
                      placeholder="e.g., My IPTV Provider"
                      className="w-full p-3 bg-gray-800 border border-gray-700 rounded-lg text-white focus:ring-2 focus:ring-blue-500"
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-gray-400 mb-1">M3U URL</label>
                    <input
                      type="url"
                      value={newM3uUrl}
                      onChange={(e) => setNewM3uUrl(e.target.value)}
                      placeholder="https://example.com/playlist.m3u"
                      className="w-full p-3 bg-gray-800 border border-gray-700 rounded-lg text-white focus:ring-2 focus:ring-blue-500"
                    />
                  </div>
                  <button
                    onClick={addM3uSource}
                    className="flex items-center justify-center gap-2 px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 w-fit"
                  >
                    <Plus className="w-4 h-4" />
                    Add Source
                  </button>
                </div>
              </div>

              {/* Current M3U Sources */}
              {m3uSources.length > 0 && (
                <div>
                  <h3 className="text-lg font-medium text-white mb-4">🔗 Your Custom M3U Sources</h3>
                  <div className="space-y-2">
                    {m3uSources.map((source, index) => (
                      <div
                        key={index}
                        className={`flex items-center justify-between p-3 rounded-lg border ${
                          source.enabled
                            ? 'bg-gray-800 border-gray-700'
                            : 'bg-gray-800/50 border-gray-800 opacity-60'
                        }`}
                      >
                        <div className="flex items-center gap-3 flex-1 min-w-0">
                          <input
                            type="checkbox"
                            checked={source.enabled}
                            onChange={() => toggleM3uSource(index)}
                            className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                          />
                          <div className="min-w-0">
                            <div className="font-medium text-white truncate">{source.name}</div>
                            <div className="text-xs text-gray-500 truncate">{source.url}</div>
                          </div>
                        </div>
                        <button
                          onClick={() => removeM3uSource(index)}
                          className="p-1 text-red-400 hover:text-red-300"
                        >
                          <X className="w-4 h-4" />
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div className="p-4 bg-yellow-900/20 border border-yellow-800 rounded-lg">
                <h4 className="text-yellow-400 font-medium mb-2">💡 Tips</h4>
                <ul className="text-sm text-gray-300 space-y-1 list-disc list-inside">
                  <li>Local M3U file: <code className="text-xs bg-gray-800 px-1 rounded">./channels/m3u_formatted.dat</code></li>
                  <li>M3U sources are loaded when the server starts</li>
                  <li>After adding/removing sources, save and restart the server</li>
                  <li>Duplicate channels (same name) are automatically merged</li>
                </ul>
              </div>
            </div>
          </div>
        )}

        {/* Filters Tab */}
        {activeTab === 'filters' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-blue-900/30 border border-blue-800 rounded-lg">
                <h3 className="text-blue-400 font-medium mb-2">🔍 Filter Settings</h3>
                <p className="text-sm text-gray-400">
                  Filter out unwanted releases by release group, language, or quality. 
                  Separate multiple patterns with <code className="text-xs bg-gray-800 px-1 rounded">|</code> (pipe character).
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Excluded Release Groups
                </label>
                <input
                  type="text"
                  value={settings.excluded_release_groups || ''}
                  onChange={(e) => updateSetting('excluded_release_groups', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                  placeholder="TVHUB|FILM"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Block releases from specific groups. Example: <code className="bg-gray-800 px-1 rounded">TVHUB|FILM</code> blocks Russian releases like "Movie.TVHUB.FILM.mkv"
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Excluded Language Tags
                </label>
                <input
                  type="text"
                  value={settings.excluded_language_tags || ''}
                  onChange={(e) => updateSetting('excluded_language_tags', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                  placeholder="RUSSIAN|RUS|HINDI|HIN|GERMAN|GER|FRENCH|FRE|ITALIAN|ITA|SPANISH|SPA|LATINO"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Block releases with language indicators in filename. Example: <code className="bg-gray-800 px-1 rounded">RUSSIAN|RUS|HINDI|GERMAN</code>
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Excluded Qualities
                </label>
                <input
                  type="text"
                  value={settings.excluded_qualities || ''}
                  onChange={(e) => updateSetting('excluded_qualities', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                  placeholder="REMUX|HDR|DV|Dolby.?Vision|3D|CAM|TS|SCR|HDTS|HDCAM|TELESYNC|TELECINE|TC"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Block certain quality types. Example: <code className="bg-gray-800 px-1 rounded">REMUX|HDR|CAM|TS</code> blocks REMUX (too large), HDR (compatibility), CAM/TS (low quality)
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Custom Exclude Patterns (Advanced)
                </label>
                <input
                  type="text"
                  value={settings.custom_exclude_patterns || ''}
                  onChange={(e) => updateSetting('custom_exclude_patterns', e.target.value)}
                  className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                  placeholder="Sample|Trailer|\\[Dual\\]"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Custom regex patterns. Example: <code className="bg-gray-800 px-1 rounded">Sample|Trailer</code> blocks sample files and trailers
                </p>
              </div>

              {/* Filter Preview */}
              <div className="pt-6 border-t border-gray-700">
                <h3 className="text-md font-medium text-gray-300 mb-4">Filter Preview</h3>
                <p className="text-xs text-gray-500 mb-3">These release names would be blocked:</p>
                <div className="bg-gray-800 rounded-lg p-4 font-mono text-sm space-y-1">
                  {(settings.excluded_release_groups || '').split('|').filter(Boolean).length > 0 && (
                    <>
                      <div className="text-red-400">❌ Frontier.Crucible.2025.TVHUB.FILM.WEB.720p.mkv</div>
                    </>
                  )}
                  {(settings.excluded_language_tags || '').toLowerCase().includes('russian') && (
                    <>
                      <div className="text-red-400">❌ Movie.2025.RUSSIAN.1080p.BluRay.mkv</div>
                      <div className="text-red-400">❌ Film.2025.RUS.DUB.WEB-DL.720p.mkv</div>
                    </>
                  )}
                  {(settings.excluded_qualities || '').toLowerCase().includes('remux') && (
                    <div className="text-red-400">❌ Movie.2025.REMUX.2160p.BluRay.mkv</div>
                  )}
                  {(settings.excluded_qualities || '').toLowerCase().includes('hdr') && (
                    <div className="text-red-400">❌ Film.2025.HDR.DV.2160p.mkv</div>
                  )}
                  {(settings.excluded_qualities || '').toLowerCase().includes('cam') && (
                    <>
                      <div className="text-red-400">❌ Movie.2025.CAM.720p.mkv</div>
                    </>
                  )}
                  {(settings.excluded_qualities || '').toLowerCase().includes('telesync') && (
                    <div className="text-red-400">❌ Film.2025.TELESYNC.720p.mkv</div>
                  )}
                  {(settings.excluded_language_tags || '').toLowerCase().includes('hindi') && (
                    <div className="text-red-400">❌ Movie.2025.HINDI.1080p.WEB-DL.mkv</div>
                  )}
                  {(settings.excluded_language_tags || '').toLowerCase().includes('german') && (
                    <div className="text-red-400">❌ Film.2025.GERMAN.DL.1080p.mkv</div>
                  )}
                  {!(settings.excluded_release_groups || settings.excluded_language_tags || settings.excluded_qualities || settings.custom_exclude_patterns) && (
                    <div className="text-gray-500">No filters configured. Add patterns above to see preview.</div>
                  )}
                </div>
              </div>

              {/* Common Presets */}
              <div className="pt-6 border-t border-gray-700">
                <h3 className="text-md font-medium text-gray-300 mb-4">Common Presets</h3>
                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={() => {
                      updateSetting('excluded_language_tags', 'RUSSIAN|RUS|HINDI|HIN|GERMAN|GER|FRENCH|FRE|ITALIAN|ITA|SPANISH|SPA|LATINO|PORTUGUESE|POR|KOREAN|KOR|JAPANESE|JAP|CHINESE|CHI|ARABIC|ARA|TURKISH|TUR|POLISH|POL|DUTCH|DUT|THAI|VIETNAMESE|INDONESIAN');
                    }}
                    className="px-3 py-2 bg-blue-600/20 text-blue-400 rounded-lg hover:bg-blue-600/30 transition-colors border border-blue-600/30 text-sm"
                  >
                    🇺🇸 English Only
                  </button>
                  <button
                    onClick={() => {
                      updateSetting('excluded_qualities', 'CAM|TS|SCR|HDTS|HDCAM|TELESYNC|TELECINE|TC|DVDSCR|R5|R6');
                    }}
                    className="px-3 py-2 bg-orange-600/20 text-orange-400 rounded-lg hover:bg-orange-600/30 transition-colors border border-orange-600/30 text-sm"
                  >
                    🎬 No CAM/TS
                  </button>
                  <button
                    onClick={() => {
                      updateSetting('excluded_qualities', 'REMUX|HDR|DV|Dolby.?Vision|3D|ATMOS|TrueHD|DTS-HD');
                    }}
                    className="px-3 py-2 bg-green-600/20 text-green-400 rounded-lg hover:bg-green-600/30 transition-colors border border-green-600/30 text-sm"
                  >
                    📺 Player Friendly
                  </button>
                  <button
                    onClick={() => {
                      updateSetting('excluded_release_groups', '');
                      updateSetting('excluded_language_tags', '');
                      updateSetting('excluded_qualities', '');
                      updateSetting('custom_exclude_patterns', '');
                    }}
                    className="px-3 py-2 bg-red-600/20 text-red-400 rounded-lg hover:bg-red-600/30 transition-colors border border-red-600/30 text-sm"
                  >
                    ❌ Clear All
                  </button>
                </div>
                <p className="text-xs text-gray-500 mt-3">
                  Presets will replace the corresponding filter field. Click "Save Changes" to apply.
                </p>
              </div>

              {/* Stream Sorting Section */}
              <div className="pt-6 border-t border-gray-700">
                <h3 className="text-md font-medium text-gray-300 mb-4">🔢 Stream Sorting & Selection</h3>
                <p className="text-xs text-gray-500 mb-4">
                  Configure how streams are sorted and which one is selected for playback.
                </p>

                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <label className="block text-sm font-medium text-gray-300">Enable Release Filters</label>
                      <p className="text-xs text-gray-500">Apply the filter patterns above when selecting streams</p>
                    </div>
                    <button
                      onClick={() => updateSetting('enable_release_filters', !settings.enable_release_filters)}
                      className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                        settings.enable_release_filters ? 'bg-green-600' : 'bg-gray-600'
                      }`}
                    >
                      <span
                        className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                          settings.enable_release_filters ? 'translate-x-6' : 'translate-x-1'
                        }`}
                      />
                    </button>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Sort Priority Order
                    </label>
                    <select
                      value={settings.stream_sort_order || 'quality,size,seeders'}
                      onChange={(e) => updateSetting('stream_sort_order', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      <option value="quality,size,seeders">Quality → Size → Seeders (Default)</option>
                      <option value="quality,seeders,size">Quality → Seeders → Size</option>
                      <option value="size,quality,seeders">Size → Quality → Seeders</option>
                      <option value="seeders,quality,size">Seeders → Quality → Size</option>
                    </select>
                    <p className="text-xs text-gray-500 mt-1">
                      Order of priority when comparing streams. First field has highest priority.
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Selection Preference
                    </label>
                    <select
                      value={settings.stream_sort_prefer || 'best'}
                      onChange={(e) => updateSetting('stream_sort_prefer', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      <option value="best">Best Quality (Highest quality, largest size)</option>
                      <option value="smallest">Smallest File (Lowest size, data saver)</option>
                      <option value="balanced">Balanced (Good quality, reasonable size)</option>
                    </select>
                    <p className="text-xs text-gray-500 mt-1">
                      "Best" selects highest values, "Smallest" selects lowest values for each sort field.
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Services Tab */}
        {activeTab === 'services' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-blue-900/30 border border-blue-800 rounded-lg">
                <h3 className="text-blue-400 font-medium mb-2">⚙️ Background Services</h3>
                <p className="text-sm text-gray-400">
                  Monitor and control background tasks. Services run automatically at their configured intervals,
                  or you can trigger them manually. Data refreshes every 5 seconds while on this tab.
                </p>
              </div>

              <div className="space-y-4">
                {services.length === 0 ? (
                  <div className="text-gray-500 text-center py-8">
                    <Activity className="w-12 h-12 mx-auto mb-2 opacity-50" />
                    <p>No services registered. Start the worker process to enable background tasks.</p>
                  </div>
                ) : (
                  services.map((service) => (
                    <div
                      key={service.name}
                      className={`p-4 rounded-lg border ${
                        service.running
                          ? 'bg-blue-900/20 border-blue-700'
                          : service.enabled
                          ? 'bg-gray-800/50 border-gray-700'
                          : 'bg-gray-800/30 border-gray-700 opacity-60'
                      }`}
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex-1">
                          <div className="flex items-center gap-3">
                            <h4 className="text-white font-medium">{formatServiceName(service.name)}</h4>
                            {service.running && (
                              <span className="flex items-center gap-1 text-xs bg-blue-500/30 text-blue-400 px-2 py-0.5 rounded">
                                <RefreshCw className="w-3 h-3 animate-spin" />
                                Running
                              </span>
                            )}
                            {!service.enabled && (
                              <span className="text-xs bg-gray-600/50 text-gray-400 px-2 py-0.5 rounded">
                                Disabled
                              </span>
                            )}
                          </div>
                          <p className="text-sm text-gray-400 mt-1">{service.description}</p>
                          
                          <div className="flex flex-wrap gap-4 mt-3 text-xs text-gray-500">
                            <div className="flex items-center gap-1">
                              <Clock className="w-3 h-3" />
                              <span>Interval: {service.interval}</span>
                            </div>
                            <div>
                              Last run: {formatDateTime(service.last_run)}
                            </div>
                            <div>
                              Next run: {formatDateTime(service.next_run)}
                            </div>
                            <div>
                              Run count: {service.run_count}
                            </div>
                          </div>

                          {service.last_error && (
                            <div className="mt-2 text-xs text-red-400 bg-red-900/20 px-2 py-1 rounded">
                              Last error: {service.last_error}
                            </div>
                          )}
                        </div>

                        <div className="flex items-center gap-2 ml-4">
                          <button
                            onClick={() => toggleServiceEnabled(service.name, !service.enabled)}
                            className={`px-3 py-1.5 text-xs rounded transition-colors ${
                              service.enabled
                                ? 'bg-gray-700 text-gray-300 hover:bg-gray-600'
                                : 'bg-green-600/20 text-green-400 hover:bg-green-600/30'
                            }`}
                          >
                            {service.enabled ? 'Disable' : 'Enable'}
                          </button>
                          <button
                            onClick={() => triggerService(service.name)}
                            disabled={service.running || triggeringService === service.name}
                            className={`flex items-center gap-1 px-3 py-1.5 text-xs rounded transition-colors ${
                              service.running || triggeringService === service.name
                                ? 'bg-gray-700 text-gray-500 cursor-not-allowed'
                                : 'bg-blue-600 text-white hover:bg-blue-700'
                            }`}
                          >
                            <Play className="w-3 h-3" />
                            {triggeringService === service.name ? 'Triggering...' : 'Run Now'}
                          </button>
                        </div>
                      </div>
                    </div>
                  ))
                )}
              </div>

              {/* Service Legend */}
              <div className="pt-4 border-t border-gray-700">
                <h4 className="text-sm font-medium text-gray-400 mb-3">Service Information</h4>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs text-gray-500">
                  <div>
                    <strong className="text-gray-400">Playlist Generation:</strong> Creates M3U8 playlist file with all library content for IPTV players
                  </div>
                  <div>
                    <strong className="text-gray-400">Cache Cleanup:</strong> Removes expired stream links and temporary data
                  </div>
                  <div>
                    <strong className="text-gray-400">EPG Update:</strong> Fetches program guide data for Live TV channels
                  </div>
                  <div>
                    <strong className="text-gray-400">Channel Refresh:</strong> Reloads Live TV channels from configured M3U sources
                  </div>
                  <div>
                    <strong className="text-gray-400">MDBList Sync:</strong> Imports content from your MDBList watchlists
                  </div>
                  <div>
                    <strong className="text-gray-400">Collection Sync:</strong> Adds missing movies from incomplete collections
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Xtream Tab */}
        {activeTab === 'xtream' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-purple-900/30 border border-purple-800 rounded-lg">
                <h3 className="text-purple-400 font-medium mb-2">📡 Xtream Codes API</h3>
                <p className="text-sm text-gray-300">
                  StreamArr exposes an Xtream Codes compatible API that can be used with IPTV players like TiviMate, XCIPTV, or OTT Navigator.
                  Use the connection details below to configure your player.
                </p>
              </div>

              {/* Connection Details */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">🔗 Connection Details</h3>
                <div className="bg-gray-800 rounded-lg p-4 space-y-4">
                  <div>
                    <label className="block text-sm text-gray-400 mb-1">Server URL</label>
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={`http://${settings.host || 'localhost'}:${settings.server_port || 8080}`}
                        readOnly
                        className="flex-1 p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                      />
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(`http://${settings.host || 'localhost'}:${settings.server_port || 8080}`);
                          setMessage('Server URL copied to clipboard');
                          setTimeout(() => setMessage(''), 2000);
                        }}
                        className="px-3 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                      >
                        Copy
                      </button>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm text-gray-400 mb-1">Username</label>
                      <div className="flex gap-2">
                        <input
                          type="text"
                          value="streamarr"
                          readOnly
                          className="flex-1 p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                        />
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText('streamarr');
                            setMessage('Username copied to clipboard');
                            setTimeout(() => setMessage(''), 2000);
                          }}
                          className="px-3 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                        >
                          Copy
                        </button>
                      </div>
                    </div>
                    <div>
                      <label className="block text-sm text-gray-400 mb-1">Password</label>
                      <div className="flex gap-2">
                        <input
                          type="text"
                          value="streamarr"
                          readOnly
                          className="flex-1 p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                        />
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText('streamarr');
                            setMessage('Password copied to clipboard');
                            setTimeout(() => setMessage(''), 2000);
                          }}
                          className="px-3 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                        >
                          Copy
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              {/* API Endpoints */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">🌐 API Endpoints</h3>
                <div className="bg-gray-800 rounded-lg p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Player API</div>
                      <code className="text-xs text-gray-400">/player_api.php</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get Live Categories</div>
                      <code className="text-xs text-gray-400">/player_api.php?action=get_live_categories</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get Live Streams</div>
                      <code className="text-xs text-gray-400">/player_api.php?action=get_live_streams</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get VOD Categories</div>
                      <code className="text-xs text-gray-400">/player_api.php?action=get_vod_categories</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get VOD Streams</div>
                      <code className="text-xs text-gray-400">/player_api.php?action=get_vod_streams</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get Series Categories</div>
                      <code className="text-xs text-gray-400">/player_api.php?action=get_series_categories</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get Series</div>
                      <code className="text-xs text-gray-400">/player_api.php?action=get_series</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                </div>
              </div>

              {/* Quick Setup Guide */}
              <div className="p-4 bg-blue-900/20 border border-blue-800 rounded-lg">
                <h4 className="text-blue-400 font-medium mb-3">📱 Quick Setup for IPTV Players</h4>
                <div className="space-y-3 text-sm text-gray-300">
                  <div>
                    <strong className="text-white">TiviMate / XCIPTV / OTT Navigator:</strong>
                    <ol className="list-decimal list-inside mt-1 space-y-1 ml-2">
                      <li>Select "Xtream Codes" or "Xtream Codes API"</li>
                      <li>Enter the Server URL, Username, and Password from above</li>
                      <li>Save and refresh to load your channels</li>
                    </ol>
                  </div>
                  <div>
                    <strong className="text-white">M3U URL (Alternative):</strong>
                    <div className="mt-1 flex gap-2">
                      <code className="flex-1 text-xs bg-gray-800 px-2 py-1 rounded overflow-x-auto">
                        {`http://${settings.host || 'localhost'}:${settings.server_port || 8080}/get.php?username=streamarr&password=streamarr&type=m3u_plus&output=ts`}
                      </code>
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(`http://${settings.host || 'localhost'}:${settings.server_port || 8080}/get.php?username=streamarr&password=streamarr&type=m3u_plus&output=ts`);
                          setMessage('M3U URL copied to clipboard');
                          setTimeout(() => setMessage(''), 2000);
                        }}
                        className="px-2 py-1 bg-blue-600 text-white text-xs rounded hover:bg-blue-700"
                      >
                        Copy
                      </button>
                    </div>
                  </div>
                </div>
              </div>

              {/* EPG Info */}
              <div className="p-4 bg-green-900/20 border border-green-800 rounded-lg">
                <h4 className="text-green-400 font-medium mb-2">📺 EPG (Electronic Program Guide)</h4>
                <p className="text-sm text-gray-300 mb-2">
                  EPG data is available for Live TV channels. Use this URL in your IPTV player:
                </p>
                <div className="flex gap-2">
                  <code className="flex-1 text-xs bg-gray-800 px-2 py-1 rounded overflow-x-auto text-gray-300">
                    {`http://${settings.host || 'localhost'}:${settings.server_port || 8080}/xmltv.php`}
                  </code>
                  <button
                    onClick={() => {
                      navigator.clipboard.writeText(`http://${settings.host || 'localhost'}:${settings.server_port || 8080}/xmltv.php`);
                      setMessage('EPG URL copied to clipboard');
                      setTimeout(() => setMessage(''), 2000);
                    }}
                    className="px-2 py-1 bg-green-600 text-white text-xs rounded hover:bg-green-700"
                  >
                    Copy
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Notifications Tab */}
        {activeTab === 'notifications' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              <div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="enable_notifications"
                    checked={settings.enable_notifications || false}
                    onChange={(e) => updateSetting('enable_notifications', e.target.checked)}
                    className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                  />
                  <label htmlFor="enable_notifications" className="text-sm font-medium text-gray-300">
                    Enable Notifications
                  </label>
                </div>
                <p className="text-xs text-gray-500 mt-1 ml-6">Send alerts when new content is added or errors occur</p>
              </div>

              <div className="pt-6 border-t border-gray-700">
                <h3 className="text-md font-medium text-gray-300 mb-4">Discord</h3>
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2">
                    Discord Webhook URL
                  </label>
                  <input
                    type="text"
                    value={settings.discord_webhook_url || ''}
                    onChange={(e) => updateSetting('discord_webhook_url', e.target.value)}
                    className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    placeholder="https://discord.com/api/webhooks/..."
                  />
                  <p className="text-xs text-gray-500 mt-1">
                    Create in Discord: Server Settings → Integrations → Webhooks → New Webhook
                  </p>
                </div>
              </div>

              <div className="pt-6 border-t border-gray-700">
                <h3 className="text-md font-medium text-gray-300 mb-4">Telegram</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Telegram Bot Token
                    </label>
                    <input
                      type="text"
                      value={settings.telegram_bot_token || ''}
                      onChange={(e) => updateSetting('telegram_bot_token', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Get from @BotFather on Telegram
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Telegram Chat ID
                    </label>
                    <input
                      type="text"
                      value={settings.telegram_chat_id || ''}
                      onChange={(e) => updateSetting('telegram_chat_id', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="123456789"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Your user ID or group chat ID. Get from @userinfobot
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Advanced Tab */}
        {activeTab === 'advanced' && (
          <div className="bg-gray-900 rounded-lg p-6 border border-gray-800">
            <div className="space-y-6">
              {/* Server Settings */}
              <div>
                <h3 className="text-md font-medium text-gray-300 mb-4">Server Settings</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Server Port
                    </label>
                    <input
                      type="number"
                      value={settings.server_port || 8080}
                      onChange={(e) => updateSetting('server_port', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="8080"
                    />
                    <p className="text-xs text-gray-500 mt-1">Port the API server listens on. Requires restart.</p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Host Binding
                    </label>
                    <input
                      type="text"
                      value={settings.host || '0.0.0.0'}
                      onChange={(e) => updateSetting('host', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="0.0.0.0"
                    />
                    <p className="text-xs text-gray-500 mt-1">0.0.0.0 = all interfaces, 127.0.0.1 = localhost only</p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Auto Cache Interval (hours)
                    </label>
                    <input
                      type="number"
                      value={settings.auto_cache_interval_hours || 6}
                      onChange={(e) => updateSetting('auto_cache_interval_hours', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      min="1"
                      max="168"
                    />
                    <p className="text-xs text-gray-500 mt-1">How often to refresh library metadata and sync MDBLists (1-168 hours)</p>
                  </div>

                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="debug"
                      checked={settings.debug || false}
                      onChange={(e) => updateSetting('debug', e.target.checked)}
                      className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                    />
                    <label htmlFor="debug" className="text-sm text-gray-300">
                      Enable Debug Mode
                    </label>
                  </div>
                  <p className="text-xs text-gray-500 ml-6 -mt-2">Verbose logging for troubleshooting</p>
                </div>
              </div>

              {/* Proxy Settings */}
              <div className="pt-6 border-t border-gray-700">
                <h3 className="text-md font-medium text-gray-300 mb-4">Proxy Settings</h3>
                <div className="space-y-4">
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="use_http_proxy"
                      checked={settings.use_http_proxy || false}
                      onChange={(e) => updateSetting('use_http_proxy', e.target.checked)}
                      className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                    />
                    <label htmlFor="use_http_proxy" className="text-sm text-gray-300">
                      Use HTTP Proxy
                    </label>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      HTTP Proxy URL
                    </label>
                    <input
                      type="text"
                      value={settings.http_proxy || ''}
                      onChange={(e) => updateSetting('http_proxy', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="http://proxy:8080 or socks5://proxy:1080"
                    />
                    <p className="text-xs text-gray-500 mt-1">Route outbound requests through a proxy (HTTP/SOCKS5)</p>
                  </div>
                </div>
              </div>

              {/* HeadlessVidX Settings */}
              <div className="pt-6 border-t border-gray-700">
                <h3 className="text-md font-medium text-gray-300 mb-4">HeadlessVidX Settings</h3>
                <p className="text-xs text-gray-500 mb-4">Optional browser automation for scraping protected video sources</p>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      HeadlessVidX Address
                    </label>
                    <input
                      type="text"
                      value={settings.headless_vidx_address || 'localhost:3202'}
                      onChange={(e) => updateSetting('headless_vidx_address', e.target.value)}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="localhost:3202"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Max Threads
                    </label>
                    <input
                      type="number"
                      value={settings.headless_vidx_max_threads || 5}
                      onChange={(e) => updateSetting('headless_vidx_max_threads', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      min="1"
                      max="20"
                    />
                    <p className="text-xs text-gray-500 mt-1">Concurrent browser sessions (higher = faster but more RAM)</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
