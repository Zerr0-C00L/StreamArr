import React, { useState, useEffect, useRef } from 'react';
import axios from 'axios';

// v1.2.1 - Added manual IP configuration
const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

// Create axios instance with auth token
const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add auth token to all requests
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

  stremio_addons: Array<{name: string; url: string; enabled: boolean}>;
  stream_providers: string[] | string;
  torrentio_providers: string;
  enable_quality_variants: boolean;
  show_full_stream_name: boolean;
  auto_add_collections: boolean;
  include_live_tv: boolean;
  include_adult_vod: boolean;
  import_adult_vod_from_github: boolean;
  balkan_vod_enabled: boolean;
  balkan_vod_auto_sync: boolean;
  balkan_vod_sync_interval_hours: number;
  balkan_vod_selected_categories: string[];
  iptv_import_mode: 'live_only' | 'vod_only' | 'both';
  iptv_vod_sync_interval_hours: number;
  duplicate_vod_per_provider: boolean;
  min_year: number;
  min_runtime: number;
  enable_notifications: boolean;
  discord_webhook_url: string;
  telegram_bot_token: string;
  telegram_chat_id: string;
  debug: boolean;
  server_port: number;
  host: string;
  user_set_host: string;
  mdblist_lists: string;
  http_proxy: string;
  use_http_proxy: boolean;
  headless_vidx_address: string;
  headless_vidx_max_threads: number;
  auto_cache_interval_hours: number;
  excluded_release_groups: string;
  excluded_language_tags: string;
  excluded_qualities: string;
  custom_exclude_patterns: string;
  enable_release_filters: boolean;
  stream_sort_order: string;
  stream_sort_prefer: string;
  livetv_enable_plutotv: boolean;
  livetv_validate_streams: boolean;
  livetv_enabled_sources: string[];
  livetv_enabled_categories: string[];
  only_cached_streams: boolean;
  hide_unavailable_content: boolean;
  update_branch: string;
  xtream_username: string;
  xtream_password: string;
  stremio_addon: {
    enabled: boolean;
    public_server_url: string;
    addon_name: string;
    shared_token: string;
    per_user_tokens: boolean;
    catalogs: Array<{
      id: string;
      type: string;
      name: string;
      enabled: boolean;
    }>;
    catalog_placement: string;
  };
  // Stream Checker (Phase 1 Cache) Settings
  cache_check_interval_minutes: number;
  cache_check_batch_size: number;
  cache_auto_upgrade: boolean;
  cache_min_upgrade_points: number;
  cache_max_upgrade_size_gb: number;
}

interface MDBListEntry {
  url: string;
  name?: string;
  enabled: boolean;
}

interface M3USource {
  name: string;
  url: string;
  epg_url?: string;
  enabled: boolean;
  selected_categories?: string[];
}

interface XtreamSource {
  name: string;
  server_url: string;
  username: string;
  password: string;
  enabled: boolean;
  selected_categories?: string[];
}

