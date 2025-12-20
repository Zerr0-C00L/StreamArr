import { useState, useEffect } from 'react';
import { Save, Key, Layers, Settings as SettingsIcon, Bell, Code, Plus, X, Tv, Server, Activity, Play, Clock, RefreshCw, Filter, Database, Trash2, AlertTriangle, Info, Github, Download, ExternalLink, CheckCircle, AlertCircle, Film, User, Camera, Loader, Search } from 'lucide-react';
import axios from 'axios';

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

  // Comet Provider Settings
  comet_enabled: boolean;
  comet_indexers: string;
  comet_only_show_cached: boolean;
  comet_max_results: number;
  comet_sort_by: string;
  comet_excluded_qualities: string;
  comet_priority_languages: string;
  comet_max_size: string;

  stremio_addons: Array<{name: string; url: string; enabled: boolean}>;
  stream_providers: string[] | string; // Legacy, will be removed
  torrentio_providers: string; // Legacy, will be removed
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
  livetv_validate_streams: boolean;
  livetv_enabled_sources: string[];
  livetv_enabled_categories: string[];
  // IPTV-org settings
  iptv_org_enabled: boolean;
  iptv_org_countries: string[];
  iptv_org_languages: string[];
  iptv_org_categories: string[];
  // Content filter settings
  only_released_content: boolean;
  hide_unavailable_content: boolean;
  // Update settings
  update_branch: string;
  // Xtream API settings
  xtream_username: string;
  xtream_password: string;
  // Stremio Addon settings
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

interface SourceStatus {
  url: string;
  online: boolean;
  status_code: number;
  message: string;
  checking?: boolean;
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
  progress: number;
  progress_message: string;
  items_processed: number;
  items_total: number;
}

type TabType = 'account' | 'api' | 'addons' | 'quality' | 'content' | 'livetv' | 'stremio' | 'filters' | 'services' | 'xtream' | 'notifications' | 'database' | 'blacklist' | 'about';

interface VersionInfo {
  current_version: string;
  current_commit: string;
  build_date: string;
  latest_version: string;
  latest_commit: string;
  latest_date: string;
  update_available: boolean;
  changelog: string;
  update_branch?: string;
}

export default function Settings() {
  const [settings, setSettings] = useState<SettingsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');
  const [activeTab, setActiveTab] = useState<TabType>('account');
  const [newListUrl, setNewListUrl] = useState('');
  const [mdbLists, setMdbLists] = useState<MDBListEntry[]>([]);
  const [userLists, setUserLists] = useState<Array<{id: number; name: string; slug: string; items: number; user_name?: string}>>([]);
  const [mdbUsername, setMdbUsername] = useState('');
  const [fetchingUserLists, setFetchingUserLists] = useState(false);
  const [m3uSources, setM3uSources] = useState<M3USource[]>([]);
  const [xtreamSources, setXtreamSources] = useState<XtreamSource[]>([]);
  const [sourceStatuses, setSourceStatuses] = useState<Map<string, SourceStatus>>(new Map());
  const [checkingAllSources, setCheckingAllSources] = useState(false);
  const [newM3uName, setNewM3uName] = useState('');
  const [newM3uUrl, setNewM3uUrl] = useState('');
  const [newM3uEpg, setNewM3uEpg] = useState('');
  const [newM3uCategories, setNewM3uCategories] = useState<string[]>([]);
  const [availableCategories, setAvailableCategories] = useState<Array<{name: string; count: number}>>([]);
  const [loadingCategories, setLoadingCategories] = useState(false);
  const [showCategoryModal, setShowCategoryModal] = useState(false);
  const [newXtreamName, setNewXtreamName] = useState('');
  const [newXtreamUrl, setNewXtreamUrl] = useState('');
  const [newXtreamUsername, setNewXtreamUsername] = useState('');
  const [newXtreamPassword, setNewXtreamPassword] = useState('');
  const [newXtreamCategories, setNewXtreamCategories] = useState<string[]>([]);
  const [availableXtreamCategories, setAvailableXtreamCategories] = useState<Array<{name: string; count: number}>>([]);
  const [loadingXtreamCategories, setLoadingXtreamCategories] = useState(false);
  const [showXtreamCategoryModal, setShowXtreamCategoryModal] = useState(false);
  const [newBalkanCategories, setNewBalkanCategories] = useState<string[]>([]);
  const [availableBalkanCategories, setAvailableBalkanCategories] = useState<Array<{name: string; count: number}>>([]);
  const [loadingBalkanCategories, setLoadingBalkanCategories] = useState(false);
  const [showBalkanCategoryModal, setShowBalkanCategoryModal] = useState(false);
  const [channelStats, setChannelStats] = useState<ChannelStats | null>(null);
  const [enabledCategories, setEnabledCategories] = useState<Set<string>>(new Set());
  const [enabledSources, setEnabledSources] = useState<Set<string>>(new Set());
  const [services, setServices] = useState<ServiceStatus[]>([]);
  const [triggeringService, setTriggeringService] = useState<string | null>(null);
  const [dbOperation, setDbOperation] = useState<string | null>(null);
  const [confirmDialog, setConfirmDialog] = useState<{ action: string; title: string; message: string } | null>(null);
  const [dbStats, setDbStats] = useState<{ movies: number; series: number; episodes: number; streams: number; collections: number } | null>(null);
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null);
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [installingUpdate, setInstallingUpdate] = useState(false);
  
  // Account profile state
  const [profileUsername, setProfileUsername] = useState('');
  const [profileEmail, setProfileEmail] = useState('');
  const [profileAvatar, setProfileAvatar] = useState<string | null>(null);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [profileMessage, setProfileMessage] = useState('');

  // Blacklist state
  const [blacklist, setBlacklist] = useState<Array<{
    id: number;
    tmdb_id: number;
    type: string;
    title: string;
    reason: string;
    created_at: string;
  }>>([]);
  const [loadingBlacklist, setLoadingBlacklist] = useState(false);
  const [removingFromBlacklist, setRemovingFromBlacklist] = useState<number | null>(null);

  useEffect(() => {
    fetchSettings();
    fetchChannelStats();
    fetchServices();
    fetchDbStats();
    fetchVersionInfo();
    fetchUserProfile();
    
    // Load avatar from localStorage as fallback
    const savedAvatar = localStorage.getItem('profile_picture');
    if (savedAvatar) {
      setProfileAvatar(savedAvatar);
    }
  }, []);

  // Fetch blacklist when on blacklist tab
  useEffect(() => {
    if (activeTab === 'blacklist') {
      fetchBlacklist();
    }
  }, [activeTab]);

  // Poll services status when on services tab
  useEffect(() => {
    if (activeTab === 'services') {
      const interval = setInterval(() => {
        fetchServices();
      }, 5000);
      return () => clearInterval(interval);
    }
  }, [activeTab]);

  const fetchServices = async () => {
    try {
      const response = await api.get('/services');
      const data = response.data;
      // Sort services by name to maintain consistent order
      const sortedServices = (data.services || []).sort((a: ServiceStatus, b: ServiceStatus) => 
        a.name.localeCompare(b.name)
      );
      setServices(sortedServices);
    } catch (error) {
      console.error('Failed to fetch services:', error);
    }
  };

  const triggerService = async (serviceName: string) => {
    setTriggeringService(serviceName);
    try {
      await api.post(`/services/${serviceName}/trigger?name=${serviceName}`);
      setMessage(`✅ Service "${serviceName}" triggered successfully`);
      setTimeout(() => setMessage(''), 3000);
      // Refresh services after a short delay
      setTimeout(fetchServices, 500);
    } catch (error: any) {
      setMessage(`❌ Failed to trigger service: ${error.response?.data?.error || error.message}`);
      setTimeout(() => setMessage(''), 5000);
    }
    setTriggeringService(null);
  };

  const fetchDbStats = async () => {
    try {
      const response = await api.get('/database/stats');
      setDbStats(response.data);
    } catch (error) {
      console.error('Failed to fetch database stats:', error);
    }
  };

  const fetchBlacklist = async () => {
    setLoadingBlacklist(true);
    try {
      const response = await api.get('/blacklist');
      setBlacklist(response.data.blacklist || []);
    } catch (error) {
      console.error('Failed to fetch blacklist:', error);
      setMessage('❌ Failed to load blacklist');
      setTimeout(() => setMessage(''), 3000);
    }
    setLoadingBlacklist(false);
  };

  const removeFromBlacklist = async (id: number) => {
    setRemovingFromBlacklist(id);
    try {
      await api.delete(`/blacklist/${id}`);
      setMessage('✅ Item removed from blacklist');
      setTimeout(() => setMessage(''), 3000);
      fetchBlacklist(); // Refresh the list
    } catch (error: any) {
      setMessage(`❌ Failed to remove from blacklist: ${error.response?.data?.error || error.message}`);
      setTimeout(() => setMessage(''), 5000);
    }
    setRemovingFromBlacklist(null);
  };

  const clearBlacklist = async () => {
    if (!confirm('Are you sure you want to clear the entire blacklist? This cannot be undone.')) {
      return;
    }
    setLoadingBlacklist(true);
    try {
      await api.post('/blacklist/clear');
      setMessage('✅ Blacklist cleared successfully');
      setTimeout(() => setMessage(''), 3000);
      fetchBlacklist(); // Refresh the list
    } catch (error: any) {
      setMessage(`❌ Failed to clear blacklist: ${error.response?.data?.error || error.message}`);
      setTimeout(() => setMessage(''), 5000);
    }
    setLoadingBlacklist(false);
  };

  const executeDbAction = async (action: string) => {
    setDbOperation(action);
    setConfirmDialog(null);
    try {
      const response = await api.post(`/database/${action}`);
      setMessage(`✅ ${response.data.message}`);
      setTimeout(() => setMessage(''), 5000);
      fetchDbStats();
      // Also refresh services if we cleared something that affects them
      fetchServices();
    } catch (error: any) {
      setMessage(`❌ Failed: ${error.response?.data?.error || error.message}`);
      setTimeout(() => setMessage(''), 5000);
    }
    setDbOperation(null);
  };

  const fetchVersionInfo = async () => {
    try {
      const response = await api.get('/version');
      const data = response.data;
      setVersionInfo(data);
    } catch (error) {
      console.error('Failed to fetch version info:', error);
    }
  };

  const checkForUpdates = async () => {
    setCheckingUpdate(true);
    try {
      const response = await api.get('/version/check');
      const data = response.data;
      setVersionInfo(data);
      if (data.update_available) {
        setMessage('🎉 New update available!');
      } else {
        setMessage('✅ You are running the latest version');
      }
      setTimeout(() => setMessage(''), 5000);
    } catch (error: any) {
      setMessage(`❌ Failed to check for updates: ${error.response?.data?.error || error.message}`);
      setTimeout(() => setMessage(''), 5000);
    }
    setCheckingUpdate(false);
  };

  const installUpdate = async () => {
    if (!confirm('Are you sure you want to install the update? The server will restart.')) {
      return;
    }
    setInstallingUpdate(true);
    setMessage('🔄 Installing update... Server will restart shortly.');
    try {
      await api.post('/update/install');
      setMessage('✅ Update started! The page will reload in 30 seconds...');
      // Wait for server to restart and reload
      setTimeout(() => {
        window.location.reload();
      }, 30000);
    } catch (error: any) {
      setMessage(`❌ Update failed: ${error.response?.data?.error || error.message}`);
      setInstallingUpdate(false);
    }
  };

  // Account profile management functions
  const fetchUserProfile = async () => {
    try {
      const response = await api.get('/auth/profile');
      const data = response.data;
      setProfileUsername(data.username || '');
      setProfileEmail(data.email || '');
      if (data.profile_picture) {
        setProfileAvatar(data.profile_picture);
        localStorage.setItem('profile_picture', data.profile_picture);
      }
      localStorage.setItem('username', data.username || '');
    } catch (error) {
      console.error('Failed to fetch profile:', error);
      // Set defaults from localStorage if API fails
      const savedUsername = localStorage.getItem('username');
      if (savedUsername) {
        setProfileUsername(savedUsername);
      }
      const savedAvatar = localStorage.getItem('profile_picture');
      if (savedAvatar) {
        setProfileAvatar(savedAvatar);
      }
    }
  };

  const updateProfile = async () => {
    if (!profileUsername.trim()) {
      setProfileMessage('❌ Username cannot be empty');
      setTimeout(() => setProfileMessage(''), 3000);
      return;
    }

    try {
      await api.put('/auth/profile', {
        username: profileUsername,
        email: profileEmail,
        profile_picture: profileAvatar || ''
      });
      
      localStorage.setItem('username', profileUsername);
      if (profileAvatar) {
        localStorage.setItem('profile_picture', profileAvatar);
      } else {
        localStorage.removeItem('profile_picture');
      }
      window.dispatchEvent(new Event('storage'));
      setProfileMessage('✅ Profile updated successfully');
      setTimeout(() => setProfileMessage(''), 3000);
    } catch (error: any) {
      setProfileMessage(`❌ ${error.response?.data?.error || 'Failed to update profile'}`);
      setTimeout(() => setProfileMessage(''), 3000);
    }
  };

  const changePassword = async () => {
    if (!currentPassword || !newPassword || !confirmPassword) {
      setProfileMessage('❌ All password fields are required');
      setTimeout(() => setProfileMessage(''), 3000);
      return;
    }

    if (newPassword !== confirmPassword) {
      setProfileMessage('❌ New passwords do not match');
      setTimeout(() => setProfileMessage(''), 3000);
      return;
    }

    if (newPassword.length < 6) {
      setProfileMessage('❌ Password must be at least 6 characters');
      setTimeout(() => setProfileMessage(''), 3000);
      return;
    }

    try {
      await api.put('/auth/password', {
        current_password: currentPassword,
        new_password: newPassword
      });
      
      setProfileMessage('✅ Password changed successfully');
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setTimeout(() => setProfileMessage(''), 3000);
    } catch (error: any) {
      setProfileMessage(`❌ ${error.response?.data?.error || 'Failed to change password'}`);
      setTimeout(() => setProfileMessage(''), 3000);
    }
  };

  const showConfirmDialog = (action: string, title: string, message: string) => {
    setConfirmDialog({ action, title, message });
  };

  const toggleServiceEnabled = async (serviceName: string, enabled: boolean) => {
    try {
      await api.put(`/services/${serviceName}?name=${serviceName}`, { enabled });
      fetchServices();
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
      const response = await api.get('/channels/stats');
      const data = response.data;
      setChannelStats(data);
      // Note: Don't initialize enabled sets here - they come from settings
    } catch (error) {
      console.error('Failed to fetch channel stats:', error);
    }
  };

  const fetchSettings = async () => {
    try {
      const response = await api.get('/settings');
      const data = response.data;
      
      // Only set defaults if the backend explicitly returns empty/null/undefined
      // Don't override if backend returns actual values
      if (data.xtream_username === undefined || data.xtream_username === null || data.xtream_username === '') {
        data.xtream_username = 'streamarr';
      }
      if (data.xtream_password === undefined || data.xtream_password === null || data.xtream_password === '') {
        data.xtream_password = 'streamarr';
      }
      
      console.log('Xtream credentials from backend:', {
        username: data.xtream_username,
        password: data.xtream_password
      });
      
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
      
      // Load Xtream sources
      if (data.xtream_sources && Array.isArray(data.xtream_sources)) {
        setXtreamSources(data.xtream_sources);
      }
      
      // Load enabled sources and categories (default to empty = all disabled)
      if (data.livetv_enabled_sources && Array.isArray(data.livetv_enabled_sources)) {
        setEnabledSources(new Set(data.livetv_enabled_sources));
      } else {
        setEnabledSources(new Set()); // Default to none enabled
      }
      if (data.livetv_enabled_categories && Array.isArray(data.livetv_enabled_categories)) {
        setEnabledCategories(new Set(data.livetv_enabled_categories));
      } else {
        setEnabledCategories(new Set()); // Default to none enabled
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
        xtream_sources: xtreamSources,
        livetv_enabled_sources: Array.from(enabledSources),
        livetv_enabled_categories: Array.from(enabledCategories),
      };
      
      console.log('Saving settings:', settingsToSave);
      console.log('Xtream credentials being saved:', {
        xtream_username: settingsToSave.xtream_username,
        xtream_password: settingsToSave.xtream_password
      });
      console.log('Settings keys:', Object.keys(settingsToSave));
      
      let body: string;
      try {
        body = JSON.stringify(settingsToSave);
        console.log('Body size:', body.length, 'bytes');
      } catch (e) {
        console.error('JSON stringify error:', e);
        setMessage(`❌ Error preparing settings: ${e instanceof Error ? e.message : 'Unknown'}`);
        setSaving(false);
        return;
      }
      
      console.log('Sending to:', `${API_BASE_URL}/settings`);
      
      await api.put('/settings', settingsToSave);
      
      setMessage('✅ Settings saved successfully!');
      setTimeout(() => setMessage(''), 3000);
    } catch (error) {
      console.error('Failed to save settings:', error);
      setMessage(`❌ Error saving settings: ${error instanceof Error ? error.message : 'Unknown error'}`);
    } finally {
      setSaving(false);
    }
  };

  // Save a specific setting immediately (merge with current state and send full payload)
  const saveSettingsImmediate = async (patch: Partial<SettingsData>) => {
    if (!settings) return;
    const next: SettingsData = { ...settings, ...patch } as SettingsData;
    const settingsToSave = {
      ...next,
      mdblist_lists: JSON.stringify(mdbLists),
      m3u_sources: m3uSources,
      xtream_sources: xtreamSources,
      livetv_enabled_sources: Array.from(enabledSources),
      livetv_enabled_categories: Array.from(enabledCategories),
    };
    try {
      await api.put('/settings', settingsToSave);
      setSettings(next);
      setMessage('✅ Setting saved');
      setTimeout(() => setMessage(''), 2000);
    } catch (error: any) {
      setMessage(`❌ Error saving: ${error.response?.data?.error || error.message}`);
      setTimeout(() => setMessage(''), 3000);
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
      const response = await api.get(`/mdblist/user-lists?apiKey=${encodeURIComponent(settings.mdblist_api_key)}`);
      const data = response.data;
      
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
  const previewM3UCategories = async () => {
    if (!newM3uUrl.trim()) {
      setMessage('❌ Please enter an M3U URL first');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    // Validate M3U URL
    if (!newM3uUrl.startsWith('http://') && !newM3uUrl.startsWith('https://')) {
      setMessage('❌ M3U URL must start with http:// or https://');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    setLoadingCategories(true);
    try {
      const res = await api.post('iptv-vod/preview-categories', {
        url: newM3uUrl.trim(),
        import_mode: settings?.iptv_import_mode || 'both'
      });
      
      setAvailableCategories(res.data.categories || []);
      setNewM3uCategories([]);
      setShowCategoryModal(true);
      setMessage('');
    } catch (err: any) {
      console.error('Failed to preview categories:', err);
      setMessage(`❌ Failed to preview categories: ${err.response?.data?.error || err.message}`);
      setTimeout(() => setMessage(''), 5000);
    } finally {
      setLoadingCategories(false);
    }
  };

  const toggleCategory = (categoryName: string) => {
    setNewM3uCategories(prev => {
      if (prev.includes(categoryName)) {
        return prev.filter(c => c !== categoryName);
      } else {
        return [...prev, categoryName];
      }
    });
  };

  const selectAllCategories = () => {
    setNewM3uCategories(availableCategories.map(c => c.name));
  };

  const deselectAllCategories = () => {
    setNewM3uCategories([]);
  };

  const addM3uSource = () => {
    if (!newM3uName.trim() || !newM3uUrl.trim()) {
      setMessage('❌ Please enter both a name and URL for the M3U source');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    // Validate M3U URL
    if (!newM3uUrl.startsWith('http://') && !newM3uUrl.startsWith('https://')) {
      setMessage('❌ M3U URL must start with http:// or https://');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    // Validate EPG URL if provided
    if (newM3uEpg.trim() && !newM3uEpg.startsWith('http://') && !newM3uEpg.startsWith('https://')) {
      setMessage('❌ EPG URL must start with http:// or https://');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    // Check for duplicates
    if (m3uSources.some(s => s.url === newM3uUrl || s.name === newM3uName.trim())) {
      setMessage('⚠️ A source with this name or URL already exists');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    const newSource: M3USource = {
      name: newM3uName.trim(),
      url: newM3uUrl.trim(),
      enabled: true
    };
    
    if (newM3uEpg.trim()) {
      newSource.epg_url = newM3uEpg.trim();
    }
    
    // Add selected categories if any
    if (newM3uCategories.length > 0) {
      newSource.selected_categories = newM3uCategories;
    }
    
    setM3uSources([...m3uSources, newSource]);
    setNewM3uName('');
    setNewM3uUrl('');
    setNewM3uEpg('');
    setNewM3uCategories([]);
    setAvailableCategories([]);
    setShowCategoryModal(false);
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

  // Xtream category preview function
  const previewXtreamCategories = async () => {
    if (!newXtreamUrl.trim() || !newXtreamUsername.trim() || !newXtreamPassword.trim()) {
      setMessage('❌ Please enter server URL, username, and password first');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    setLoadingXtreamCategories(true);
    try {
      const response = await api.post('iptv-vod/preview-xtream-categories', {
        server_url: newXtreamUrl.trim().replace(/\/$/, ''),
        username: newXtreamUsername.trim(),
        password: newXtreamPassword.trim(),
        import_mode: settings?.iptv_import_mode || 'both'
      });
      
      if (response.data.categories && response.data.categories.length > 0) {
        setAvailableXtreamCategories(response.data.categories);
        setShowXtreamCategoryModal(true);
      } else {
        setMessage('⚠️ No categories found');
        setTimeout(() => setMessage(''), 3000);
      }
    } catch (error: any) {
      console.error('Failed to preview categories:', error);
      setMessage(`❌ Failed to fetch categories: ${error.response?.data?.error || error.message}`);
      setTimeout(() => setMessage(''), 5000);
    } finally {
      setLoadingXtreamCategories(false);
    }
  };

  const toggleXtreamCategory = (categoryName: string) => {
    setNewXtreamCategories(prev => {
      if (prev.includes(categoryName)) {
        return prev.filter(c => c !== categoryName);
      } else {
        return [...prev, categoryName];
      }
    });
  };

  const selectAllXtreamCategories = () => {
    setNewXtreamCategories(availableXtreamCategories.map(c => c.name));
  };

  const deselectAllXtreamCategories = () => {
    setNewXtreamCategories([]);
  };

  // Balkan VOD category functions
  const previewBalkanCategories = async () => {
    setLoadingBalkanCategories(true);
    try {
      const response = await api.post('/balkan-vod/preview-categories');
      if (response.data.categories) {
        setAvailableBalkanCategories(response.data.categories);
        setNewBalkanCategories(settings?.balkan_vod_selected_categories || []);
        setShowBalkanCategoryModal(true);
      } else {
        setMessage('⚠️ No categories found');
        setTimeout(() => setMessage(''), 3000);
      }
    } catch (error: any) {
      console.error('Balkan categories error:', error);
      setMessage(`❌ Failed to fetch categories: ${error.response?.data?.error || error.message}`);
      setTimeout(() => setMessage(''), 5000);
    } finally {
      setLoadingBalkanCategories(false);
    }
  };

  const toggleBalkanCategory = (categoryName: string) => {
    setNewBalkanCategories(prev => {
      if (prev.includes(categoryName)) {
        return prev.filter(c => c !== categoryName);
      } else {
        return [...prev, categoryName];
      }
    });
  };

  const selectAllBalkanCategories = () => {
    setNewBalkanCategories(availableBalkanCategories.map(c => c.name));
  };

  const deselectAllBalkanCategories = () => {
    setNewBalkanCategories([]);
  };

  const saveBalkanCategories = () => {
    updateSetting('balkan_vod_selected_categories', newBalkanCategories);
    setShowBalkanCategoryModal(false);
    setMessage('✅ Balkan VOD categories updated');
    setTimeout(() => setMessage(''), 3000);
  };

  // Xtream Source management functions
  const addXtreamSource = () => {
    if (!newXtreamName.trim() || !newXtreamUrl.trim() || !newXtreamUsername.trim() || !newXtreamPassword.trim()) {
      setMessage('❌ Please fill in all fields for the Xtream source');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    // Validate URL
    if (!newXtreamUrl.startsWith('http://') && !newXtreamUrl.startsWith('https://')) {
      setMessage('❌ Server URL must start with http:// or https://');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    // Check for duplicates
    if (xtreamSources.some(s => s.server_url === newXtreamUrl || s.name === newXtreamName.trim())) {
      setMessage('⚠️ An Xtream source with this name or URL already exists');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    const newSource: XtreamSource = {
      name: newXtreamName.trim(),
      server_url: newXtreamUrl.trim().replace(/\/$/, ''), // Remove trailing slash
      username: newXtreamUsername.trim(),
      password: newXtreamPassword.trim(),
      enabled: true,
      selected_categories: newXtreamCategories.length > 0 ? newXtreamCategories : undefined
    };
    
    setXtreamSources([...xtreamSources, newSource]);
    setNewXtreamName('');
    setNewXtreamUrl('');
    setNewXtreamUsername('');
    setNewXtreamPassword('');
    setNewXtreamCategories([]);
    setAvailableXtreamCategories([]);
    setMessage('✅ Xtream source added. Click Save to apply.');
    setTimeout(() => setMessage(''), 3000);
  };

  const removeXtreamSource = (index: number) => {
    setXtreamSources(xtreamSources.filter((_, i) => i !== index));
  };

  const toggleXtreamSource = (index: number) => {
    setXtreamSources(xtreamSources.map((source, i) => 
      i === index ? { ...source, enabled: !source.enabled } : source
    ));
  };

  // Check single M3U source status
  const checkSourceStatus = async (url: string) => {
    // Mark as checking
    setSourceStatuses(prev => {
      const newMap = new Map(prev);
      newMap.set(url, { url, online: false, status_code: 0, message: 'Checking...', checking: true });
      return newMap;
    });
    
    try {
      const response = await api.post('/channels/check-source', { url });
      const data = response.data;
      setSourceStatuses(prev => {
        const newMap = new Map(prev);
        newMap.set(url, { ...data, checking: false });
        return newMap;
      });
    } catch (error) {
      setSourceStatuses(prev => {
        const newMap = new Map(prev);
        newMap.set(url, { url, online: false, status_code: 0, message: 'Check failed', checking: false });
        return newMap;
      });
    }
  };

  // Check all M3U sources status
  const checkAllSourcesStatus = async () => {
    setCheckingAllSources(true);
    for (const source of m3uSources) {
      await checkSourceStatus(source.url);
    }
    setCheckingAllSources(false);
  };

  // Get status indicator for a source URL
  const getSourceStatusIndicator = (url: string) => {
    const status = sourceStatuses.get(url);
    if (!status) return null;
    if (status.checking) {
      return <div className="w-3 h-3 bg-yellow-500 rounded-full animate-pulse" title="Checking..." />;
    }
    if (status.online) {
      return <div className="w-3 h-3 bg-green-500 rounded-full" title={`Online (${status.status_code})`} />;
    }
    return <div className="w-3 h-3 bg-red-500 rounded-full" title={status.message || 'Offline'} />;
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
    { id: 'account' as TabType, label: 'Account', icon: User },
    { id: 'api' as TabType, label: 'API Keys', icon: Key },
    { id: 'addons' as TabType, label: 'Addons', icon: Layers },
    { id: 'quality' as TabType, label: 'Quality', icon: SettingsIcon },
    { id: 'content' as TabType, label: 'Content', icon: Film },
    { id: 'livetv' as TabType, label: 'Live TV', icon: Tv },
    { id: 'stremio' as TabType, label: 'Stremio', icon: Play },
    { id: 'filters' as TabType, label: 'Filters', icon: Filter },
    { id: 'services' as TabType, label: 'Services', icon: Activity },
    { id: 'xtream' as TabType, label: 'Xtream', icon: Server },
    { id: 'notifications' as TabType, label: 'Notifications', icon: Bell },
    { id: 'database' as TabType, label: 'Database', icon: Database },
    { id: 'blacklist' as TabType, label: 'Blacklist', icon: Trash2 },
    { id: 'about' as TabType, label: 'About', icon: Info },
  ];

  return (
    <div className="min-h-screen bg-[#141414] -m-6 p-8">
      <div className="max-w-6xl mx-auto">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-4xl font-black text-white">Settings</h1>
            <p className="text-slate-400 mt-1">Manage your StreamArr configuration</p>
          </div>
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-2 px-6 py-3 bg-red-600 text-white rounded-lg font-bold hover:bg-red-700 disabled:bg-slate-600 disabled:cursor-not-allowed transition-colors"
          >
            <Save className="w-4 h-4" />
            {saving ? 'Saving...' : 'Save Changes'}
          </button>
        </div>

        {message && (
          <div className="mb-6 p-4 bg-[#1e1e1e] border border-white/10 rounded-lg text-white">
            {message}
          </div>
        )}

        {/* Tabs */}
        <div className="mb-8 border-b border-white/10 overflow-x-auto">
          <div className="flex gap-1">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 px-5 py-3 border-b-2 transition-all whitespace-nowrap font-medium ${
                  activeTab === tab.id
                    ? 'border-red-600 text-white bg-white/5'
                    : 'border-transparent text-slate-400 hover:text-white hover:bg-white/5'
                }`}
              >
                <tab.icon className="w-4 h-4" />
                {tab.label}
              </button>
            ))}
          </div>
        </div>

        {/* Account Tab */}
        {activeTab === 'account' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-blue-900/30 border border-blue-800 rounded-lg">
                <h3 className="text-red-400 font-medium mb-2">👤 User Profile</h3>
                <p className="text-sm text-slate-400">
                  Manage your account settings, change your username or password, and personalize your profile with an avatar.
                </p>
              </div>

              {profileMessage && (
                <div className="p-4 bg-[#2a2a2a] border border-white/10 rounded-lg text-white">
                  {profileMessage}
                </div>
              )}

              {/* Avatar Section */}
              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">Profile Picture</h3>
                <div className="flex items-center gap-6">
                  <div className="relative">
                    {profileAvatar ? (
                      <img 
                        src={profileAvatar} 
                        alt="Profile" 
                        className="w-24 h-24 rounded-full object-cover"
                      />
                    ) : (
                      <div className="w-24 h-24 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-white text-3xl font-bold">
                        {profileUsername?.charAt(0).toUpperCase() || 'U'}
                      </div>
                    )}
                    <input
                      type="file"
                      id="avatar-upload"
                      accept="image/jpeg,image/png,image/gif"
                      className="hidden"
                      onChange={async (e) => {
                        const file = e.target.files?.[0];
                        if (file) {
                          if (file.size > 2 * 1024 * 1024) {
                            setProfileMessage('Image size must be less than 2MB');
                            return;
                          }
                          const reader = new FileReader();
                          reader.onload = async (event) => {
                            if (event.target?.result) {
                              const avatarData = event.target.result as string;
                              setProfileAvatar(avatarData);
                              
                              // Save to server
                              try {
                                await api.put('/auth/profile', {
                                  profile_picture: avatarData
                                });
                                localStorage.setItem('profile_picture', avatarData);
                                setProfileMessage('✅ Profile picture updated!');
                                setTimeout(() => setProfileMessage(''), 3000);
                              } catch (error) {
                                setProfileMessage('❌ Failed to save profile picture to server');
                                setTimeout(() => setProfileMessage(''), 3000);
                              }
                            }
                          };
                          reader.readAsDataURL(file);
                        }
                      }}
                    />
                    <button 
                      onClick={() => document.getElementById('avatar-upload')?.click()}
                      className="absolute bottom-0 right-0 p-2 bg-red-600 rounded-full hover:bg-red-700 transition-colors"
                    >
                      <Camera className="w-4 h-4 text-white" />
                    </button>
                  </div>
                  <div>
                    <p className="text-sm text-slate-300 mb-2">Upload a profile picture</p>
                    <p className="text-xs text-slate-500">JPG, PNG or GIF. Max 2MB.</p>
                    {profileAvatar && (
                      <button
                        onClick={async () => {
                          try {
                            await api.put('/auth/profile', {
                              profile_picture: ''
                            });
                            setProfileAvatar(null);
                            localStorage.removeItem('profile_picture');
                            setProfileMessage('✅ Profile picture removed');
                            setTimeout(() => setProfileMessage(''), 2000);
                          } catch (error) {
                            setProfileMessage('❌ Failed to remove profile picture');
                            setTimeout(() => setProfileMessage(''), 3000);
                          }
                        }}
                        className="mt-2 text-xs text-red-400 hover:text-red-300"
                      >
                        Remove picture
                      </button>
                    )}
                  </div>
                </div>
              </div>

              {/* Profile Information Section */}
              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">Profile Information</h3>
                <div className="max-w-md space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Username
                    </label>
                    <input
                      type="text"
                      value={profileUsername}
                      onChange={(e) => setProfileUsername(e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="Enter username"
                    />
                    <p className="text-xs text-slate-500 mt-1">Your display name across the app</p>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Email
                    </label>
                    <input
                      type="email"
                      value={profileEmail}
                      onChange={(e) => setProfileEmail(e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="Enter email"
                    />
                    <p className="text-xs text-slate-500 mt-1">Used for account recovery</p>
                  </div>
                  <button
                    onClick={updateProfile}
                    className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
                  >
                    <Save className="w-4 h-4" />
                    Update Profile
                  </button>
                </div>
              </div>

              {/* Change Password Section */}
              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">Change Password</h3>
                <div className="max-w-md space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Current Password
                    </label>
                    <input
                      type="password"
                      value={currentPassword}
                      onChange={(e) => setCurrentPassword(e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="Enter current password"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      New Password
                    </label>
                    <input
                      type="password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="Enter new password (min 6 characters)"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Confirm New Password
                    </label>
                    <input
                      type="password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="Confirm new password"
                    />
                  </div>
                  <button
                    onClick={changePassword}
                    className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
                  >
                    <Save className="w-4 h-4" />
                    Update Password
                  </button>
                </div>
              </div>

              {/* Account Info Section */}
              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">Account Information</h3>
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-slate-400">Role:</span>
                    <span className="text-slate-300">{localStorage.getItem('is_admin') === 'true' ? 'Administrator' : 'User'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-400">Account Status:</span>
                    <span className="text-green-400">Active</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* API Keys Tab */}
        {activeTab === 'api' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  TMDB API Key <span className="text-red-400">*</span>
                </label>
                <input
                  type="text"
                  value={settings.tmdb_api_key || ''}
                  onChange={(e) => updateSetting('tmdb_api_key', e.target.value)}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="Your TMDB API key"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Required. Used to fetch movie/series metadata, posters, and discover content.{' '}
                  <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noopener noreferrer" className="text-red-400 hover:underline">
                    Get one free
                  </a>
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Real-Debrid API Key
                </label>
                <input
                  type="text"
                  value={settings.realdebrid_api_key || ''}
                  onChange={(e) => updateSetting('realdebrid_api_key', e.target.value)}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="Your Real-Debrid API key"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Debrid service for fast, cached torrent streams. Enables higher quality sources.{' '}
                  <a href="https://real-debrid.com/apitoken" target="_blank" rel="noopener noreferrer" className="text-red-400 hover:underline">
                    Get API token
                  </a>
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Premiumize API Key
                </label>
                <input
                  type="text"
                  value={settings.premiumize_api_key || ''}
                  onChange={(e) => updateSetting('premiumize_api_key', e.target.value)}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="Your Premiumize API key"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Alternative debrid service. Use either Real-Debrid OR Premiumize (or both).
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  MDBList API Key
                </label>
                <input
                  type="text"
                  value={settings.mdblist_api_key || ''}
                  onChange={(e) => updateSetting('mdblist_api_key', e.target.value)}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="Your MDBList API key (optional for public lists)"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Only required for private lists. Get yours from{' '}
                  <a href="https://mdblist.com/preferences/" target="_blank" rel="noopener noreferrer" className="text-red-400 hover:underline">
                    mdblist.com/preferences
                  </a>
                </p>
              </div>

              {/* MDBList Auto-Import */}
              <div className="mt-6 pt-6 border-t border-white/10">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-lg font-medium text-slate-300">MDBList Auto-Import Lists</h3>
                  <button
                    onClick={fetchUserMDBLists}
                    disabled={fetchingUserLists || !settings.mdblist_api_key}
                    className="px-3 py-1.5 text-sm bg-gray-700 text-white rounded-lg hover:bg-gray-600 disabled:bg-[#2a2a2a] disabled:text-slate-500 disabled:cursor-not-allowed transition-colors"
                  >
                    {fetchingUserLists ? 'Loading...' : '📋 Fetch My Lists'}
                  </button>
                </div>
                <p className="text-sm text-slate-400 mb-4">
                  Automatically add movies/series from MDBList curated lists to your library (Movies/Series pages). 
                  The worker periodically syncs these lists. This is separate from playlist generation.
                </p>

                {/* User's MDBLists from API */}
                {userLists.length > 0 && (
                  <div className="mb-4">
                    <p className="text-sm text-slate-300 mb-3">📋 Your MDBLists (click to add to library sync):</p>
                    <div className="grid grid-cols-2 md:grid-cols-3 gap-2 max-h-64 overflow-y-auto">
                      {userLists.map((list) => {
                        const isAdded = mdbLists.some(l => l.url.includes(list.slug));
                        return (
                          <div
                            key={list.id}
                            onClick={() => !isAdded && addUserList(list)}
                            className={`flex items-center gap-2 p-3 rounded-lg border cursor-pointer transition-colors ${
                              isAdded
                                ? 'bg-green-900/30 border-green-700 cursor-default'
                                : 'bg-[#2a2a2a]/50 border-white/10 hover:bg-blue-900/30 hover:border-blue-700'
                            }`}
                          >
                            <input
                              type="checkbox"
                              checked={isAdded}
                              onChange={() => {}}
                              className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded pointer-events-none"
                            />
                            <div className="flex-1 min-w-0">
                              <div className="text-sm font-medium text-white truncate">{list.name}</div>
                              <div className="text-xs text-slate-500">{list.items} items</div>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}
                
                <div className="flex gap-2 mb-3">
                  <input
                    type="text"
                    value={newListUrl}
                    onChange={(e) => setNewListUrl(e.target.value)}
                    onKeyPress={(e) => e.key === 'Enter' && addMDBList()}
                    className="flex-1 px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    placeholder="https://mdblist.com/lists/username/list-name"
                  />
                  <button
                    onClick={addMDBList}
                    className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                  </button>
                </div>

                {mdbLists.length > 0 && (
                  <div className="mb-4">
                    <p className="text-sm text-slate-300 mb-3">✅ Added Lists (synced to library):</p>
                    <div className="grid grid-cols-2 md:grid-cols-3 gap-2 max-h-64 overflow-y-auto">
                      {mdbLists.map((list, index) => (
                        <div
                          key={index}
                          className={`flex items-center gap-2 p-3 rounded-lg border transition-colors ${
                            list.enabled
                              ? 'bg-green-900/30 border-green-700'
                              : 'bg-[#2a2a2a]/50 border-white/10 opacity-60'
                          }`}
                        >
                          <input
                            type="checkbox"
                            checked={list.enabled}
                            onChange={() => toggleMDBList(index)}
                            className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded cursor-pointer"
                          />
                          <div className="flex-1 min-w-0 cursor-pointer" onClick={() => toggleMDBList(index)}>
                            <div className="text-sm font-medium text-white truncate">{list.name}</div>
                            <div className="text-xs text-slate-500 truncate">{list.url.split('/').pop()}</div>
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
                  </div>
                )}

                {mdbLists.length === 0 && (
                  <div className="text-center py-6 text-slate-500 bg-[#2a2a2a] rounded-lg">
                    <p className="text-sm">No lists configured yet</p>
                    <p className="text-xs mt-1">Add popular lists like "Top Watched Movies of the Week"</p>
                  </div>
                )}

                <div className="mt-3 text-xs text-slate-500">
                  <p className="mb-1">💡 Popular lists:</p>
                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={() => setNewListUrl('https://mdblist.com/lists/linaspuransen/top-watched-movies-of-the-week')}
                      className="text-red-400 hover:underline"
                    >
                      Top Watched Movies
                    </button>
                    <span>•</span>
                    <button
                      onClick={() => setNewListUrl('https://mdblist.com/lists/linaspuransen/top-watched-tv-shows-of-the-week')}
                      className="text-red-400 hover:underline"
                    >
                      Top Watched TV Shows
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Addons Tab */}
        {activeTab === 'addons' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              {/* Debrid Services */}
              <div>
                <h3 className="text-lg font-medium text-white mb-2">💎 Debrid Services</h3>
                <p className="text-sm text-slate-400 mb-4">Premium services that cache torrents for instant high-speed streaming</p>
                <div className="grid grid-cols-3 gap-3">
                  <div
                    className={`flex items-center gap-3 p-4 rounded-lg border cursor-pointer transition-colors ${
                      settings.use_realdebrid
                        ? 'bg-green-900/30 border-green-700 hover:bg-green-900/50'
                        : 'bg-[#2a2a2a]/50 border-white/10 opacity-60 hover:opacity-80'
                    }`}
                    onClick={() => updateSetting('use_realdebrid', !settings.use_realdebrid)}
                  >
                    <input
                      type="checkbox"
                      checked={settings.use_realdebrid || false}
                      onChange={() => {}}
                      className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded pointer-events-none"
                    />
                    <div className="flex-1">
                      <div className="text-sm font-medium text-white">Real-Debrid</div>
                      <div className="text-xs text-slate-500">Most popular debrid service</div>
                    </div>
                  </div>
                  <div
                    className={`flex items-center gap-3 p-4 rounded-lg border cursor-pointer transition-colors ${
                      settings.use_premiumize
                        ? 'bg-green-900/30 border-green-700 hover:bg-green-900/50'
                        : 'bg-[#2a2a2a]/50 border-white/10 opacity-60 hover:opacity-80'
                    }`}
                    onClick={() => updateSetting('use_premiumize', !settings.use_premiumize)}
                  >
                    <input
                      type="checkbox"
                      checked={settings.use_premiumize || false}
                      onChange={() => {}}
                      className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded pointer-events-none"
                    />
                    <div className="flex-1">
                      <div className="text-sm font-medium text-white">Premiumize</div>
                      <div className="text-xs text-slate-500">Premium multi-host service</div>
                    </div>
                  </div>
                  <div
                    className="flex items-center gap-3 p-4 rounded-lg border border-white/10 bg-[#2a2a2a]/30 opacity-50 cursor-not-allowed"
                    title="Coming soon"
                  >
                    <input
                      type="checkbox"
                      checked={false}
                      disabled
                      onChange={() => {}}
                      className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded pointer-events-none"
                    />
                    <div className="flex-1">
                      <div className="text-sm font-medium text-white">TorBox</div>
                      <div className="text-xs text-slate-500">Coming soon</div>
                    </div>
                  </div>
                </div>
              </div>

              <hr className="border-white/10" />

              {/* Comet Provider Configuration */}
              <div className="mb-6">
                <h3 className="text-lg font-semibold text-white mb-2">🌟 Comet Torrent Provider</h3>
                <p className="text-sm text-slate-400 mb-4">
                  Configure Comet for real-time torrent searches. Comet searches multiple indexers and checks RealDebrid cache availability.
                </p>

                <label className="flex items-center gap-2 mb-4 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={settings.comet_enabled !== false} // Default to true
                    onChange={(e) => updateSetting('comet_enabled', e.target.checked)}
                    className="w-4 h-4 rounded border-white/20 bg-[#2a2a2a] text-blue-600 focus:ring-blue-500"
                  />
                  <span className="text-sm text-slate-300">Enable Comet Provider</span>
                </label>

                <label className="flex items-center gap-2 mb-4 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={settings.comet_only_show_cached || false}
                    onChange={(e) => updateSetting('comet_only_show_cached', e.target.checked)}
                    className="w-4 h-4 rounded border-white/20 bg-[#2a2a2a] text-blue-600 focus:ring-blue-500"
                    disabled={settings.comet_enabled === false}
                  />
                  <span className="text-sm text-slate-300">Only Show Cached Torrents</span>
                </label>
                <p className="text-xs text-slate-500 mb-4">
                  When enabled, only torrents that are already cached in RealDebrid will be shown, ensuring instant playback.
                </p>

                <div className="mb-4">
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Indexers (comma-separated)
                  </label>
                  <input
                    type="text"
                    value={settings.comet_indexers || 'bitorrent,therarbg,yts,eztv,thepiratebay'}
                    onChange={(e) => updateSetting('comet_indexers', e.target.value)}
                    className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    placeholder="bitorrent,therarbg,yts,eztv,thepiratebay"
                    disabled={settings.comet_enabled === false}
                  />
                  <p className="text-xs text-slate-500 mt-1">
                    Available indexers: bitorrent, therarbg, yts, eztv, thepiratebay, kickass, torrentgalaxy, magnetdl
                  </p>
                </div>

                <div className="mb-4">
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Sorting
                  </label>
                  <select
                    value={settings.comet_sort_by || 'quality'}
                    onChange={(e) => updateSetting('comet_sort_by', e.target.value)}
                    className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    disabled={settings.comet_enabled === false}
                  >
                    <option value="quality">By quality then seeders</option>
                    <option value="qualitysize">By quality then size</option>
                    <option value="seeders">By seeders</option>
                    <option value="size">By size</option>
                  </select>
                </div>

                <div className="mb-4">
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Priority Languages (comma-separated)
                  </label>
                  <input
                    type="text"
                    value={settings.comet_priority_languages || ''}
                    onChange={(e) => updateSetting('comet_priority_languages', e.target.value)}
                    className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    placeholder="e.g., russian,italian,spanish"
                    disabled={settings.comet_enabled === false}
                  />
                  <p className="text-xs text-slate-500 mt-1">
                    Streams with selected language dubs/subs will be prioritized at the top
                  </p>
                </div>

                <div className="mb-4">
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Exclude Qualities/Resolutions (comma-separated)
                  </label>
                  <input
                    type="text"
                    value={settings.comet_excluded_qualities || ''}
                    onChange={(e) => updateSetting('comet_excluded_qualities', e.target.value)}
                    className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    placeholder="e.g., cam,screener,480p"
                    disabled={settings.comet_enabled === false}
                  />
                  <p className="text-xs text-slate-500 mt-1">
                    Options: 4k, 1080p, 720p, 480p, cam, screener, brremux, hdr, dolbyvision, 3d
                  </p>
                </div>

                <div className="mb-4">
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Max Results Per Quality
                  </label>
                  <input
                    type="number"
                    value={settings.comet_max_results || 5}
                    onChange={(e) => updateSetting('comet_max_results', parseInt(e.target.value) || 5)}
                    className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    placeholder="5"
                    min="1"
                    max="50"
                    disabled={settings.comet_enabled === false}
                  />
                  <p className="text-xs text-slate-500 mt-1">
                    Limit the number of results shown per quality tier (1-50)
                  </p>
                </div>

                <div className="mb-4">
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Max Video Size
                  </label>
                  <input
                    type="text"
                    value={settings.comet_max_size || ''}
                    onChange={(e) => updateSetting('comet_max_size', e.target.value)}
                    className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    placeholder="e.g., 10GB or 10GB,2GB for movies,series"
                    disabled={settings.comet_enabled === false}
                  />
                  <p className="text-xs text-slate-500 mt-1">
                    Set maximum file size (e.g., 5GB, 800MB). Use comma for different limits: movies,series
                  </p>
                </div>
              </div>

              <hr className="border-white/10" />

              {/* Stremio Addons */}
              <div>
                <div className="flex items-center justify-between mb-4">
                  <div>
                    <h3 className="text-lg font-medium text-white mb-1">🎬 Stremio Addons</h3>
                    <p className="text-sm text-slate-400">Manage Stremio addons to fetch streams. You can add any standard Stremio addon.</p>
                  </div>
                  <button
                    onClick={() => {
                      const newAddon = { name: 'New Addon', url: 'https://addon.example.com', enabled: false };
                      const currentAddons = settings.stremio_addons || [];
                      updateSetting('stremio_addons', [...currentAddons, newAddon]);
                    }}
                    className="flex items-center gap-2 px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                    Add Addon
                  </button>
                </div>

                {/* Help Information */}
                <div className="bg-blue-900/20 border border-blue-500/30 rounded-lg p-4 mb-4">
                  <h4 className="text-sm font-medium text-blue-400 mb-2">ℹ️ How to Add Addons</h4>
                  <ul className="text-xs text-slate-400 space-y-1 list-disc list-inside">
                    <li>Use the <strong>full manifest URL</strong> ending with <code className="bg-slate-700/50 px-1 rounded">/manifest.json</code></li>
                    <li>Configure your addon on its website first (add Real-Debrid token, filters, etc.)</li>
                    <li>Copy the <strong>configured URL</strong> - it contains your settings encoded in it</li>
                    <li>Remove <code className="bg-slate-700/50 px-1 rounded">stremio://</code> prefix if present, use <code className="bg-slate-700/50 px-1 rounded">https://</code></li>
                    <li>Addons are tried <strong>in order</strong> - drag to reorder priority (first addon is tried first)</li>
                    <li><strong>Restart server</strong> after adding/changing addons for changes to take effect</li>
                  </ul>
                  <div className="mt-3 text-xs text-slate-500">
                    <span className="font-medium">Popular addons:</span> Torrentio, Comet, MediaFusion, Autostream, Sootio, Stremthru
                  </div>
                </div>
                
                <div className="space-y-3">
                  {(settings.stremio_addons || []).map((addon, index) => (
                    <div key={index} className="bg-[#2a2a2a]/50 border border-white/10 rounded-lg p-4">
                      <div className="flex items-start gap-3">
                        {/* Enable/Disable Toggle */}
                        <div className="flex items-center pt-2">
                          <input
                            type="checkbox"
                            checked={addon.enabled}
                            onChange={(e) => {
                              const newAddons = [...(settings.stremio_addons || [])];
                              newAddons[index] = { ...addon, enabled: e.target.checked };
                              updateSetting('stremio_addons', newAddons);
                            }}
                            className="w-5 h-5 bg-gray-700 border-gray-600 rounded"
                          />
                        </div>
                        
                        {/* Addon Fields */}
                        <div className="flex-1 space-y-3">
                          <div className="grid grid-cols-2 gap-3">
                            <div>
                              <label className="block text-sm font-medium text-slate-300 mb-1">Name</label>
                              <input
                                type="text"
                                value={addon.name}
                                onChange={(e) => {
                                  const newAddons = [...(settings.stremio_addons || [])];
                                  newAddons[index] = { ...addon, name: e.target.value };
                                  updateSetting('stremio_addons', newAddons);
                                }}
                                className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white"
                                placeholder="e.g., Torrentio"
                              />
                            </div>
                            <div>
                              <label className="block text-sm font-medium text-slate-300 mb-1">Base URL</label>
                              <input
                                type="text"
                                value={addon.url}
                                onChange={(e) => {
                                  const newAddons = [...(settings.stremio_addons || [])];
                                  newAddons[index] = { ...addon, url: e.target.value };
                                  updateSetting('stremio_addons', newAddons);
                                }}
                                className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white"
                                placeholder="https://addon.example.com"
                              />
                            </div>
                          </div>
                          <div className="flex items-center gap-2 text-xs text-slate-400">
                            <Info className="w-3 h-3" />
                            <span>Status: {addon.enabled ? <span className="text-green-400">Enabled</span> : <span className="text-slate-500">Disabled</span>}</span>
                          </div>
                        </div>
                        
                        {/* Delete Button */}
                        <button
                          onClick={() => {
                            const newAddons = (settings.stremio_addons || []).filter((_, i) => i !== index);
                            updateSetting('stremio_addons', newAddons);
                          }}
                          className="p-2 text-red-400 hover:bg-red-900/20 rounded-lg transition-colors"
                          title="Remove addon"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </div>
                  ))}
                  
                  {(!settings.stremio_addons || settings.stremio_addons.length === 0) && (
                    <div className="text-center py-8 text-slate-400">
                      <Layers className="w-12 h-12 mx-auto mb-2 opacity-50" />
                      <p>No addons configured. Click "Add Addon" to get started.</p>
                      <p className="text-xs mt-2">Popular addons: Torrentio, Comet, MediaFusion</p>
                    </div>
                  )}
                </div>
              </div>

              <hr className="border-white/10" />

              {/* Legacy Torrent Indexers - Removed with new addon system
              <div>
                <h3 className="text-lg font-medium text-white mb-2">🔍 Torrent Indexers</h3>
                <p className="text-sm text-slate-400 mb-4">Indexer configuration is now handled per-addon.</p>
              </div>
              */}
            </div>
          </div>
        )}

        {/* Quality Tab */}
        {activeTab === 'quality' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Maximum Resolution
                </label>
                <select
                  value={settings.max_resolution || 1080}
                  onChange={(e) => updateSetting('max_resolution', Number(e.target.value))}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                >
                  <option value="720">720p (HD)</option>
                  <option value="1080">1080p (Full HD)</option>
                  <option value="2160">2160p (4K)</option>
                </select>
                <p className="text-xs text-slate-500 mt-1">Highest video quality to include in streams. Lower = smaller files, faster loading.</p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Max File Size (MB)
                </label>
                <input
                  type="number"
                  value={settings.max_file_size || 0}
                  onChange={(e) => updateSetting('max_file_size', Number(e.target.value))}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                  placeholder="0 (unlimited)"
                />
                <p className="text-xs text-slate-500 mt-1">Skip files larger than this. 0 = no limit. Useful for slow connections.</p>
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="quality_variants"
                    checked={settings.enable_quality_variants || false}
                    onChange={(e) => updateSetting('enable_quality_variants', e.target.checked)}
                    className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                  />
                  <label htmlFor="quality_variants" className="text-sm text-slate-300">
                    Enable Quality Variants
                  </label>
                </div>
                <p className="text-xs text-slate-500 mt-1 ml-6">Show multiple quality options (720p, 1080p, 4K) for each stream.</p>
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="full_stream_name"
                    checked={settings.show_full_stream_name || false}
                    onChange={(e) => updateSetting('show_full_stream_name', e.target.checked)}
                    className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                  />
                  <label htmlFor="full_stream_name" className="text-sm text-slate-300">
                    Show Full Stream Names
                  </label>
                </div>
                <p className="text-xs text-slate-500 mt-1 ml-6">Display detailed stream info (codec, size, etc.) in player.</p>
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="auto_add_collections"
                    checked={settings.auto_add_collections || false}
                    onChange={(e) => updateSetting('auto_add_collections', e.target.checked)}
                    className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                  />
                  <label htmlFor="auto_add_collections" className="text-sm text-slate-300">
                    Add Entire Collection
                  </label>
                </div>
                <p className="text-xs text-slate-500 mt-1 ml-6">When adding a movie that belongs to a collection (e.g., The Dark Knight Trilogy), automatically add all other movies from that collection.</p>
              </div>

              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">Content Filters</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Minimum Year
                    </label>
                    <input
                      type="number"
                      value={settings.min_year || 1900}
                      onChange={(e) => updateSetting('min_year', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="1900"
                    />
                    <p className="text-xs text-slate-500 mt-1">Exclude movies/series released before this year.</p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Minimum Runtime (minutes)
                    </label>
                    <input
                      type="number"
                      value={settings.min_runtime || 0}
                      onChange={(e) => updateSetting('min_runtime', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="0"
                    />
                    <p className="text-xs text-slate-500 mt-1">Exclude short content (trailers, clips). 60+ recommended for movies.</p>
                  </div>
                </div>
              </div>

              {/* Stream Sorting Section */}
              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">🔢 Stream Sorting & Selection</h3>
                <p className="text-xs text-slate-500 mb-4">
                  Configure how streams are sorted and which one is selected for playback.
                </p>

                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <label className="block text-sm font-medium text-slate-300">Enable Release Filters</label>
                      <p className="text-xs text-slate-500">Apply the filter patterns above when selecting streams</p>
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
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Sort Priority Order
                    </label>
                    <select
                      value={settings.stream_sort_order || 'quality,size,seeders'}
                      onChange={(e) => updateSetting('stream_sort_order', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      <option value="quality,size,seeders">Quality → Size → Seeders (Default)</option>
                      <option value="quality,seeders,size">Quality → Seeders → Size</option>
                      <option value="size,quality,seeders">Size → Quality → Seeders</option>
                      <option value="seeders,quality,size">Seeders → Quality → Size</option>
                    </select>
                    <p className="text-xs text-slate-500 mt-1">
                      Order of priority when comparing streams. First field has highest priority.
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Selection Preference
                    </label>
                    <select
                      value={settings.stream_sort_prefer || 'best'}
                      onChange={(e) => updateSetting('stream_sort_prefer', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      <option value="best">Best Quality (Highest quality, largest size)</option>
                      <option value="smallest">Smallest File (Lowest size, data saver)</option>
                      <option value="balanced">Balanced (Good quality, reasonable size)</option>
                    </select>
                    <p className="text-xs text-slate-500 mt-1">
                      "Best" selects highest values, "Smallest" selects lowest values for each sort field.
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Content Tab */}
        {activeTab === 'content' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-blue-900/30 border border-blue-800 rounded-lg">
                <h3 className="text-red-400 font-medium mb-2">🎬 Content Availability</h3>
                <p className="text-sm text-slate-300">
                  Control which content appears in your IPTV apps based on stream availability.
                  The "Stream Search" background service periodically scans your library to check if streams are available.
                </p>
              </div>

              <div className="pt-4 border-t border-white/10">
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="hide_unavailable"
                    checked={settings.hide_unavailable_content || false}
                    onChange={(e) => updateSetting('hide_unavailable_content', e.target.checked)}
                    className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                  />
                  <label htmlFor="hide_unavailable" className="text-sm font-medium text-slate-300">
                    Hide Content Without Streams
                  </label>
                </div>
                <p className="text-xs text-slate-500 mt-1 ml-6">
                  Only show movies and episodes in IPTV apps if they have at least one stream available.
                  Content without streams will be hidden from your playlist but remain in your library.
                </p>
              </div>

              <div className="pt-4 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">📊 How It Works</h3>
                <div className="space-y-3 text-sm text-slate-400">
                  <div className="flex items-start gap-2">
                    <span className="text-red-400">1.</span>
                    <span>The "Stream Search" service runs periodically (check Services tab)</span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="text-red-400">2.</span>
                    <span>It checks Comet and MediaFusion for available streams</span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="text-red-400">3.</span>
                    <span>Movies and episodes are marked as "available" or "unavailable"</span>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="text-red-400">4.</span>
                    <span>When enabled above, unavailable content is filtered from IPTV apps</span>
                  </div>
                </div>
              </div>

              <div className="pt-4 border-t border-white/10">
                <div className="p-4 bg-yellow-900/20 border border-yellow-800 rounded-lg">
                  <h4 className="text-yellow-400 font-medium mb-2">💡 Tips</h4>
                  <ul className="text-sm text-slate-300 space-y-1 list-disc list-inside">
                    <li>New releases may not have streams immediately - give it a few days</li>
                    <li>Streams are re-checked every 7 days for items without streams</li>
                    <li>You can manually trigger a scan from the Services tab</li>
                    <li>This is especially useful for filtering out unreleased episodes</li>
                  </ul>
                </div>
              </div>

              <div className="pt-4 border-t border-white/10">
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="include_adult"
                    checked={settings.include_adult_vod || false}
                    onChange={(e) => updateSetting('include_adult_vod', e.target.checked)}
                    className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                  />
                  <label htmlFor="include_adult" className="text-sm font-medium text-slate-300">
                    Include Adult Content (TMDB)
                  </label>
                </div>
                <p className="text-xs text-slate-500 mt-1 ml-6">
                  Include adult-rated content (18+) in TMDB discovery and playlists.
                </p>
              </div>

              {/* Adult VOD Import Section */}
              <div className="pt-4 border-t border-white/10 bg-red-900/10 p-4 rounded-lg">
                <div className="flex items-center gap-2 mb-2">
                  <input
                    type="checkbox"
                    id="import_adult_vod"
                    checked={settings.import_adult_vod_from_github || false}
                    onChange={(e) => updateSetting('import_adult_vod_from_github', e.target.checked)}
                    className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                  />
                  <label htmlFor="import_adult_vod" className="text-sm font-medium text-slate-300">
                    Import Adult VOD from GitHub (18+)
                  </label>
                </div>
                <p className="text-xs text-slate-500 mt-1 ml-6 mb-3">
                  Enable importing adult VOD content from public-files GitHub repository. This is separate from TMDB adult content.
                </p>
                {settings.import_adult_vod_from_github && (
                  <button
                    onClick={async () => {
                      setMessage('⏳ Importing adult VOD from GitHub...');
                      try {
                        const response = await api.post('/adult-vod/import');
                        const data = response.data;
                        setMessage(`✅ ${data.message} - Imported: ${data.imported}, Skipped: ${data.skipped}, Errors: ${data.errors}`);
                        setTimeout(() => setMessage(''), 8000);
                      } catch (error: any) {
                        setMessage(`❌ Failed to import: ${error.response?.data?.error || error.message}`);
                        setTimeout(() => setMessage(''), 5000);
                      }
                    }}
                    className="ml-6 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 flex items-center gap-2 text-sm"
                  >
                    <Download className="w-4 h-4" />
                    Import Adult VOD Now
                  </button>
                )}
              </div>

              <div className="pt-4 border-t border-white/10">
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="only_released"
                    checked={settings.only_released_content || false}
                    onChange={(e) => updateSetting('only_released_content', e.target.checked)}
                    className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                  />
                  <label htmlFor="only_released" className="text-sm font-medium text-slate-300">
                    Only Include Released Content in Playlist
                  </label>
                </div>
                <p className="text-xs text-slate-500 mt-1 ml-6">
                  Only include movies/series in the IPTV playlist that are already released on streaming, digital, or Blu-ray. 
                  Unreleased items remain in your library but won't appear in the playlist until they're available for streaming.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Live TV Tab */}
        {activeTab === 'livetv' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-blue-900/30 border border-blue-800 rounded-lg">
                <h3 className="text-red-400 font-medium mb-2">📺 Live TV Configuration</h3>
                <p className="text-sm text-slate-300">
                  Manage Live TV channels from M3U sources. Currently loaded: <span className="font-bold text-white">{channelStats?.total_channels || 0}</span> channels
                  from <span className="font-bold text-white">{channelStats?.sources?.length || 0}</span> sources across <span className="font-bold text-white">{channelStats?.categories?.length || 0}</span> categories.
                </p>
              </div>

              {/* Enable Live TV */}
              <div className="p-4 bg-green-900/20 border border-green-800 rounded-lg">
                <div className="flex items-center justify-between">
                  <div>
                    <h4 className="text-white font-medium flex items-center gap-2">
                      <span>📺</span> Enable Live TV
                    </h4>
                    <p className="text-sm text-slate-400 mt-1">
                      Turn on Live TV channels in your M3U8 playlist. Configure sources below to add channels.
                    </p>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={settings.include_live_tv || false}
                      onChange={(e) => updateSetting('include_live_tv', e.target.checked)}
                      className="sr-only peer"
                    />
                    <div className="w-11 h-6 bg-gray-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-green-600"></div>
                  </label>
                </div>
              </div>

              {/* IPTV Import Mode */}
              <div className="p-4 bg-yellow-900/20 border border-yellow-800 rounded-lg">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h4 className="text-white font-medium flex items-center gap-2">
                      <span>🎬</span> IPTV Import Mode
                    </h4>
                    <p className="text-sm text-slate-400 mt-1">
                      Choose how to handle content from M3U/Xtream: live channels only, VOD only, or split both.
                    </p>
                  </div>
                  <div>
                    <select
                      value={settings.iptv_import_mode || 'live_only'}
                      onChange={(e) => updateSetting('iptv_import_mode', e.target.value)}
                      className="bg-[#333] text-white border border-white/10 rounded-md px-3 py-2"
                    >
                      <option value="live_only">Live TV only</option>
                      <option value="vod_only">VOD only</option>
                      <option value="both">Both (Live TV + VOD)</option>
                    </select>
                  </div>
                </div>
                <div className="mt-3 flex items-center justify-between gap-4">
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="duplicate_vod_per_provider"
                      checked={settings.duplicate_vod_per_provider || false}
                      onChange={(e) => {
                        const val = e.target.checked;
                        // Update local state and persist immediately
                        updateSetting('duplicate_vod_per_provider', val);
                        saveSettingsImmediate({ duplicate_vod_per_provider: val });
                      }}
                      className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                    />
                    <label htmlFor="duplicate_vod_per_provider" className="text-sm text-slate-300">
                      Duplicate VOD entries per provider in VOD list
                    </label>
                  </div>
                  <span className="text-xs text-slate-500">Helps clients that ignore multiple video sources</span>
                </div>
                <div className="mt-3 flex items-center justify-between gap-4">
                  <div className="flex items-center gap-2">
                    <label htmlFor="iptv_vod_sync_interval_hours" className="text-sm text-slate-300">
                      IPTV VOD Sync Interval (hours)
                    </label>
                    <input
                      type="number"
                      id="iptv_vod_sync_interval_hours"
                      min={1}
                      value={Number(settings.iptv_vod_sync_interval_hours || 6)}
                      onChange={(e) => updateSetting('iptv_vod_sync_interval_hours', Math.max(1, parseInt(e.target.value || '6', 10)))}
                      className="w-24 p-2 bg-[#2a2a2a] border border-white/10 rounded text-white"
                    />
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-xs text-slate-500">Controls auto-import/cleanup cadence (default 6h)</span>
                    <button
                      onClick={() => triggerService('iptv_vod_sync')}
                      disabled={triggeringService === 'iptv_vod_sync'}
                      className={`inline-flex items-center gap-2 px-3 py-2 rounded-md text-sm ${triggeringService === 'iptv_vod_sync' ? 'bg-gray-700 text-gray-300 cursor-not-allowed' : 'bg-blue-600 hover:bg-blue-700 text-white'}`}
                      title="Run IPTV VOD sync now"
                    >
                      <RefreshCw className={`w-4 h-4 ${triggeringService === 'iptv_vod_sync' ? 'animate-spin-slow' : ''}`} />
                      {triggeringService === 'iptv_vod_sync' ? 'Running…' : 'Run now'}
                    </button>
                  </div>
                </div>
                <div className="mt-3 flex items-center justify-between gap-4">
                  <p className="text-xs text-slate-400">
                    When VOD is selected (vod_only/both), import IPTV VOD items into your Library.
                  </p>
                  <button
                    onClick={async () => {
                      try {
                        const res = await api.post('iptv-vod/import');
                        const d = res.data || {};
                        setMessage(
                          `IPTV VOD import done: movies ${d.movies_imported || 0}, series ${d.series_imported || 0}, skipped ${d.skipped || 0}, errors ${d.errors || 0}`
                        );
                      } catch (err: any) {
                        console.error('IPTV VOD Import Error:', err);
                        const statusCode = err?.response?.status || 'unknown';
                        const errorMsg = err?.response?.data?.error || err?.response?.data || err.message;
                        setMessage(`Import failed (${statusCode}): ${errorMsg}`);
                      }
                    }}
                    className="inline-flex items-center gap-2 px-3 py-2 bg-green-600 hover:bg-green-700 text-white rounded-md text-sm"
                  >
                    <Download className="w-4 h-4" /> Import IPTV VOD to Library
                  </button>
                </div>
              </div>

              {/* Balkan VOD Import Section */}
              <div className="p-4 bg-blue-900/20 border border-blue-800 rounded-lg">
                <div className="flex items-center justify-between mb-2">
                  <div>
                    <h4 className="text-white font-medium flex items-center gap-2">
                      <span>🇧🇦</span> Balkan/Ex-Yu VOD from GitHub
                    </h4>
                    <p className="text-sm text-slate-400 mt-1">
                      Import domestic movies and series from Ex-Yugoslavia region (Serbian, Croatian, Bosnian content).
                    </p>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      type="checkbox"
                      checked={settings.balkan_vod_enabled || false}
                      onChange={(e) => updateSetting('balkan_vod_enabled', e.target.checked)}
                      className="sr-only peer"
                    />
                    <div className="w-11 h-6 bg-gray-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                  </label>
                </div>

                {settings.balkan_vod_enabled && (
                  <div className="mt-4 space-y-3">
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id="balkan_vod_auto_sync"
                        checked={settings.balkan_vod_auto_sync || false}
                        onChange={(e) => updateSetting('balkan_vod_auto_sync', e.target.checked)}
                        className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                      />
                      <label htmlFor="balkan_vod_auto_sync" className="text-sm text-slate-300">
                        Auto-sync new content
                      </label>
                    </div>

                    <div className="flex items-center gap-3">
                      <label htmlFor="balkan_vod_sync_interval" className="text-sm text-slate-300">
                        Sync interval:
                      </label>
                      <select
                        id="balkan_vod_sync_interval"
                        value={settings.balkan_vod_sync_interval_hours || 24}
                        onChange={(e) => updateSetting('balkan_vod_sync_interval_hours', parseInt(e.target.value))}
                        className="px-3 py-1.5 bg-[#2a2a2a] border border-white/10 rounded-lg text-sm"
                      >
                        <option value="6">Every 6 hours</option>
                        <option value="12">Every 12 hours</option>
                        <option value="24">Every 24 hours</option>
                        <option value="48">Every 2 days</option>
                        <option value="72">Every 3 days</option>
                        <option value="168">Weekly</option>
                      </select>
                    </div>

                    <div className="mt-3 p-3 bg-[#2a2a2a] rounded-lg">
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="text-sm text-slate-300 font-medium">Category Selection</p>
                          <p className="text-xs text-slate-500 mt-1">
                            {settings.balkan_vod_selected_categories && settings.balkan_vod_selected_categories.length > 0
                              ? `${settings.balkan_vod_selected_categories.length} categories selected`
                              : 'All categories (default)'}
                          </p>
                        </div>
                        <button
                          onClick={previewBalkanCategories}
                          disabled={loadingBalkanCategories}
                          className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-md text-sm disabled:opacity-50"
                        >
                          {loadingBalkanCategories ? (
                            <Loader className="w-4 h-4 animate-spin" />
                          ) : (
                            <Filter className="w-4 h-4" />
                          )}
                          {loadingBalkanCategories ? 'Loading...' : 'Select Categories'}
                        </button>
                      </div>
                    </div>

                    <div className="flex items-center justify-between gap-4 pt-2">
                      <span className="text-xs text-slate-500">Controls auto-import cadence (default 24h)</span>
                      <button
                        onClick={() => triggerService('balkan_vod_sync')}
                        disabled={triggeringService === 'balkan_vod_sync'}
                        className={`inline-flex items-center gap-2 px-3 py-2 rounded-md text-sm ${triggeringService === 'balkan_vod_sync' ? 'bg-gray-700 text-gray-300 cursor-not-allowed' : 'bg-blue-600 hover:bg-blue-700 text-white'}`}
                        title="Run Balkan VOD sync now"
                      >
                        <RefreshCw className={`w-4 h-4 ${triggeringService === 'balkan_vod_sync' ? 'animate-spin-slow' : ''}`} />
                        {triggeringService === 'balkan_vod_sync' ? 'Running…' : 'Run now'}
                      </button>
                    </div>
                  </div>
                )}
              </div>

              {/* Custom M3U Sources */}
              <div className="p-4 bg-purple-900/20 border border-purple-800 rounded-lg">
                <h4 className="text-white font-medium mb-4 flex items-center gap-2">
                  <span>📡</span> Custom M3U Sources
                </h4>
                <p className="text-sm text-slate-400 mb-4">
                  Add your own M3U playlist URLs with optional EPG support. Perfect for custom IPTV providers.
                </p>
                <p className="text-xs text-red-400 mb-4">
                  💡 Example sources: <a href="https://github.com/gogetta69/public-files" target="_blank" rel="noopener noreferrer" className="underline">gogetta69</a>, <a href="http://epg.serbianforum.org" target="_blank" rel="noopener noreferrer" className="underline">Serbian Forum</a>
                </p>
              </div>

              {/* Add New M3U Source */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">➕ Add M3U Source</h3>
                <div className="flex flex-col gap-3">
                  <div>
                    <label className="block text-sm text-slate-400 mb-1">Source Name</label>
                    <input
                      type="text"
                      value={newM3uName}
                      onChange={(e) => setNewM3uName(e.target.value)}
                      placeholder="e.g., My IPTV Provider"
                      className="w-full p-3 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:ring-2 focus:ring-red-500"
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-slate-400 mb-1">M3U URL</label>
                    <input
                      type="url"
                      value={newM3uUrl}
                      onChange={(e) => setNewM3uUrl(e.target.value)}
                      placeholder="https://example.com/playlist.m3u"
                      className="w-full p-3 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:ring-2 focus:ring-red-500"
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-slate-400 mb-1">EPG URL (Optional)</label>
                    <input
                      type="url"
                      value={newM3uEpg}
                      onChange={(e) => setNewM3uEpg(e.target.value)}
                      placeholder="https://example.com/epg.xml or http://epg.serbianforum.org/losmij/epg.xml.gz"
                      className="w-full p-3 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:ring-2 focus:ring-red-500"
                    />
                    <p className="text-xs text-slate-500 mt-1">XML or XMLTV format EPG for program guide data</p>
                  </div>
                  
                  {/* Preview Categories Button */}
                  <div className="flex gap-2">
                    <button
                      onClick={previewM3UCategories}
                      disabled={loadingCategories || !newM3uUrl.trim()}
                      className="flex items-center justify-center gap-2 px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {loadingCategories ? (
                        <>
                          <RefreshCw className="w-4 h-4 animate-spin" />
                          Loading Categories...
                        </>
                      ) : (
                        <>
                          <span>🏷️</span>
                          Preview Categories
                        </>
                      )}
                    </button>
                    {newM3uCategories.length > 0 && (
                      <span className="flex items-center gap-2 px-4 py-2 bg-purple-900/30 text-purple-300 rounded-lg text-sm">
                        {newM3uCategories.length} categories selected
                      </span>
                    )}
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

              {/* Category Selection Modal */}
              {showCategoryModal && (
                <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
                  <div className="bg-[#1a1a1a] border border-white/10 rounded-lg max-w-2xl w-full max-h-[80vh] overflow-hidden flex flex-col">
                    <div className="p-6 border-b border-white/10">
                      <div className="flex items-center justify-between">
                        <h3 className="text-xl font-semibold text-white">Select Categories to Import</h3>
                        <button
                          onClick={() => setShowCategoryModal(false)}
                          className="text-slate-400 hover:text-white"
                        >
                          <X className="w-6 h-6" />
                        </button>
                      </div>
                      <p className="text-sm text-slate-400 mt-2">
                        Choose which categories you want to import from this M3U source. Leave all unchecked to import everything.
                      </p>
                      <div className="flex gap-2 mt-4">
                        <button
                          onClick={selectAllCategories}
                          className="px-3 py-1 bg-green-600 text-white text-sm rounded hover:bg-green-700"
                        >
                          Select All
                        </button>
                        <button
                          onClick={deselectAllCategories}
                          className="px-3 py-1 bg-red-600 text-white text-sm rounded hover:bg-red-700"
                        >
                          Deselect All
                        </button>
                      </div>
                    </div>
                    
                    <div className="flex-1 overflow-y-auto p-6">
                      {availableCategories.length === 0 ? (
                        <p className="text-slate-400 text-center py-8">No categories found in this M3U file</p>
                      ) : (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                          {availableCategories.map((cat) => (
                            <label
                              key={cat.name}
                              className="flex items-center gap-3 p-3 bg-[#2a2a2a] border border-white/10 rounded-lg cursor-pointer hover:bg-[#333333] transition-colors"
                            >
                              <input
                                type="checkbox"
                                checked={newM3uCategories.includes(cat.name)}
                                onChange={() => toggleCategory(cat.name)}
                                className="w-4 h-4 bg-[#1a1a1a] border-white/10 rounded"
                              />
                              <div className="flex-1 min-w-0">
                                <div className="font-medium text-white truncate">{cat.name}</div>
                                <div className="text-xs text-slate-500">{cat.count} items</div>
                              </div>
                            </label>
                          ))}
                        </div>
                      )}
                    </div>
                    
                    <div className="p-6 border-t border-white/10 flex justify-end gap-3">
                      <button
                        onClick={() => {
                          setShowCategoryModal(false);
                          setNewM3uCategories([]);
                        }}
                        className="px-4 py-2 bg-[#2a2a2a] text-white rounded-lg hover:bg-[#333333]"
                      >
                        Cancel
                      </button>
                      <button
                        onClick={() => setShowCategoryModal(false)}
                        className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700"
                      >
                        Done ({newM3uCategories.length} selected)
                      </button>
                    </div>
                  </div>
                </div>
              )}

              {/* Xtream Category Selection Modal */}
              {showXtreamCategoryModal && (
                <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
                  <div className="bg-[#1a1a1a] border border-white/10 rounded-lg max-w-2xl w-full max-h-[80vh] overflow-hidden flex flex-col">
                    <div className="p-6 border-b border-white/10">
                      <div className="flex items-center justify-between">
                        <h3 className="text-xl font-semibold text-white">Select Categories to Import</h3>
                        <button
                          onClick={() => setShowXtreamCategoryModal(false)}
                          className="text-slate-400 hover:text-white"
                        >
                          <X className="w-6 h-6" />
                        </button>
                      </div>
                      <p className="text-sm text-slate-400 mt-2">
                        Choose which categories you want to import from this Xtream source. Leave all unchecked to import everything.
                      </p>
                      <div className="flex gap-2 mt-4">
                        <button
                          onClick={selectAllXtreamCategories}
                          className="px-3 py-1 bg-green-600 text-white text-sm rounded hover:bg-green-700"
                        >
                          Select All
                        </button>
                        <button
                          onClick={deselectAllXtreamCategories}
                          className="px-3 py-1 bg-red-600 text-white text-sm rounded hover:bg-red-700"
                        >
                          Deselect All
                        </button>
                      </div>
                    </div>
                    
                    <div className="flex-1 overflow-y-auto p-6">
                      {availableXtreamCategories.length === 0 ? (
                        <p className="text-slate-400 text-center py-8">No categories found</p>
                      ) : (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                          {availableXtreamCategories.map((cat) => (
                            <label
                              key={cat.name}
                              className="flex items-center gap-3 p-3 bg-[#2a2a2a] border border-white/10 rounded-lg cursor-pointer hover:bg-[#333333] transition-colors"
                            >
                              <input
                                type="checkbox"
                                checked={newXtreamCategories.includes(cat.name)}
                                onChange={() => toggleXtreamCategory(cat.name)}
                                className="w-4 h-4 bg-[#1a1a1a] border-white/10 rounded"
                              />
                              <div className="flex-1 min-w-0">
                                <div className="font-medium text-white truncate">{cat.name}</div>
                                {cat.count > 0 && <div className="text-xs text-slate-500">{cat.count} items</div>}
                              </div>
                            </label>
                          ))}
                        </div>
                      )}
                    </div>
                    
                    <div className="p-6 border-t border-white/10 flex justify-end gap-3">
                      <button
                        onClick={() => {
                          setShowXtreamCategoryModal(false);
                          setNewXtreamCategories([]);
                        }}
                        className="px-4 py-2 bg-[#2a2a2a] text-white rounded-lg hover:bg-[#333333]"
                      >
                        Cancel
                      </button>
                      <button
                        onClick={() => setShowXtreamCategoryModal(false)}
                        className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700"
                      >
                        Done ({newXtreamCategories.length} selected)
                      </button>
                    </div>
                  </div>
                </div>
              )}

              {/* Balkan Category Selection Modal */}
              {showBalkanCategoryModal && (
                <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
                  <div className="bg-[#1a1a1a] border border-white/10 rounded-lg max-w-2xl w-full max-h-[80vh] overflow-hidden flex flex-col">
                    <div className="p-6 border-b border-white/10">
                      <div className="flex items-center justify-between">
                        <h3 className="text-xl font-semibold text-white">🇧🇦 Select Balkan VOD Categories</h3>
                        <button
                          onClick={() => setShowBalkanCategoryModal(false)}
                          className="text-slate-400 hover:text-white"
                        >
                          <X className="w-6 h-6" />
                        </button>
                      </div>
                      <p className="text-sm text-slate-400 mt-2">
                        Choose which categories you want to import from Balkan GitHub repos. Leave all unchecked to import everything.
                      </p>
                      <div className="flex gap-2 mt-4">
                        <button
                          onClick={selectAllBalkanCategories}
                          className="px-3 py-1 bg-green-600 text-white text-sm rounded hover:bg-green-700"
                        >
                          Select All
                        </button>
                        <button
                          onClick={deselectAllBalkanCategories}
                          className="px-3 py-1 bg-red-600 text-white text-sm rounded hover:bg-red-700"
                        >
                          Deselect All
                        </button>
                      </div>
                    </div>
                    
                    <div className="flex-1 overflow-y-auto p-6">
                      {availableBalkanCategories.length === 0 ? (
                        <p className="text-slate-400 text-center py-8">No categories found</p>
                      ) : (
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                          {availableBalkanCategories.map((cat) => (
                            <label
                              key={cat.name}
                              className="flex items-center gap-3 p-3 bg-[#2a2a2a] border border-white/10 rounded-lg cursor-pointer hover:bg-[#333333] transition-colors"
                            >
                              <input
                                type="checkbox"
                                checked={newBalkanCategories.includes(cat.name)}
                                onChange={() => toggleBalkanCategory(cat.name)}
                                className="w-4 h-4 bg-[#1a1a1a] border-white/10 rounded"
                              />
                              <div className="flex-1 min-w-0">
                                <div className="font-medium text-white truncate">{cat.name}</div>
                                {cat.count > 0 && <div className="text-xs text-slate-500">{cat.count} items</div>}
                              </div>
                            </label>
                          ))}
                        </div>
                      )}
                    </div>
                    
                    <div className="p-6 border-t border-white/10 flex justify-end gap-3">
                      <button
                        onClick={() => {
                          setShowBalkanCategoryModal(false);
                          setNewBalkanCategories([]);
                        }}
                        className="px-4 py-2 bg-[#2a2a2a] text-white rounded-lg hover:bg-[#333333]"
                      >
                        Cancel
                      </button>
                      <button
                        onClick={saveBalkanCategories}
                        className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                      >
                        Save ({newBalkanCategories.length} selected)
                      </button>
                    </div>
                  </div>
                </div>
              )}

              {/* Current M3U Sources */}
              {m3uSources.length > 0 && (
                <div>
                  <div className="flex items-center justify-between mb-4">
                    <h3 className="text-lg font-medium text-white">🔗 Your Custom M3U Sources</h3>
                    <button
                      onClick={checkAllSourcesStatus}
                      disabled={checkingAllSources}
                      className="flex items-center gap-2 px-3 py-1.5 bg-red-600 text-white text-sm rounded-lg hover:bg-red-700 disabled:opacity-50"
                    >
                      <RefreshCw className={`w-4 h-4 ${checkingAllSources ? 'animate-spin' : ''}`} />
                      {checkingAllSources ? 'Checking...' : 'Check All'}
                    </button>
                  </div>
                  <div className="space-y-2">
                    {m3uSources.map((source, index) => (
                      <div
                        key={index}
                        className={`flex items-center justify-between p-3 rounded-lg border ${
                          source.enabled
                            ? 'bg-[#2a2a2a] border-white/10'
                            : 'bg-[#2a2a2a]/50 border-white/10 opacity-60'
                        }`}
                      >
                        <div className="flex items-center gap-3 flex-1 min-w-0">
                          {getSourceStatusIndicator(source.url)}
                          <input
                            type="checkbox"
                            checked={source.enabled}
                            onChange={() => toggleM3uSource(index)}
                            className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                          />
                          <div className="min-w-0 flex-1">
                            <div className="font-medium text-white truncate">{source.name}</div>
                            <div className="text-xs text-slate-500 truncate">{source.url}</div>
                            {source.epg_url && (
                              <div className="text-xs text-red-400 truncate flex items-center gap-1 mt-1">
                                <span>📺</span> EPG: {source.epg_url}
                              </div>
                            )}
                            {source.selected_categories && source.selected_categories.length > 0 && (
                              <div className="text-xs text-purple-400 flex items-center gap-1 mt-1">
                                <span>🏷️</span> {source.selected_categories.length} categories: {source.selected_categories.slice(0, 3).join(', ')}
                                {source.selected_categories.length > 3 && '...'}
                              </div>
                            )}
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => checkSourceStatus(source.url)}
                            className="p-1 text-red-400 hover:text-blue-300"
                            title="Check status"
                          >
                            <RefreshCw className={`w-4 h-4 ${sourceStatuses.get(source.url)?.checking ? 'animate-spin' : ''}`} />
                          </button>
                          <button
                            onClick={() => removeM3uSource(index)}
                            className="p-1 text-red-400 hover:text-red-300"
                          >
                            <X className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <hr className="border-white/10" />

              {/* Custom Xtream Sources */}
              <div className="p-4 bg-cyan-900/20 border border-cyan-800 rounded-lg">
                <h4 className="text-white font-medium mb-4 flex items-center gap-2">
                  <span>📺</span> Custom Xtream Sources
                </h4>
                <p className="text-sm text-slate-400 mb-4">
                  Add Xtream Codes compatible IPTV providers. These require a server URL, username, and password from your IPTV provider.
                </p>
              </div>

              {/* Add New Xtream Source */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">➕ Add Xtream Source</h3>
                <div className="flex flex-col gap-3">
                  <div>
                    <label className="block text-sm text-slate-400 mb-1">Source Name</label>
                    <input
                      type="text"
                      value={newXtreamName}
                      onChange={(e) => setNewXtreamName(e.target.value)}
                      placeholder="e.g., My IPTV Provider"
                      className="w-full p-3 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:ring-2 focus:ring-cyan-500"
                    />
                  </div>
                  <div>
                    <label className="block text-sm text-slate-400 mb-1">Server URL</label>
                    <input
                      type="url"
                      value={newXtreamUrl}
                      onChange={(e) => setNewXtreamUrl(e.target.value)}
                      placeholder="http://provider.com:8080"
                      className="w-full p-3 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:ring-2 focus:ring-cyan-500"
                    />
                    <p className="text-xs text-slate-500 mt-1">The base URL of your IPTV provider (without /player_api.php)</p>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-sm text-slate-400 mb-1">Username</label>
                      <input
                        type="text"
                        value={newXtreamUsername}
                        onChange={(e) => setNewXtreamUsername(e.target.value)}
                        placeholder="your_username"
                        className="w-full p-3 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:ring-2 focus:ring-cyan-500"
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-slate-400 mb-1">Password</label>
                      <input
                        type="text"
                        value={newXtreamPassword}
                        onChange={(e) => setNewXtreamPassword(e.target.value)}
                        placeholder="your_password"
                        className="w-full p-3 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:ring-2 focus:ring-cyan-500"
                      />
                    </div>
                  </div>
                  
                  {/* Category Preview and Selection */}
                  <div className="flex items-center gap-2">
                    <button
                      onClick={previewXtreamCategories}
                      disabled={loadingXtreamCategories || !newXtreamUrl.trim() || !newXtreamUsername.trim() || !newXtreamPassword.trim()}
                      className="flex items-center justify-center gap-2 px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {loadingXtreamCategories ? (
                        <>
                          <Loader className="w-4 h-4 animate-spin" />
                          Loading...
                        </>
                      ) : (
                        <>
                          <Search className="w-4 h-4" />
                          Preview Categories
                        </>
                      )}
                    </button>
                    {newXtreamCategories.length > 0 && (
                      <span className="text-sm text-green-400">
                        \u2705 {newXtreamCategories.length} categories selected
                      </span>
                    )}
                  </div>
                  
                  <button
                    onClick={addXtreamSource}
                    className="flex items-center justify-center gap-2 px-4 py-2 bg-cyan-600 text-white rounded-lg hover:bg-cyan-700 w-fit"
                  >
                    <Plus className="w-4 h-4" />
                    Add Source
                  </button>
                </div>
              </div>

              {/* Current Xtream Sources */}
              {xtreamSources.length > 0 && (
                <div>
                  <h3 className="text-lg font-medium text-white mb-4">🔗 Your Custom Xtream Sources</h3>
                  <div className="space-y-2">
                    {xtreamSources.map((source, index) => (
                      <div
                        key={index}
                        className={`flex items-center justify-between p-3 rounded-lg border ${
                          source.enabled
                            ? 'bg-[#2a2a2a] border-white/10'
                            : 'bg-[#2a2a2a]/50 border-white/10 opacity-60'
                        }`}
                      >
                        <div className="flex items-center gap-3 flex-1 min-w-0">
                          <input
                            type="checkbox"
                            checked={source.enabled}
                            onChange={() => toggleXtreamSource(index)}
                            className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                          />
                          <div className="min-w-0 flex-1">
                            <div className="font-medium text-white truncate">{source.name}</div>
                            <div className="text-xs text-slate-500 truncate">{source.server_url}</div>
                            <div className="text-xs text-cyan-400 truncate flex items-center gap-1 mt-1">
                              <span>👤</span> {source.username}
                            </div>
                            {source.selected_categories && source.selected_categories.length > 0 && (
                              <div className="text-xs text-purple-400 truncate flex items-center gap-1 mt-1">
                                <span>\ud83c\udff7\ufe0f</span> {source.selected_categories.length} categories: {source.selected_categories.slice(0, 3).join(', ')}
                                {source.selected_categories.length > 3 && '...'}
                              </div>
                            )}
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => removeXtreamSource(index)}
                            className="p-1 text-red-400 hover:text-red-300"
                          >
                            <X className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div className="p-4 bg-yellow-900/20 border border-yellow-800 rounded-lg">
                <h4 className="text-yellow-400 font-medium mb-2">💡 Tips</h4>
                <ul className="text-sm text-slate-300 space-y-1 list-disc list-inside">
                  <li>Local M3U file: <code className="text-xs bg-[#2a2a2a] px-1 rounded">./channels/m3u_formatted.dat</code></li>
                  <li>M3U and Xtream sources are loaded when the server starts</li>
                  <li>After adding/removing sources, save and restart the server</li>
                  <li>Duplicate channels (same name) are automatically merged</li>
                </ul>
              </div>
            </div>
          </div>
        )}

        {/* Stremio Tab */}
        {activeTab === 'stremio' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-purple-900/30 border border-purple-800 rounded-lg">
                <h3 className="text-purple-400 font-medium mb-2 flex items-center gap-2">
                  <Play className="w-5 h-5" />
                  Stremio Addon
                </h3>
                <p className="text-sm text-slate-300">
                  Enable the built-in Stremio addon to stream your library directly in Stremio. Configure which catalogs to show and customize their names.
                </p>
                <p className="text-sm text-yellow-400 mt-2">
                  💡 Installation: In Stremio go to Add-ons → Community → Add-on Repository → Install from URL, then paste the Manifest URL from this page (use "Copy Manifest URL"). For remote access, set "Server Host/IP" below to your public domain/IP and open the server port.
                </p>
              </div>

              {/* Enable Addon */}
              <div>
                <label className="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={settings.stremio_addon?.enabled || false}
                    onChange={async (e) => {
                      if (!settings) return;
                      const currentAddon = settings.stremio_addon || {};
                      const newAddon = { ...currentAddon, enabled: e.target.checked };
                      updateSetting('stremio_addon', newAddon);
                      // Auto-save when enabling/disabling
                      try {
                        // Make sure to send ALL current settings with the updated addon
                        const settingsToSave = { ...settings, stremio_addon: newAddon };
                        await api.put('/settings', settingsToSave);
                        setMessage(e.target.checked ? 'Stremio addon enabled' : 'Stremio addon disabled');
                        setTimeout(() => setMessage(''), 2000);
                      } catch (error) {
                        console.error('Failed to save:', error);
                        setMessage('Failed to save setting');
                      }
                    }}
                    className="w-5 h-5 text-red-600 bg-gray-700 border-gray-600 rounded focus:ring-red-500"
                  />
                  <span className="text-white font-medium">Enable Stremio Addon</span>
                </label>
              </div>

              {settings.stremio_addon?.enabled && (
                <>
                  {/* Addon Name */}
                  <div>
                    <label className="block text-sm text-slate-300 mb-2">Addon Name</label>
                    <input
                      type="text"
                      value={settings.stremio_addon?.addon_name || 'StreamArr Pro'}
                      onChange={(e) => updateSetting('stremio_addon', { ...settings.stremio_addon, addon_name: e.target.value })}
                      placeholder="StreamArr Pro"
                      className="w-full p-3 bg-gray-700 border border-gray-600 rounded-lg text-white"
                    />
                    <p className="text-xs text-slate-500 mt-1">The name shown in Stremio's addon list</p>
                  </div>

                  {/* Server Host/IP */}
                  <div>
                    <label className="block text-sm text-slate-300 mb-2">Server Host/IP</label>
                    <input
                      type="text"
                      value={settings.stremio_addon?.public_server_url || ''}
                      onChange={(e) => updateSetting('stremio_addon', { ...settings.stremio_addon, public_server_url: e.target.value })}
                      placeholder="e.g., streamarr.mydomain.com:8080 or 123.45.67.89:8080"
                      className="w-full p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                    />
                    <p className="text-xs text-slate-500 mt-1">
                      Your public domain or IP with port if needed. Leave empty to auto-detect. Required for using the addon outside your home network.
                    </p>
                  </div>

                  {/* Authentication Token */}
                  <div>
                    <label className="block text-sm text-slate-300 mb-2">Shared Access Token</label>
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={settings.stremio_addon?.shared_token || ''}
                        readOnly
                        placeholder="Click 'Generate Token' to create"
                        className="flex-1 p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                      />
                      <button
                        onClick={async () => {
                          try {
                            const response = await api.post('/stremio/generate-token');
                            if (!settings) return;
                            const currentAddon = settings.stremio_addon || {};
                            // Ensure addon is enabled when token is generated
                            const newAddon = { ...currentAddon, enabled: true, shared_token: response.data.token };
                            updateSetting('stremio_addon', newAddon);
                            // Auto-save the token and ensure enabled flag is set
                            const settingsToSave = { ...settings, stremio_addon: newAddon };
                            await api.put('/settings', settingsToSave);
                            setMessage('Token generated successfully');
                            setTimeout(() => setMessage(''), 2000);
                          } catch (error: any) {
                            console.error('Failed to generate token:', error);
                            const errorMsg = error?.response?.data?.error || error?.message || 'Failed to generate token';
                            setMessage(errorMsg);
                          }
                        }}
                        className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 whitespace-nowrap"
                      >
                        Generate Token
                      </button>
                    </div>
                    <p className="text-xs text-slate-500 mt-1">
                      Secure token for addon authentication. Keep this private!
                    </p>
                  </div>

                  {/* Catalog Configuration */}
                  <div>
                    <h4 className="text-white font-medium mb-3">Library Catalogs</h4>
                    <div className="space-y-3">
                      {(settings.stremio_addon?.catalogs || []).map((catalog, index) => (
                        <div key={catalog.id} className="bg-[#2a2a2a] rounded-lg p-4 border border-gray-700">
                          <div className="flex items-start gap-4">
                            <label className="flex items-center gap-3 cursor-pointer">
                              <input
                                type="checkbox"
                                checked={catalog.enabled}
                                onChange={(e) => {
                                  const currentAddon = settings.stremio_addon || {};
                                  const newCatalogs = [...(currentAddon.catalogs || [])];
                                  newCatalogs[index] = { ...catalog, enabled: e.target.checked };
                                  updateSetting('stremio_addon', { ...currentAddon, catalogs: newCatalogs });
                                }}
                                className="w-5 h-5 text-red-600 bg-gray-700 border-gray-600 rounded focus:ring-red-500"
                              />
                            </label>
                            <div className="flex-1">
                              <div className="mb-2">
                                <span className="text-sm text-slate-400 uppercase">{catalog.type}</span>
                              </div>
                              <input
                                type="text"
                                value={catalog.name}
                                onChange={(e) => {
                                  const currentAddon = settings.stremio_addon || {};
                                  const newCatalogs = [...(currentAddon.catalogs || [])];
                                  newCatalogs[index] = { ...catalog, name: e.target.value };
                                  updateSetting('stremio_addon', { ...currentAddon, catalogs: newCatalogs });
                                }}
                                placeholder="Catalog Name"
                                className="w-full p-2 bg-gray-700 border border-gray-600 rounded-lg text-white text-sm"
                              />
                              <p className="text-xs text-slate-500 mt-1">
                                Catalog ID: {catalog.id}
                              </p>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                    <p className="text-xs text-slate-500 mt-2">
                      Enable/disable catalogs and customize their names as shown in Stremio
                    </p>
                  </div>

                  {/* Catalog Placement removed per request */}

                  {/* Manifest URL */}
                  {settings.stremio_addon?.shared_token && (
                    <div className="bg-[#2a2a2a] rounded-lg p-4 border border-green-800">
                      <h4 className="text-green-400 font-medium mb-3 flex items-center gap-2">
                        <CheckCircle className="w-5 h-5" />
                        Addon Ready to Install
                      </h4>
                      <div className="space-y-3">
                        <button
                          onClick={async () => {
                            try {
                              const response = await api.get('/stremio/manifest-url');
                              const url = response.data.manifest_url;
                              navigator.clipboard.writeText(url);
                              setMessage('Manifest URL copied! Paste it in Stremio');
                              setTimeout(() => setMessage(''), 3000);
                            } catch (error: any) {
                              console.error('Failed to get manifest URL:', error);
                              const errorMsg = error?.response?.data?.error || error?.message || 'Failed to get manifest URL';
                              setMessage(errorMsg);
                            }
                          }}
                          className="w-full px-4 py-3 bg-red-600 text-white rounded-lg hover:bg-red-700 font-medium"
                        >
                          Copy Manifest URL
                        </button>
                        <div className="bg-blue-900/30 border border-blue-800 rounded p-3">
                          <p className="text-sm text-blue-200 font-medium mb-2">📋 Installation Steps:</p>
                          <ol className="text-xs text-slate-300 space-y-1 list-decimal list-inside">
                            <li>Click "Copy Manifest URL" above</li>
                            <li>Open Stremio → Add-ons (puzzle icon)</li>
                            <li>Scroll to bottom → Click "+ Add-on Repository"</li>
                            <li>Paste the URL and click OK</li>
                            <li>Your addon will appear—streams show when playing titles!</li>
                          </ol>
                        </div>
                      </div>
                    </div>
                  )}

                  {!settings.stremio_addon?.shared_token && (
                    <div className="bg-yellow-900/30 border border-yellow-800 rounded-lg p-4">
                      <p className="text-yellow-400 text-sm flex items-center gap-2">
                        <AlertCircle className="w-4 h-4" />
                        Generate a token to get your manifest URL
                      </p>
                    </div>
                  )}
                </>
              )}
            </div>
          </div>
        )}

        {/* Filters Tab */}
        {activeTab === 'filters' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-blue-900/30 border border-blue-800 rounded-lg">
                <h3 className="text-red-400 font-medium mb-2">🔍 Filter Settings</h3>
                <p className="text-sm text-slate-400">
                  Filter out unwanted releases by release group, language, or quality. 
                  Separate multiple patterns with <code className="text-xs bg-[#2a2a2a] px-1 rounded">|</code> (pipe character).
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Excluded Release Groups
                </label>
                <input
                  type="text"
                  value={settings.excluded_release_groups || ''}
                  onChange={(e) => updateSetting('excluded_release_groups', e.target.value)}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                  placeholder="TVHUB|FILM"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Block releases from specific groups. Example: <code className="bg-[#2a2a2a] px-1 rounded">TVHUB|FILM</code> blocks Russian releases like "Movie.TVHUB.FILM.mkv"
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Excluded Language Tags
                </label>
                <input
                  type="text"
                  value={settings.excluded_language_tags || ''}
                  onChange={(e) => updateSetting('excluded_language_tags', e.target.value)}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                  placeholder="RUSSIAN|RUS|HINDI|HIN|GERMAN|GER|FRENCH|FRE|ITALIAN|ITA|SPANISH|SPA|LATINO"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Block releases with language indicators in filename. Example: <code className="bg-[#2a2a2a] px-1 rounded">RUSSIAN|RUS|HINDI|GERMAN</code>
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Excluded Qualities
                </label>
                <input
                  type="text"
                  value={settings.excluded_qualities || ''}
                  onChange={(e) => updateSetting('excluded_qualities', e.target.value)}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                  placeholder="REMUX|HDR|DV|Dolby.?Vision|3D|CAM|TS|SCR|HDTS|HDCAM|TELESYNC|TELECINE|TC"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Block certain quality types. Example: <code className="bg-[#2a2a2a] px-1 rounded">REMUX|HDR|CAM|TS</code> blocks REMUX (too large), HDR (compatibility), CAM/TS (low quality)
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Custom Exclude Patterns (Advanced)
                </label>
                <input
                  type="text"
                  value={settings.custom_exclude_patterns || ''}
                  onChange={(e) => updateSetting('custom_exclude_patterns', e.target.value)}
                  className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                  placeholder="Sample|Trailer|\\[Dual\\]"
                />
                <p className="text-xs text-slate-500 mt-1">
                  Custom regex patterns. Example: <code className="bg-[#2a2a2a] px-1 rounded">Sample|Trailer</code> blocks sample files and trailers
                </p>
              </div>

              {/* Filter Preview */}
              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">Filter Preview</h3>
                <p className="text-xs text-slate-500 mb-3">These release names would be blocked:</p>
                <div className="bg-[#2a2a2a] rounded-lg p-4 font-mono text-sm space-y-1">
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
                    <div className="text-slate-500">No filters configured. Add patterns above to see preview.</div>
                  )}
                </div>
              </div>

              {/* Common Presets */}
              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">Common Presets</h3>
                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={() => {
                      updateSetting('excluded_language_tags', 'RUSSIAN|RUS|HINDI|HIN|GERMAN|GER|FRENCH|FRE|ITALIAN|ITA|SPANISH|SPA|LATINO|PORTUGUESE|POR|KOREAN|KOR|JAPANESE|JAP|CHINESE|CHI|ARABIC|ARA|TURKISH|TUR|POLISH|POL|DUTCH|DUT|THAI|VIETNAMESE|INDONESIAN');
                    }}
                    className="px-3 py-2 bg-red-600/20 text-red-400 rounded-lg hover:bg-red-600/30 transition-colors border border-blue-600/30 text-sm"
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
                <p className="text-xs text-slate-500 mt-3">
                  Presets will replace the corresponding filter field. Click "Save Changes" to apply.
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Services Tab */}
        {activeTab === 'services' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-blue-900/30 border border-blue-800 rounded-lg">
                <h3 className="text-red-400 font-medium mb-2">⚙️ Background Services</h3>
                <p className="text-sm text-slate-400">
                  Monitor and control background tasks. Services run automatically at their configured intervals,
                  or you can trigger them manually. Data refreshes every 5 seconds while on this tab.
                </p>
              </div>

              <div className="space-y-4">
                {services.length === 0 ? (
                  <div className="text-slate-500 text-center py-8">
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
                          ? 'bg-[#2a2a2a]/50 border-white/10'
                          : 'bg-[#2a2a2a]/30 border-white/10 opacity-60'
                      }`}
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex-1">
                          <div className="flex items-center gap-3">
                            <h4 className="text-white font-medium">{formatServiceName(service.name)}</h4>
                            {service.running && (
                              <span className="flex items-center gap-1 text-xs bg-red-500/30 text-red-400 px-2 py-0.5 rounded">
                                <RefreshCw className="w-3 h-3 animate-spin" />
                                Running
                              </span>
                            )}
                            {!service.enabled && (
                              <span className="text-xs bg-gray-600/50 text-slate-400 px-2 py-0.5 rounded">
                                Disabled
                              </span>
                            )}
                          </div>
                          <p className="text-sm text-slate-400 mt-1">{service.description}</p>
                          
                          {/* Progress Bar - Show when running */}
                          {service.running && service.items_total > 0 && (
                            <div className="mt-3">
                              <div className="flex justify-between text-xs text-slate-400 mb-1">
                                <span>{service.progress_message || 'Processing...'}</span>
                                <span>{service.items_processed}/{service.items_total} ({service.progress}%)</span>
                              </div>
                              <div className="w-full bg-gray-700 rounded-full h-2 overflow-hidden">
                                <div 
                                  className="bg-red-500 h-full rounded-full transition-all duration-300"
                                  style={{ width: `${service.progress}%` }}
                                />
                              </div>
                            </div>
                          )}
                          
                          {/* Current Activity - Show when running without total */}
                          {service.running && service.items_total === 0 && service.progress_message && (
                            <div className="mt-2 text-xs text-red-400 bg-blue-900/30 px-2 py-1 rounded flex items-center gap-2">
                              <RefreshCw className="w-3 h-3 animate-spin" />
                              {service.progress_message}
                            </div>
                          )}
                          
                          <div className="flex flex-wrap gap-4 mt-3 text-xs text-slate-500">
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
                                ? 'bg-gray-700 text-slate-300 hover:bg-gray-600'
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
                                ? 'bg-gray-700 text-slate-500 cursor-not-allowed'
                                : 'bg-red-600 text-white hover:bg-red-700'
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
              <div className="pt-4 border-t border-white/10">
                <h4 className="text-sm font-medium text-slate-400 mb-3">Service Information</h4>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs text-slate-500">
                  <div>
                    <strong className="text-slate-400">Playlist Generation:</strong> Creates M3U8 playlist file with all library content for IPTV players
                  </div>
                  <div>
                    <strong className="text-slate-400">Cache Cleanup:</strong> Removes expired stream links and temporary data
                  </div>
                  <div>
                    <strong className="text-slate-400">EPG Update:</strong> Fetches program guide data for Live TV channels
                  </div>
                  <div>
                    <strong className="text-slate-400">Channel Refresh:</strong> Reloads Live TV channels from configured M3U sources
                  </div>
                  <div>
                    <strong className="text-slate-400">MDBList Sync:</strong> Imports content from your MDBList watchlists
                  </div>
                  <div>
                    <strong className="text-slate-400">Collection Sync:</strong> Adds missing movies from incomplete collections
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Xtream Tab */}
        {activeTab === 'xtream' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-purple-900/30 border border-purple-800 rounded-lg">
                <h3 className="text-purple-400 font-medium mb-2">📡 Xtream Codes API</h3>
                <p className="text-sm text-slate-300">
                  StreamArr exposes an Xtream Codes compatible API that can be used with IPTV players like TiviMate, XCIPTV, or OTT Navigator.
                  Use the connection details below to configure your player.
                </p>
                <p className="text-sm text-yellow-400 mt-2">
                  ⚠️ <strong>Note:</strong> These credentials are separate from your web app login. They're only used by IPTV players to access the Xtream API.
                </p>
              </div>

              {/* Xtream Credentials Configuration */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">🔐 Xtream API Credentials</h3>
                <div className="bg-[#2a2a2a] rounded-lg p-4 space-y-4">
                  <p className="text-sm text-slate-400 mb-2">
                    Set custom credentials for your IPTV players. These are separate from your web dashboard login.
                  </p>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm text-slate-300 mb-2">Xtream Username</label>
                      <input
                        type="text"
                        value={settings.xtream_username || 'streamarr'}
                        onChange={(e) => updateSetting('xtream_username', e.target.value)}
                        className="w-full p-3 bg-gray-700 border border-gray-600 rounded-lg text-white"
                        placeholder="streamarr"
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-slate-300 mb-2">Xtream Password</label>
                      <input
                        type="text"
                        value={settings.xtream_password || 'streamarr'}
                        onChange={(e) => updateSetting('xtream_password', e.target.value)}
                        className="w-full p-3 bg-gray-700 border border-gray-600 rounded-lg text-white"
                        placeholder="streamarr"
                      />
                    </div>
                  </div>
                  <p className="text-xs text-slate-500">
                    💡 Use these credentials in your IPTV player, not your web app password. Click "Save Changes" at the top after modifying.
                  </p>
                </div>
              </div>

              {/* Connection Details */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">🔗 Connection Details</h3>
                <div className="bg-[#2a2a2a] rounded-lg p-4 space-y-4">
                  <div>
                    <label className="block text-sm text-slate-400 mb-1">Server URL</label>
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
                        className="px-3 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
                      >
                        Copy
                      </button>
                    </div>
                    <p className="text-xs text-yellow-400 mt-1">
                      ⚠️ Make sure port {settings.server_port || 8080} is accessible from your IPTV player device
                    </p>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm text-slate-400 mb-1">Username</label>
                      <div className="flex gap-2">
                        <input
                          type="text"
                          value={settings.xtream_username || 'streamarr'}
                          readOnly
                          className="flex-1 p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                        />
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(settings.xtream_username || 'streamarr');
                            setMessage('Username copied to clipboard');
                            setTimeout(() => setMessage(''), 2000);
                          }}
                          className="px-3 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
                        >
                          Copy
                        </button>
                      </div>
                    </div>
                    <div>
                      <label className="block text-sm text-slate-400 mb-1">Password</label>
                      <div className="flex gap-2">
                        <input
                          type="text"
                          value={settings.xtream_password || 'streamarr'}
                          readOnly
                          className="flex-1 p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                        />
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(settings.xtream_password || 'streamarr');
                            setMessage('Password copied to clipboard');
                            setTimeout(() => setMessage(''), 2000);
                          }}
                          className="px-3 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
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
                <div className="bg-[#2a2a2a] rounded-lg p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Player API</div>
                      <code className="text-xs text-slate-400">/player_api.php</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get Live Categories</div>
                      <code className="text-xs text-slate-400">/player_api.php?action=get_live_categories</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get Live Streams</div>
                      <code className="text-xs text-slate-400">/player_api.php?action=get_live_streams</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get VOD Categories</div>
                      <code className="text-xs text-slate-400">/player_api.php?action=get_vod_categories</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get VOD Streams</div>
                      <code className="text-xs text-slate-400">/player_api.php?action=get_vod_streams</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get Series Categories</div>
                      <code className="text-xs text-slate-400">/player_api.php?action=get_series_categories</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-white font-medium">Get Series</div>
                      <code className="text-xs text-slate-400">/player_api.php?action=get_series</code>
                    </div>
                    <span className="px-2 py-1 bg-green-600 text-white text-xs rounded">Active</span>
                  </div>
                </div>
              </div>

              {/* Quick Setup Guide */}
              <div className="p-4 bg-blue-900/20 border border-blue-800 rounded-lg">
                <h4 className="text-red-400 font-medium mb-3">📱 Quick Setup for IPTV Players</h4>
                <div className="space-y-3 text-sm text-slate-300">
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
                      <code className="flex-1 text-xs bg-[#2a2a2a] px-2 py-1 rounded overflow-x-auto">
                        {`http://${settings.host || 'localhost'}:${settings.server_port || 8080}/get.php?username=${settings.xtream_username || 'streamarr'}&password=${settings.xtream_password || 'streamarr'}&type=m3u_plus&output=ts`}
                      </code>
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(`http://${settings.host || 'localhost'}:${settings.server_port || 8080}/get.php?username=${settings.xtream_username || 'streamarr'}&password=${settings.xtream_password || 'streamarr'}&type=m3u_plus&output=ts`);
                          setMessage('M3U URL copied to clipboard');
                          setTimeout(() => setMessage(''), 2000);
                        }}
                        className="px-2 py-1 bg-red-600 text-white text-xs rounded hover:bg-red-700"
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
                <p className="text-sm text-slate-300 mb-2">
                  EPG data is available for Live TV channels. Use this URL in your IPTV player:
                </p>
                <div className="flex gap-2">
                  <code className="flex-1 text-xs bg-[#2a2a2a] px-2 py-1 rounded overflow-x-auto text-slate-300">
                    {`http://${settings.host || 'localhost'}:${settings.server_port || 8080}/xmltv.php?username=${settings.xtream_username || 'streamarr'}&password=${settings.xtream_password || 'streamarr'}`}
                  </code>
                  <button
                    onClick={() => {
                      navigator.clipboard.writeText(`http://${settings.host || 'localhost'}:${settings.server_port || 8080}/xmltv.php?username=${settings.xtream_username || 'streamarr'}&password=${settings.xtream_password || 'streamarr'}`);
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
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div>
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="enable_notifications"
                    checked={settings.enable_notifications || false}
                    onChange={(e) => updateSetting('enable_notifications', e.target.checked)}
                    className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                  />
                  <label htmlFor="enable_notifications" className="text-sm font-medium text-slate-300">
                    Enable Notifications
                  </label>
                </div>
                <p className="text-xs text-slate-500 mt-1 ml-6">Send alerts when new content is added or errors occur</p>
              </div>

              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">Discord</h3>
                <div>
                  <label className="block text-sm font-medium text-slate-300 mb-2">
                    Discord Webhook URL
                  </label>
                  <input
                    type="text"
                    value={settings.discord_webhook_url || ''}
                    onChange={(e) => updateSetting('discord_webhook_url', e.target.value)}
                    className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    placeholder="https://discord.com/api/webhooks/..."
                  />
                  <p className="text-xs text-slate-500 mt-1">
                    Create in Discord: Server Settings → Integrations → Webhooks → New Webhook
                  </p>
                </div>
              </div>

              <div className="pt-6 border-t border-white/10">
                <h3 className="text-md font-medium text-slate-300 mb-4">Telegram</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Telegram Bot Token
                    </label>
                    <input
                      type="text"
                      value={settings.telegram_bot_token || ''}
                      onChange={(e) => updateSetting('telegram_bot_token', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
                    />
                    <p className="text-xs text-slate-500 mt-1">
                      Get from @BotFather on Telegram
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Telegram Chat ID
                    </label>
                    <input
                      type="text"
                      value={settings.telegram_chat_id || ''}
                      onChange={(e) => updateSetting('telegram_chat_id', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="123456789"
                    />
                    <p className="text-xs text-slate-500 mt-1">
                      Your user ID or group chat ID. Get from @userinfobot
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Database Tab */}
        {activeTab === 'database' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              <div className="mb-4 p-4 bg-blue-900/30 border border-blue-800 rounded-lg">
                <h3 className="text-red-400 font-medium mb-2">🗄️ Database Management</h3>
                <p className="text-sm text-slate-300">
                  Manage your library database. Use these options to clear data and regenerate content.
                </p>
              </div>

              {/* Database Statistics */}
              <div className="p-4 bg-[#2a2a2a]/50 rounded-lg border border-white/10">
                <h3 className="text-lg font-medium text-white mb-4 flex items-center gap-2">
                  <Database className="h-5 w-5" /> Database Statistics
                </h3>
                <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
                  <div className="p-3 bg-gray-900 rounded-lg text-center">
                    <div className="text-2xl font-bold text-red-400">{dbStats?.movies || 0}</div>
                    <div className="text-xs text-slate-400">Movies</div>
                  </div>
                  <div className="p-3 bg-gray-900 rounded-lg text-center">
                    <div className="text-2xl font-bold text-purple-400">{dbStats?.series || 0}</div>
                    <div className="text-xs text-slate-400">Series</div>
                  </div>
                  <div className="p-3 bg-gray-900 rounded-lg text-center">
                    <div className="text-2xl font-bold text-green-400">{dbStats?.episodes || 0}</div>
                    <div className="text-xs text-slate-400">Episodes</div>
                  </div>
                  <div className="p-3 bg-gray-900 rounded-lg text-center">
                    <div className="text-2xl font-bold text-yellow-400">{dbStats?.streams || 0}</div>
                    <div className="text-xs text-slate-400">Streams</div>
                  </div>
                  <div className="p-3 bg-gray-900 rounded-lg text-center">
                    <div className="text-2xl font-bold text-cyan-400">{dbStats?.collections || 0}</div>
                    <div className="text-xs text-slate-400">Collections</div>
                  </div>
                </div>
                <button
                  onClick={fetchDbStats}
                  className="mt-4 flex items-center gap-2 px-3 py-1.5 text-sm bg-gray-700 text-slate-300 rounded hover:bg-gray-600"
                >
                  <RefreshCw className="h-4 w-4" /> Refresh Stats
                </button>
              </div>

              {/* Movies Section */}
              <div className="p-4 bg-blue-900/20 border border-blue-800 rounded-lg">
                <h3 className="text-lg font-medium text-white mb-4">🎬 Movies</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <button
                    onClick={() => showConfirmDialog('clear-movies', 'Clear All Movies', 'This will permanently delete ALL movies from your library. MDBList sync will repopulate them on next run. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-red-900/50 text-red-300 rounded-lg hover:bg-red-900/70 border border-red-700 disabled:opacity-50"
                  >
                    <Trash2 className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Clear All Movies</div>
                      <div className="text-xs text-red-400">Delete all movies from library</div>
                    </div>
                  </button>
                  <button
                    onClick={() => showConfirmDialog('reset-movie-status', 'Reset Movie Status', 'This will reset the search status and collection_checked flag for all movies, allowing them to be re-scanned. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-yellow-900/50 text-yellow-300 rounded-lg hover:bg-yellow-900/70 border border-yellow-700 disabled:opacity-50"
                  >
                    <RefreshCw className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Reset Movie Status</div>
                      <div className="text-xs text-yellow-400">Re-enable scanning for all movies</div>
                    </div>
                  </button>
                </div>
              </div>

              {/* Series Section */}
              <div className="p-4 bg-purple-900/20 border border-purple-800 rounded-lg">
                <h3 className="text-lg font-medium text-white mb-4">📺 Series</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <button
                    onClick={() => showConfirmDialog('clear-series', 'Clear All Series', 'This will permanently delete ALL series and their episodes from your library. MDBList sync will repopulate them on next run. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-red-900/50 text-red-300 rounded-lg hover:bg-red-900/70 border border-red-700 disabled:opacity-50"
                  >
                    <Trash2 className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Clear All Series</div>
                      <div className="text-xs text-red-400">Delete all series and episodes</div>
                    </div>
                  </button>
                  <button
                    onClick={() => showConfirmDialog('reset-series-status', 'Reset Series Status', 'This will reset the search status for all series, allowing them to be re-scanned. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-yellow-900/50 text-yellow-300 rounded-lg hover:bg-yellow-900/70 border border-yellow-700 disabled:opacity-50"
                  >
                    <RefreshCw className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Reset Series Status</div>
                      <div className="text-xs text-yellow-400">Re-enable scanning for all series</div>
                    </div>
                  </button>
                  <button
                    onClick={() => triggerService('episode_scan')}
                    disabled={triggeringService === 'episode_scan'}
                    className="flex items-center gap-2 px-4 py-3 bg-blue-900/50 text-blue-300 rounded-lg hover:bg-blue-900/70 border border-blue-700 disabled:opacity-50"
                  >
                    <Download className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Scan Episodes</div>
                      <div className="text-xs text-red-400">Fetch episode metadata from TMDB</div>
                    </div>
                  </button>
                </div>
              </div>

              {/* Streams Section */}
              <div className="p-4 bg-yellow-900/20 border border-yellow-800 rounded-lg">
                <h3 className="text-lg font-medium text-white mb-4">🔗 Streams</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <button
                    onClick={() => showConfirmDialog('clear-streams', 'Clear All Streams', 'This will delete ALL cached streams. They will be fetched again when content is played. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-red-900/50 text-red-300 rounded-lg hover:bg-red-900/70 border border-red-700 disabled:opacity-50"
                  >
                    <Trash2 className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Clear All Streams</div>
                      <div className="text-xs text-red-400">Delete all cached stream URLs</div>
                    </div>
                  </button>
                  <button
                    onClick={() => showConfirmDialog('clear-stale-streams', 'Clear Stale Streams', 'This will delete streams older than 7 days. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-orange-900/50 text-orange-300 rounded-lg hover:bg-orange-900/70 border border-orange-700 disabled:opacity-50"
                  >
                    <Clock className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Clear Stale Streams</div>
                      <div className="text-xs text-orange-400">Delete streams older than 7 days</div>
                    </div>
                  </button>
                </div>
              </div>

              {/* Collections Section */}
              <div className="p-4 bg-cyan-900/20 border border-cyan-800 rounded-lg">
                <h3 className="text-lg font-medium text-white mb-4">📦 Collections</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <button
                    onClick={() => showConfirmDialog('clear-collections', 'Clear All Collections', 'This will delete ALL collections. Collection sync will repopulate them on next run. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-red-900/50 text-red-300 rounded-lg hover:bg-red-900/70 border border-red-700 disabled:opacity-50"
                  >
                    <Trash2 className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Clear All Collections</div>
                      <div className="text-xs text-red-400">Delete all movie collections</div>
                    </div>
                  </button>
                  <button
                    onClick={() => showConfirmDialog('resync-collections', 'Re-sync Collections', 'This will clear all collections and trigger a full re-sync. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-cyan-900/50 text-cyan-300 rounded-lg hover:bg-cyan-900/70 border border-cyan-700 disabled:opacity-50"
                  >
                    <RefreshCw className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Re-sync Collections</div>
                      <div className="text-xs text-cyan-400">Clear and rebuild collection data</div>
                    </div>
                  </button>
                </div>
              </div>

              {/* Live TV Section */}
              <div className="p-4 bg-green-900/20 border border-green-800 rounded-lg">
                <h3 className="text-lg font-medium text-white mb-4">📡 Live TV</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <button
                    onClick={() => showConfirmDialog('reload-livetv', 'Reload Live TV Channels', 'This will reload all M3U sources and refresh the channel list. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-green-900/50 text-green-300 rounded-lg hover:bg-green-900/70 border border-green-700 disabled:opacity-50"
                  >
                    <RefreshCw className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Reload Channels</div>
                      <div className="text-xs text-green-400">Refresh M3U sources and EPG</div>
                    </div>
                  </button>
                  <button
                    onClick={() => showConfirmDialog('clear-epg', 'Clear EPG Cache', 'This will clear the EPG program guide cache. It will be refreshed automatically. Are you sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-orange-900/50 text-orange-300 rounded-lg hover:bg-orange-900/70 border border-orange-700 disabled:opacity-50"
                  >
                    <Trash2 className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Clear EPG Cache</div>
                      <div className="text-xs text-orange-400">Delete program guide cache</div>
                    </div>
                  </button>
                </div>
              </div>

              {/* Danger Zone */}
              <div className="p-4 bg-red-900/30 border-2 border-red-700 rounded-lg">
                <h3 className="text-lg font-medium text-red-400 mb-2 flex items-center gap-2">
                  <AlertTriangle className="h-5 w-5" /> Danger Zone
                </h3>
                <p className="text-sm text-slate-400 mb-4">These actions are destructive and cannot be undone.</p>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <button
                    onClick={() => showConfirmDialog('clear-all-vod', 'Clear All VOD Content', 'This will delete ALL movies, series, episodes, streams and collections. This cannot be undone! Are you absolutely sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-red-800 text-white rounded-lg hover:bg-red-700 border border-red-600 disabled:opacity-50"
                  >
                    <Trash2 className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Clear All VOD Content</div>
                      <div className="text-xs text-red-200">Delete everything except Live TV</div>
                    </div>
                  </button>
                  <button
                    onClick={() => showConfirmDialog('factory-reset', 'Factory Reset Database', 'This will completely reset the database to its initial state. ALL data will be lost! Are you absolutely sure?')}
                    disabled={dbOperation !== null}
                    className="flex items-center gap-2 px-4 py-3 bg-red-900 text-white rounded-lg hover:bg-red-800 border border-red-500 disabled:opacity-50"
                  >
                    <AlertTriangle className="h-5 w-5" />
                    <div className="text-left">
                      <div className="font-medium">Factory Reset</div>
                      <div className="text-xs text-red-200">Reset everything to default</div>
                    </div>
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Blacklist Tab */}
        {activeTab === 'blacklist' && (
          <div className="space-y-6">
            <div className="bg-[#2a2a2a]/50 border border-white/10 rounded-lg p-6">
              <div className="flex items-center justify-between mb-6">
                <div>
                  <h3 className="text-red-400 font-medium mb-2 flex items-center gap-2">
                    <Trash2 className="h-5 w-5" />
                    Blacklisted Items
                  </h3>
                  <p className="text-sm text-slate-400">
                    Items removed from your library are blacklisted to prevent re-importing. You can remove them from the blacklist here to allow re-importing.
                  </p>
                </div>
                {blacklist.length > 0 && (
                  <button
                    onClick={clearBlacklist}
                    disabled={loadingBlacklist}
                    className="px-4 py-2 bg-red-600/20 text-red-400 border border-red-600/50 rounded-lg hover:bg-red-600/30 disabled:opacity-50 transition-colors flex items-center gap-2"
                  >
                    <Trash2 className="h-4 w-4" />
                    Clear All
                  </button>
                )}
              </div>

              {loadingBlacklist ? (
                <div className="flex items-center justify-center py-12">
                  <Loader className="h-8 w-8 animate-spin text-red-600" />
                </div>
              ) : blacklist.length === 0 ? (
                <div className="text-center py-12">
                  <Trash2 className="h-16 w-16 mx-auto mb-4 text-slate-600" />
                  <p className="text-slate-400 text-lg">No blacklisted items</p>
                  <p className="text-slate-500 text-sm mt-2">Items you remove from your library will appear here</p>
                </div>
              ) : (
                <div className="space-y-2">
                  {blacklist.map((item) => (
                    <div
                      key={item.id}
                      className="bg-[#1e1e1e] border border-white/10 rounded-lg p-4 hover:border-white/20 transition-colors"
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex-1">
                          <div className="flex items-center gap-3 mb-2">
                            <h4 className="text-white font-medium">{item.title}</h4>
                            <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                              item.type === 'movie' ? 'bg-purple-600/20 text-purple-400 border border-purple-600/50' : 'bg-green-600/20 text-green-400 border border-green-600/50'
                            }`}>
                              {item.type === 'movie' ? 'Movie' : 'Series'}
                            </span>
                          </div>
                          <div className="text-sm text-slate-400 space-y-1">
                            <p>
                              <span className="text-slate-500">Reason:</span> {item.reason || 'No reason provided'}
                            </p>
                            <p>
                              <span className="text-slate-500">TMDB ID:</span> {item.tmdb_id}
                            </p>
                            <p>
                              <span className="text-slate-500">Blacklisted:</span> {new Date(item.created_at).toLocaleString()}
                            </p>
                          </div>
                        </div>
                        <button
                          onClick={() => removeFromBlacklist(item.id)}
                          disabled={removingFromBlacklist === item.id}
                          className="px-4 py-2 bg-blue-600/20 text-blue-400 border border-blue-600/50 rounded-lg hover:bg-blue-600/30 disabled:opacity-50 transition-colors flex items-center gap-2 whitespace-nowrap"
                        >
                          {removingFromBlacklist === item.id ? (
                            <>
                              <Loader className="h-4 w-4 animate-spin" />
                              Removing...
                            </>
                          ) : (
                            <>
                              <RefreshCw className="h-4 w-4" />
                              Allow Re-import
                            </>
                          )}
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {/* About Tab */}
        {activeTab === 'about' && (
          <div className="space-y-6">
            <div className="bg-[#2a2a2a]/50 border border-white/10 rounded-lg p-6">
              <div className="bg-blue-900/30 border border-blue-700 rounded-lg p-4 mb-6">
                <h3 className="text-red-400 font-medium mb-2">ℹ️ About StreamArr Pro</h3>
                <p className="text-sm text-blue-200">
                  Self-hosted media server for Live TV, Movies & Series with Xtream Codes & M3U8 support.
                </p>
              </div>

              {/* Advanced Settings */}
              <div className="bg-gray-900 border border-white/10 rounded-lg p-4 mb-6">
                <h4 className="text-slate-300 font-medium mb-4 flex items-center gap-2">
                  <Code className="h-5 w-5" /> Advanced Settings
                </h4>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Server Port
                    </label>
                    <input
                      type="number"
                      value={settings.server_port || 8080}
                      onChange={(e) => updateSetting('server_port', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="8080"
                    />
                    <p className="text-xs text-slate-500 mt-1">Port the API server listens on. Requires restart.</p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Host Binding
                    </label>
                    <input
                      type="text"
                      value={settings.host || '0.0.0.0'}
                      onChange={(e) => updateSetting('host', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      placeholder="0.0.0.0"
                    />
                    <p className="text-xs text-slate-500 mt-1">0.0.0.0 = all interfaces, 127.0.0.1 = localhost only</p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Auto Cache Interval (hours)
                    </label>
                    <input
                      type="number"
                      value={settings.auto_cache_interval_hours || 6}
                      onChange={(e) => updateSetting('auto_cache_interval_hours', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                      min="1"
                      max="168"
                    />
                    <p className="text-xs text-slate-500 mt-1">How often to refresh library metadata and sync MDBLists (1-168 hours)</p>
                  </div>

                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="debug"
                      checked={settings.debug || false}
                      onChange={(e) => updateSetting('debug', e.target.checked)}
                      className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                    />
                    <label htmlFor="debug" className="text-sm text-slate-300">
                      Enable Debug Mode
                    </label>
                  </div>
                  <p className="text-xs text-slate-500 ml-6 -mt-2">Verbose logging for troubleshooting</p>
                </div>
              </div>

              {/* Version Info */}
              <div className="bg-gray-900 rounded-lg p-4 mb-6">
                <h4 className="text-slate-300 font-medium mb-4 flex items-center gap-2">
                  <Info className="h-5 w-5" /> Version Information
                </h4>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="bg-[#2a2a2a] rounded-lg p-4">
                    <div className="text-sm text-slate-400 mb-1">Current Version</div>
                    <div className="text-xl font-mono text-white">
                      {versionInfo?.current_version || 'Loading...'}
                    </div>
                    {versionInfo?.current_commit && (
                      <div className="text-xs text-slate-500 mt-1">
                        Commit: {versionInfo.current_commit.substring(0, 7)}
                      </div>
                    )}
                    {versionInfo?.build_date && (
                      <div className="text-xs text-slate-500">
                        Built: {new Date(versionInfo.build_date).toLocaleDateString()}
                      </div>
                    )}
                  </div>
                  <div className="bg-[#2a2a2a] rounded-lg p-4">
                    <div className="text-sm text-slate-400 mb-1">
                      Latest Version 
                      {versionInfo?.update_branch && (
                        <span className="ml-2 text-xs bg-gray-700 px-2 py-0.5 rounded">
                          {versionInfo.update_branch}
                        </span>
                      )}
                    </div>
                    <div className="text-xl font-mono text-white flex items-center gap-2">
                      {versionInfo?.latest_version || 'Check for updates'}
                      {versionInfo?.update_available && (
                        <span className="text-xs bg-green-600 text-white px-2 py-0.5 rounded">NEW</span>
                      )}
                    </div>
                    {versionInfo?.latest_commit && (
                      <div className="text-xs text-slate-500 mt-1">
                        Commit: {versionInfo.latest_commit.substring(0, 7)}
                      </div>
                    )}
                    {versionInfo?.latest_date && (
                      <div className="text-xs text-slate-500">
                        Released: {new Date(versionInfo.latest_date).toLocaleDateString()}
                      </div>
                    )}
                  </div>
                </div>
              </div>

              {/* Update Status */}
              {versionInfo?.update_available && (
                <div className="bg-green-900/30 border border-green-700 rounded-lg p-4 mb-6">
                  <div className="flex items-center gap-3">
                    <CheckCircle className="h-6 w-6 text-green-400" />
                    <div>
                      <div className="text-green-300 font-medium">Update Available!</div>
                      <div className="text-sm text-green-200">
                        A new version ({versionInfo.latest_version}) is available. 
                      </div>
                    </div>
                  </div>
                  {versionInfo.changelog && (
                    <div className="mt-3 text-sm text-slate-300 bg-gray-900/50 p-3 rounded">
                      <div className="font-medium mb-1">What's New:</div>
                      <div className="whitespace-pre-wrap">{versionInfo.changelog}</div>
                    </div>
                  )}
                </div>
              )}

              {/* Actions */}
              <div className="bg-gray-900 rounded-lg p-4 mb-6">
                <h4 className="text-slate-300 font-medium mb-4 flex items-center gap-2">
                  <Download className="h-5 w-5" /> Updates
                </h4>
                
                {/* Update Branch Selector */}
                <div className="mb-4">
                  <label className="block text-sm text-slate-400 mb-2">Update Branch</label>
                  <div className="flex items-center gap-3">
                    <div className="bg-[#2a2a2a] border border-white/10 rounded-lg px-3 py-2 text-white w-40">
                      main (Stable)
                    </div>
                    <span className="text-xs text-slate-500">
                      Using stable release branch
                    </span>
                  </div>
                </div>

                <div className="flex flex-wrap gap-3">
                  <button
                    onClick={checkForUpdates}
                    disabled={checkingUpdate || installingUpdate}
                    className="flex items-center gap-2 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50"
                  >
                    {checkingUpdate ? (
                      <>
                        <RefreshCw className="h-4 w-4 animate-spin" /> Checking...
                      </>
                    ) : (
                      <>
                        <RefreshCw className="h-4 w-4" /> Check for Updates
                      </>
                    )}
                  </button>
                  {versionInfo?.update_available && (
                    <>
                      <button
                        onClick={installUpdate}
                        disabled={installingUpdate}
                        className="flex items-center gap-2 px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50"
                      >
                        {installingUpdate ? (
                          <>
                            <RefreshCw className="h-4 w-4 animate-spin" /> Installing...
                          </>
                        ) : (
                          <>
                            <Download className="h-4 w-4" /> Install Update
                          </>
                        )}
                      </button>
                      <a
                        href="https://github.com/Zerr0-C00L/StreamArr/releases"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-2 px-4 py-2 bg-gray-700 text-white rounded-lg hover:bg-gray-600"
                      >
                        <ExternalLink className="h-4 w-4" /> View on GitHub
                      </a>
                    </>
                  )}
                </div>
                <p className="text-xs text-slate-500 mt-3">
                  {installingUpdate 
                    ? 'Update in progress. The server will restart automatically...'
                    : 'Checking updates from "main" branch (stable releases).'}
                </p>
              </div>

              {/* Links */}
              <div className="bg-gray-900 rounded-lg p-4 mb-6">
                <h4 className="text-slate-300 font-medium mb-4 flex items-center gap-2">
                  <ExternalLink className="h-5 w-5" /> Links
                </h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
                  <a
                    href="https://github.com/Zerr0-C00L/StreamArr"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 px-4 py-3 bg-[#2a2a2a] text-slate-300 rounded-lg hover:bg-gray-700 hover:text-white"
                  >
                    <Github className="h-5 w-5" /> GitHub Repository
                  </a>
                  <a
                    href="https://github.com/Zerr0-C00L/StreamArr/issues"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 px-4 py-3 bg-[#2a2a2a] text-slate-300 rounded-lg hover:bg-gray-700 hover:text-white"
                  >
                    <AlertCircle className="h-5 w-5" /> Report Issue
                  </a>
                  <a
                    href="https://github.com/Zerr0-C00L/StreamArr/discussions"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 px-4 py-3 bg-[#2a2a2a] text-slate-300 rounded-lg hover:bg-gray-700 hover:text-white"
                  >
                    <Info className="h-5 w-5" /> Discussions
                  </a>
                  <a
                    href="https://ko-fi.com/zeroq"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 px-4 py-3 bg-pink-800/50 text-pink-300 rounded-lg hover:bg-pink-700/50 hover:text-white"
                  >
                    ☕ Support on Ko-fi
                  </a>
                </div>
              </div>

              {/* Credits */}
              <div className="bg-gray-900 rounded-lg p-4">
                <h4 className="text-slate-300 font-medium mb-3">Credits</h4>
                <div className="text-sm text-slate-400 space-y-1">
                  <div>• Movie & TV data provided by <a href="https://www.themoviedb.org" target="_blank" rel="noopener noreferrer" className="text-red-400 hover:underline">TMDB</a></div>
                  <div>• Streaming via <a href="https://real-debrid.com" target="_blank" rel="noopener noreferrer" className="text-red-400 hover:underline">Real-Debrid</a>, Torrentio, Comet, MediaFusion</div>
                  <div>• Live TV channels from various free sources</div>
                </div>
                <div className="text-xs text-slate-500 mt-4 pt-4 border-t border-white/10">
                  StreamArr is open source software licensed under MIT. Use responsibly.
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Confirmation Dialog */}
        {confirmDialog && (
          <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50">
            <div className="bg-gray-900 border border-white/10 rounded-lg p-6 max-w-md mx-4">
              <div className="flex items-center gap-3 mb-4">
                <AlertTriangle className="h-8 w-8 text-yellow-500" />
                <h3 className="text-xl font-bold text-white">{confirmDialog.title}</h3>
              </div>
              <p className="text-slate-300 mb-6">{confirmDialog.message}</p>
              <div className="flex gap-3 justify-end">
                <button
                  onClick={() => setConfirmDialog(null)}
                  className="px-4 py-2 bg-gray-700 text-slate-300 rounded-lg hover:bg-gray-600"
                >
                  Cancel
                </button>
                <button
                  onClick={() => executeDbAction(confirmDialog.action)}
                  disabled={dbOperation !== null}
                  className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 flex items-center gap-2"
                >
                  {dbOperation === confirmDialog.action ? (
                    <>
                      <RefreshCw className="h-4 w-4 animate-spin" /> Processing...
                    </>
                  ) : (
                    'Confirm'
                  )}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