const Settings: React.FC = () => {
  const [settings, setSettings] = useState<SettingsData | null>(null);
  const [message, setMessage] = useState(''); // Define setMessage
  
  // Dropdown option sets
  // Set default for enable_release_filters to true if not set
  useEffect(() => {
    if (settings && settings.enable_release_filters === undefined) {
      autoSaveSetting('enable_release_filters', true);
    }
  }, [settings]);

  // State - MDBList
  const [mdbLists, setMdbLists] = useState<MDBListEntry[]>([]);

  // State - M3U
  const [m3uSources, setM3uSources] = useState<M3USource[]>([]);
  
  // State - Xtream
  const [xtreamSources, setXtreamSources] = useState<XtreamSource[]>([]);

  // Initialize
  useEffect(() => {
    fetchSettings();
    fetchUserProfile();
  }, []);

  // Auto-save MDBList changes
  const initialMdbListsLoaded = useRef(false);
  useEffect(() => {
    // Skip initial load - only save when lists change after initial load
    if (!initialMdbListsLoaded.current) {
      if (mdbLists.length > 0 || settings?.mdblist_lists) {
        initialMdbListsLoaded.current = true;
      }
      return;
    }
    
    if (!settings) return;
    
    const saveTimer = setTimeout(async () => {
      const settingsToSave = {
        ...settings,
        mdblist_lists: JSON.stringify(mdbLists),
        m3u_sources: m3uSources,
        xtream_sources: xtreamSources,
        // Removed unresolved references: 'enabledSources', 'enabledCategories'
      };
      
      try {
        await api.put('/settings', settingsToSave);
        setMessage('✅ MDBList updated & sync triggered');
        setTimeout(() => setMessage(''), 2000);
      } catch (error: any) {
        setMessage(`❌ Error saving MDBList: ${error.response?.data?.error || error.message}`);
        setTimeout(() => setMessage(''), 3000);
      }
    }, 500); // Debounce 500ms
    
    return () => clearTimeout(saveTimer);
  }, [mdbLists]);

  // ========== API Functions ==========

  const fetchUserProfile = async () => {
    try {
      const response = await api.get('/auth/profile');
      const data = response.data;
      localStorage.setItem('username', data.username || '');
    } catch (error) {
      console.error('Failed to fetch profile:', error);
      const savedUsername = localStorage.getItem('username');
      if (savedUsername) {
      }
    }
  };

  const fetchSettings = async () => {
    try {
      const response = await api.get('/settings');
      const data = response.data;
      
      if (data.xtream_username === undefined || data.xtream_username === null || data.xtream_username === '') {
        data.xtream_username = 'streamarr';
      }
      if (data.xtream_password === undefined || data.xtream_password === null || data.xtream_password === '') {
        data.xtream_password = 'streamarr';
      }
      
      setSettings(data);
      
      if (data.mdblist_lists) {
        try {
          const lists = JSON.parse(data.mdblist_lists);
          setMdbLists(Array.isArray(lists) ? lists : []);
        } catch {
          setMdbLists([]);
        }
      }
      
      if (data.m3u_sources && Array.isArray(data.m3u_sources)) {
        setM3uSources(data.m3u_sources);
      }
      
      if (data.xtream_sources && Array.isArray(data.xtream_sources)) {
        setXtreamSources(data.xtream_sources);
      }
      
      // Removed unresolved references: 'setEnabledSources', 'setEnabledCategories'
    } catch (error) {
      console.error('Failed to fetch settings:', error);
      setMessage('Failed to load settings');
    }
  };

  // Auto-save on every setting change
  const autoSaveSetting = (key: keyof SettingsData, value: any) => {
    if (!settings) return;
    const next = { ...settings, [key]: value };
    setSettings(next);
    const settingsToSave = {
      ...next,
      mdblist_lists: JSON.stringify(mdbLists),
      m3u_sources: m3uSources,
      xtream_sources: xtreamSources,
      // Removed unresolved references: 'enabledSources', 'enabledCategories'
    };
    api.put('/settings', settingsToSave)
      .then(() => {
        setMessage('✅ Setting saved');
        setTimeout(() => setMessage(''), 1500);
      })
      .catch((error: any) => {
        setMessage(`❌ Error saving: ${error.response?.data?.error || error.message}`);
        setTimeout(() => setMessage(''), 3000);
      });
  };

  // Added a function to regenerate the M3U playlist
  const regeneratePlaylist = async () => {
    try {
      const response = await fetch(
        'http://77.42.16.119:8080/get.php?username=zeroq&password=streamarrpro&type=m3u_plus&output=ts'
      );

      if (!response.ok) {
        throw new Error('Failed to regenerate playlist');
      }

      const playlist = await response.text();

      // Save or process the playlist as needed
      console.log('Playlist regenerated successfully:', playlist);
      alert('Playlist regenerated successfully!');
    } catch (error) {
      console.error('Error regenerating playlist:', error);
      alert('Failed to regenerate playlist. Please try again.');
    }
  };

  // State for version info and update check
  const [versionInfo, setVersionInfo] = useState<any>(null);
  const [updateInfo, setUpdateInfo] = useState<any>(null);
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [activeTab, setActiveTab] = useState('general');

  // Fetch version info
  useEffect(() => {
    const fetchVersionInfo = async () => {
      try {
        const response = await api.get('/version');
        setVersionInfo(response.data);
      } catch (error) {
        console.error('Failed to fetch version info:', error);
      }
    };
    fetchVersionInfo();
  }, []);

  // Check for updates
  const handleCheckUpdates = async () => {
    setCheckingUpdates(true);
    try {
      const response = await api.get('/version/check');
      setUpdateInfo(response.data);
    } catch (error) {
      console.error('Failed to check for updates:', error);
      setUpdateInfo({ error: 'Failed to check for updates' });
    } finally {
      setCheckingUpdates(false);
    }
  };

  if (!settings) {
    return (
      <div className="min-h-screen bg-[#141414] flex items-center justify-center">
        <p className="text-white">Loading settings...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#141414]">
      <div className="max-w-6xl mx-auto p-6">
        <h1 className="text-4xl font-bold text-white mb-8">Settings</h1>

        {/* Tab Navigation */}
        <div className="flex gap-2 mb-8 border-b border-white/10">
          <button
            onClick={() => setActiveTab('general')}
            className={`px-4 py-2 font-medium transition-colors ${
              activeTab === 'general'
                ? 'text-white border-b-2 border-white'
                : 'text-slate-400 hover:text-white'
            }`}
          >
            General
          </button>
          <button
            onClick={() => setActiveTab('about')}
            className={`px-4 py-2 font-medium transition-colors ${
              activeTab === 'about'
                ? 'text-white border-b-2 border-white'
                : 'text-slate-400 hover:text-white'
            }`}
          >
            About
          </button>
        </div>

        {/* Message Display */}
        {message && (
          <div className="mb-4 p-4 rounded bg-blue-900/30 text-blue-200">
            {message}
          </div>
        )}

        {/* General Tab */}
        {activeTab === 'general' && (
          <div className="space-y-6">
            <div className="bg-slate-800/50 rounded-lg p-6">
              <h2 className="text-xl font-semibold text-white mb-4">API Keys</h2>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    TMDB API Key
                  </label>
                  <input
                    type="password"
                    value={settings.tmdb_api_key || ''}
                    onChange={(e) => autoSaveSetting('tmdb_api_key', e.target.value)}
                    className="w-full px-4 py-2 bg-slate-900 text-white rounded border border-slate-700 focus:border-white focus:outline-none"
                  />
                </div>
              </div>
            </div>
          </div>
        )}

        {/* About Tab */}
        {activeTab === 'about' && (
          <div className="space-y-6">
            {/* Version Information */}
            <div className="bg-slate-800/50 rounded-lg p-6">
              <h2 className="text-2xl font-semibold text-white mb-6">Version Information</h2>
              
              {versionInfo ? (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div className="bg-slate-900/50 rounded p-4">
                    <p className="text-sm text-slate-400 mb-1">Version</p>
                    <p className="text-2xl font-bold text-white">
                      {versionInfo.current_version && versionInfo.current_version !== 'dev' 
                        ? versionInfo.current_version 
                        : 'Unknown'}
                    </p>
                  </div>
                  <div className="bg-slate-900/50 rounded p-4">
                    <p className="text-sm text-slate-400 mb-1">Commit</p>
                    <p className="text-lg font-mono text-white">
                      {versionInfo.current_commit && versionInfo.current_commit !== 'unknown' 
                        ? versionInfo.current_commit 
                        : 'Unknown'}
                    </p>
                  </div>
                  <div className="bg-slate-900/50 rounded p-4">
                    <p className="text-sm text-slate-400 mb-1">Build Date</p>
                    <p className="text-lg text-white">
                      {versionInfo.build_date && versionInfo.build_date !== 'unknown' 
                        ? new Date(versionInfo.build_date).toLocaleString() 
                        : 'Unknown'}
                    </p>
                  </div>
                  <div className="bg-slate-900/50 rounded p-4">
                    <p className="text-sm text-slate-400 mb-1">Update Channel</p>
                    <p className="text-lg text-white capitalize">
                      {settings.update_branch || 'main'}
                    </p>
                  </div>
                </div>
              ) : (
                <p className="text-slate-400">Loading version information...</p>
              )}
            </div>

            {/* Update Check */}
            <div className="bg-slate-800/50 rounded-lg p-6">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-2xl font-semibold text-white">Check for Updates</h2>
                <button
                  onClick={handleCheckUpdates}
                  disabled={checkingUpdates}
                  className="px-6 py-2 bg-white text-black font-semibold rounded-lg hover:bg-white/90 disabled:bg-slate-600 disabled:text-slate-400 transition-colors"
                >
                  {checkingUpdates ? 'Checking...' : 'Check for Update'}
                </button>
              </div>

              {updateInfo ? (
                <div className="bg-slate-900/50 rounded p-4">
                  {updateInfo.error ? (
                    <p className="text-red-400">{updateInfo.error}</p>
                  ) : (
                    <>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
                        <div>
                          <p className="text-sm text-slate-400">Current Version</p>
                          <p className="text-lg text-white font-semibold">
                            {updateInfo.current_version}
                          </p>
                        </div>
                        <div>
                          <p className="text-sm text-slate-400">Latest Available</p>
                          <p className="text-lg text-white font-semibold">
                            {updateInfo.latest_version}
                          </p>
                        </div>
                      </div>
                      {updateInfo.update_available ? (
                        <div className="p-3 rounded bg-blue-900/30 text-blue-200">
                          <p className="font-semibold">Update Available</p>
                          {updateInfo.changelog && (
                            <p className="text-sm mt-1">{updateInfo.changelog}</p>
                          )}
                        </div>
                      ) : (
                        <div className="p-3 rounded bg-green-900/30 text-green-200">
                          <p className="font-semibold">✓ You are on the latest version!</p>
                        </div>
                      )}
                    </>
                  )}
                </div>
              ) : (
                <p className="text-slate-400">Click "Check for Update" to see if a new version is available</p>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default Settings;