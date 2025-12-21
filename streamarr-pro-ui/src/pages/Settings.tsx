import { useState, useEffect, useRef } from 'react';
import { Save, Layers, Settings as SettingsIcon, Code, Plus, X, Tv, Activity, Play, Clock, RefreshCw, Filter, Database, Trash2, Info, Github, Download, ExternalLink, CheckCircle, AlertCircle, Film, User, Camera, Loader, Search } from 'lucide-react';
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
  only_released_content: boolean;
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

type TabType = 'account' | 'integrations' | 'content' | 'livetv' | 'services' | 'system';

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
  
  // Dropdown option sets
  // Set default for enable_release_filters to true if not set
  useEffect(() => {
    if (settings && settings.enable_release_filters === undefined) {
      autoSaveSetting('enable_release_filters', true);
    }
  }, [settings]);

  const languageOptions = [
    'english', 'russian', 'italian', 'spanish', 'german', 'french', 'hindi', 'turkish', 'portuguese',
    'polish', 'dutch', 'thai', 'vietnamese', 'indonesian', 'arabic', 'chinese', 'korean', 'japanese'
  ];
  const countryOptions = [
    { code: 'US', name: 'United States' },
    { code: 'GB', name: 'United Kingdom' },
    { code: 'CA', name: 'Canada' },
    { code: 'AU', name: 'Australia' },
    { code: 'NZ', name: 'New Zealand' },
    { code: 'IN', name: 'India' },
    { code: 'TR', name: 'Turkey' },
    { code: 'RU', name: 'Russia' },
    { code: 'DE', name: 'Germany' },
    { code: 'FR', name: 'France' },
    { code: 'IT', name: 'Italy' },
    { code: 'ES', name: 'Spain' },
    { code: 'PT', name: 'Portugal' },
    { code: 'BR', name: 'Brazil' },
    { code: 'AR', name: 'Argentina' },
    { code: 'MX', name: 'Mexico' },
    { code: 'JP', name: 'Japan' },
    { code: 'KR', name: 'South Korea' },
    { code: 'CN', name: 'China' },
    { code: 'HK', name: 'Hong Kong' },
    { code: 'TW', name: 'Taiwan' },
    { code: 'NL', name: 'Netherlands' },
    { code: 'PL', name: 'Poland' },
    { code: 'SE', name: 'Sweden' },
    { code: 'NO', name: 'Norway' },
    { code: 'DK', name: 'Denmark' },
    { code: 'FI', name: 'Finland' },
    { code: 'RO', name: 'Romania' },
    { code: 'HU', name: 'Hungary' },
    { code: 'RS', name: 'Serbia' },
    { code: 'HR', name: 'Croatia' },
    { code: 'BA', name: 'Bosnia & Herzegovina' },
    { code: 'SI', name: 'Slovenia' },
    { code: 'ME', name: 'Montenegro' }
  ];

  // State - UI
  const [message, setMessage] = useState('');
  const [activeTab, setActiveTab] = useState<TabType>('account');
  const [profileMessage, setProfileMessage] = useState('');

  // State - MDBList
  const [newListUrl, setNewListUrl] = useState('');
  const [mdbLists, setMdbLists] = useState<MDBListEntry[]>([]);
  const [userLists, setUserLists] = useState<Array<{id: number; name: string; slug: string; items: number; user_name?: string}>>([]);
  const [mdbUsername, setMdbUsername] = useState('');
  const [fetchingUserLists, setFetchingUserLists] = useState(false);

  // State - M3U
  const [m3uSources, setM3uSources] = useState<M3USource[]>([]);
  const [newM3uName, setNewM3uName] = useState('');
  const [newM3uUrl, setNewM3uUrl] = useState('');
  const [newM3uEpg, setNewM3uEpg] = useState('');
  const [newM3uCategories, setNewM3uCategories] = useState<string[]>([]);
  const [availableCategories, setAvailableCategories] = useState<Array<{name: string; count: number}>>([]);
  const [loadingCategories, setLoadingCategories] = useState(false);
  const [showCategoryModal, setShowCategoryModal] = useState(false);

  // State - Xtream
  const [xtreamSources, setXtreamSources] = useState<XtreamSource[]>([]);
  const [newXtreamName, setNewXtreamName] = useState('');
  const [newXtreamUrl, setNewXtreamUrl] = useState('');
  const [newXtreamUsername, setNewXtreamUsername] = useState('');
  const [newXtreamPassword, setNewXtreamPassword] = useState('');
  const [newXtreamCategories, setNewXtreamCategories] = useState<string[]>([]);
  const [availableXtreamCategories, setAvailableXtreamCategories] = useState<Array<{name: string; count: number}>>([]);
  const [loadingXtreamCategories, setLoadingXtreamCategories] = useState(false);
  const [showXtreamCategoryModal, setShowXtreamCategoryModal] = useState(false);

  // State - Blacklist
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

  // State - Account
  const [profileUsername, setProfileUsername] = useState('');
  const [profileEmail, setProfileEmail] = useState('');
  const [profileAvatar, setProfileAvatar] = useState<string | null>(null);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  // State - Version
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null);
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [installingUpdate, setInstallingUpdate] = useState(false);

  // State - Services/Database
  const [services, setServices] = useState<ServiceStatus[]>([]);
  const [triggeringService, setTriggeringService] = useState<string | null>(null);
  const [dbStats, setDbStats] = useState<any>(null);
  const [channelStats, setChannelStats] = useState<any>(null);
  const [enabledSources, setEnabledSources] = useState<Set<string>>(new Set());
  const [enabledCategories, setEnabledCategories] = useState<Set<string>>(new Set());
  const [sourceStatuses, setSourceStatuses] = useState<Map<string, SourceStatus>>(new Map());
  const [checkingAllSources, setCheckingAllSources] = useState(false);

  // Initialize
  useEffect(() => {
    fetchSettings();
    fetchChannelStats();
    fetchServices();
    fetchDbStats();
    fetchVersionInfo();
    fetchUserProfile();
    const savedAvatar = localStorage.getItem('profile_picture');
    if (savedAvatar) {
      setProfileAvatar(savedAvatar);
    }
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
        livetv_enabled_sources: Array.from(enabledSources),
        livetv_enabled_categories: Array.from(enabledCategories),
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

  useEffect(() => {
    if (activeTab === 'services') {
      fetchBlacklist();
    }
  }, [activeTab]);

  useEffect(() => {
    if (activeTab === 'services') {
      const interval = setInterval(() => {
        fetchServices();
      }, 5000);
      return () => clearInterval(interval);
    }
  }, [activeTab]);

  // Tab configuration
  const tabs = [
    { id: 'account' as TabType, label: 'Account', icon: User },
    { id: 'integrations' as TabType, label: 'Integrations', icon: Layers },
    { id: 'content' as TabType, label: 'Content', icon: Film },
    { id: 'livetv' as TabType, label: 'TV & IPTV', icon: Tv },
    { id: 'services' as TabType, label: 'Services', icon: Activity },
    { id: 'system' as TabType, label: 'System', icon: SettingsIcon },
  ];

  // ========== API Functions ==========

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
      
      if (data.livetv_enabled_sources && Array.isArray(data.livetv_enabled_sources)) {
        setEnabledSources(new Set(data.livetv_enabled_sources));
      } else {
        setEnabledSources(new Set());
      }
      if (data.livetv_enabled_categories && Array.isArray(data.livetv_enabled_categories)) {
        setEnabledCategories(new Set(data.livetv_enabled_categories));
      } else {
        setEnabledCategories(new Set());
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
      livetv_enabled_sources: Array.from(enabledSources),
      livetv_enabled_categories: Array.from(enabledCategories),
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

  // Update setting and auto-save
  const updateSetting = (key: keyof SettingsData, value: any) => {
    autoSaveSetting(key, value);
  };

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

  const fetchServices = async () => {
    try {
      const response = await api.get('/services');
      const data = response.data;
      const sortedServices = (data.services || []).sort((a: ServiceStatus, b: ServiceStatus) => a.name.localeCompare(b.name));
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

  const fetchChannelStats = async () => {
    try {
      const response = await api.get('/channels/stats');
      const data = response.data;
      setChannelStats(data);
    } catch (error) {
      console.error('Failed to fetch channel stats:', error);
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
      fetchBlacklist();
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
      fetchBlacklist();
    } catch (error: any) {
      setMessage(`❌ Failed to clear blacklist: ${error.response?.data?.error || error.message}`);
      setTimeout(() => setMessage(''), 5000);
    }
    setLoadingBlacklist(false);
  };

  const fetchVersionInfo = async () => {
    try {
      const response = await api.get('/version');
      setVersionInfo(response.data);
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
      setTimeout(() => {
        window.location.reload();
      }, 30000);
    } catch (error: any) {
      setMessage(`❌ Update failed: ${error.response?.data?.error || error.message}`);
      setInstallingUpdate(false);
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
    const username = list.user_name || mdbUsername;
    const url = `https://mdblist.com/lists/${username}/${list.slug}`;
    
    if (mdbLists.some(l => l.url.includes(list.slug))) {
      setMessage('⚠️ This list is already added');
      setTimeout(() => setMessage(''), 2000);
      return;
    }
    
    setMdbLists([...mdbLists, { url, name: list.name, enabled: true }]);
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

  // M3U helpers
  const previewM3UCategories = async () => {
    if (!newM3uUrl.trim()) {
      setMessage('❌ Please enter an M3U URL first');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
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
    
    if (!newM3uUrl.startsWith('http://') && !newM3uUrl.startsWith('https://')) {
      setMessage('❌ M3U URL must start with http:// or https://');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    if (newM3uEpg.trim() && !newM3uEpg.startsWith('http://') && !newM3uEpg.startsWith('https://')) {
      setMessage('❌ EPG URL must start with http:// or https://');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
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
    
    console.log('[DEBUG] newM3uCategories:', newM3uCategories);
    if (newM3uCategories.length > 0) {
      newSource.selected_categories = newM3uCategories;
    }
    console.log('[DEBUG] newSource:', newSource);
    
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

  // Xtream helpers
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
  const [newBalkanCategories, setNewBalkanCategories] = useState<string[]>([]);
  const [availableBalkanCategories, setAvailableBalkanCategories] = useState<Array<{name: string; count: number}>>([]);
  const [loadingBalkanCategories, setLoadingBalkanCategories] = useState(false);
  const [showBalkanCategoryModal, setShowBalkanCategoryModal] = useState(false);

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

  const addXtreamSource = () => {
    if (!newXtreamName.trim() || !newXtreamUrl.trim() || !newXtreamUsername.trim() || !newXtreamPassword.trim()) {
      setMessage('❌ Please fill in all fields for the Xtream source');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    if (!newXtreamUrl.startsWith('http://') && !newXtreamUrl.startsWith('https://')) {
      setMessage('❌ Server URL must start with http:// or https://');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    if (xtreamSources.some(s => s.server_url === newXtreamUrl || s.name === newXtreamName.trim())) {
      setMessage('⚠️ An Xtream source with this name or URL already exists');
      setTimeout(() => setMessage(''), 3000);
      return;
    }
    
    const newSource: XtreamSource = {
      name: newXtreamName.trim(),
      server_url: newXtreamUrl.trim().replace(/\/$/, ''),
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

  const checkSourceStatus = async (url: string) => {
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

  const checkAllSourcesStatus = async () => {
    setCheckingAllSources(true);
    for (const source of m3uSources) {
      await checkSourceStatus(source.url);
    }
    setCheckingAllSources(false);
  };

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

  // Helper functions
  // Removed unused confirmation dialog - using inline confirmations instead

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

  // Loading state
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

  return (
    <div className="min-h-screen bg-[#141414] -m-6 p-8">
      <div className="max-w-6xl mx-auto">
        {/* Header */}
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

        {/* ACCOUNT TAB */}
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

        {/* INTEGRATIONS TAB */}
        {activeTab === 'integrations' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              {/* API Keys Section */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">🔑 API Keys & Credentials</h3>
                
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      TMDB API Key <span className="text-red-400">*</span>
                    </label>
                    <input
                      type="text"
                      value={settings?.tmdb_api_key || ''}
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
                      value={settings?.realdebrid_api_key || ''}
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
                      value={settings?.premiumize_api_key || ''}
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
                      value={settings?.mdblist_api_key || ''}
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
                </div>
              </div>

              <hr className="border-white/10" />

              {/* Debrid Services */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">💎 Debrid Services</h3>
                <p className="text-sm text-slate-400 mb-4">Premium services that cache torrents for instant high-speed streaming</p>
                <div className="grid grid-cols-3 gap-3">
                  <div
                    className={`flex items-center gap-3 p-4 rounded-lg border cursor-pointer transition-colors ${
                      settings?.use_realdebrid
                        ? 'bg-green-900/30 border-green-700 hover:bg-green-900/50'
                        : 'bg-[#2a2a2a]/50 border-white/10 opacity-60 hover:opacity-80'
                    }`}
                    onClick={() => updateSetting('use_realdebrid', !settings?.use_realdebrid)}
                  >
                    <input
                      type="checkbox"
                      checked={settings?.use_realdebrid || false}
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
                      settings?.use_premiumize
                        ? 'bg-green-900/30 border-green-700 hover:bg-green-900/50'
                        : 'bg-[#2a2a2a]/50 border-white/10 opacity-60 hover:opacity-80'
                    }`}
                    onClick={() => updateSetting('use_premiumize', !settings?.use_premiumize)}
                  >
                    <input
                      type="checkbox"
                      checked={settings?.use_premiumize || false}
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

              {/* MDBList Auto-Import */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-lg font-medium text-slate-300">📋 MDBList Auto-Import Lists</h3>
                  <button
                    onClick={fetchUserMDBLists}
                    disabled={fetchingUserLists || !settings?.mdblist_api_key}
                    className="px-3 py-1.5 text-sm bg-gray-700 text-white rounded-lg hover:bg-gray-600 disabled:bg-[#2a2a2a] disabled:text-slate-500 disabled:cursor-not-allowed transition-colors"
                  >
                    {fetchingUserLists ? 'Loading...' : '📋 Fetch My Lists'}
                  </button>
                </div>
                <p className="text-sm text-slate-400 mb-4">
                  Automatically add movies/series from MDBList curated lists to your library. The worker periodically syncs these lists.
                </p>

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
                    Add
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
              </div>

              <hr className="border-white/10" />

              {/* Built-in Stremio Addon */}
              <div>
                <div className="mb-4 p-4 bg-purple-900/30 border border-purple-800 rounded-lg">
                  <h3 className="text-purple-400 font-medium mb-2 flex items-center gap-2">
                    <Play className="w-5 h-5" />
                    Stremio Addon (Built-in)
                  </h3>
                  <p className="text-sm text-slate-300">
                    Enable the built-in Stremio addon to stream your library directly in Stremio. Configure which catalogs to show and customize their names.
                  </p>
                  <p className="text-sm text-yellow-400 mt-2">
                    💡 Installation: In Stremio go to Add-ons → Community → Add-on Repository → Install from URL, then paste the Manifest URL from this page.
                  </p>
                </div>

                {/* Enable Addon */}
                <div className="mb-4">
                  <label className="flex items-center gap-3 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={settings?.stremio_addon?.enabled || false}
                      onChange={async (e) => {
                        if (!settings) return;
                        const currentAddon = settings.stremio_addon || {};
                        const newAddon = { ...currentAddon, enabled: e.target.checked };
                        updateSetting('stremio_addon', newAddon);
                        try {
                          const settingsToSave = { ...settings, stremio_addon: newAddon };
                          await api.put('/settings', settingsToSave);
                          setMessage(e.target.checked ? '✅ Stremio addon enabled' : '✅ Stremio addon disabled');
                          setTimeout(() => setMessage(''), 2000);
                        } catch (error) {
                          console.error('Failed to save:', error);
                          setMessage('❌ Failed to save setting');
                        }
                      }}
                      className="w-5 h-5 text-red-600 bg-gray-700 border-gray-600 rounded focus:ring-red-500"
                    />
                    <span className="text-white font-medium">Enable Stremio Addon</span>
                  </label>
                </div>

                {settings?.stremio_addon?.enabled && (
                  <>
                    {/* Addon Name */}
                    <div className="mb-4">
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
                    <div className="mb-4">
                      <label className="block text-sm text-slate-300 mb-2">Server Host/IP</label>
                      <input
                        type="text"
                        value={settings.stremio_addon?.public_server_url || ''}
                        onChange={(e) => updateSetting('stremio_addon', { ...settings.stremio_addon, public_server_url: e.target.value })}
                        placeholder="e.g., streamarr.mydomain.com:8080 or 123.45.67.89:8080"
                        className="w-full p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                      />
                      <p className="text-xs text-slate-500 mt-1">Your public domain or IP with port if needed. Leave empty to auto-detect.</p>
                    </div>

                    {/* Authentication Token */}
                    <div className="mb-4">
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
                              const newAddon = { ...currentAddon, enabled: true, shared_token: response.data.token };
                              updateSetting('stremio_addon', newAddon);
                              const settingsToSave = { ...settings, stremio_addon: newAddon };
                              await api.put('/settings', settingsToSave);
                              setMessage('✅ Token generated successfully');
                              setTimeout(() => setMessage(''), 2000);
                            } catch (error: any) {
                              console.error('Failed to generate token:', error);
                              const errorMsg = error?.response?.data?.error || error?.message || 'Failed to generate token';
                              setMessage('❌ ' + errorMsg);
                            }
                          }}
                          className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 whitespace-nowrap"
                        >
                          Generate Token
                        </button>
                      </div>
                      <p className="text-xs text-slate-500 mt-1">Secure token for addon authentication. Keep this private!</p>
                    </div>

                    {/* Catalog Configuration */}
                    <div className="mb-4">
                      <h4 className="text-white font-medium mb-3">Library Catalogs</h4>
                      <div className="space-y-3">
                        {(settings.stremio_addon?.catalogs || []).map((catalog: any, index: number) => (
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
                                <p className="text-xs text-slate-500 mt-1">Catalog ID: {catalog.id}</p>
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                      <p className="text-xs text-slate-500 mt-2">Enable/disable catalogs and customize their names as shown in Stremio</p>
                    </div>

                    {/* Manifest URL */}
                    {settings.stremio_addon?.shared_token && (
                      <div className="bg-[#2a2a2a] rounded-lg p-4 border border-green-800 mb-4">
                        <h4 className="text-green-400 font-medium mb-3 flex items-center gap-2">
                          <CheckCircle className="w-5 h-5" />
                          Addon Ready to Install
                        </h4>
                        <button
                          onClick={async () => {
                            try {
                              const response = await api.get('/stremio/manifest-url');
                              const url = response.data.manifest_url;
                              navigator.clipboard.writeText(url);
                              setMessage('✅ Manifest URL copied! Paste it in Stremio');
                              setTimeout(() => setMessage(''), 3000);
                            } catch (error: any) {
                              console.error('Failed to get manifest URL:', error);
                              const errorMsg = error?.response?.data?.error || error?.message || 'Failed to get manifest URL';
                              setMessage('❌ ' + errorMsg);
                            }
                          }}
                          className="w-full px-4 py-3 bg-red-600 text-white rounded-lg hover:bg-red-700 font-medium"
                        >
                          Copy Manifest URL
                        </button>
                      </div>
                    )}
                  </>
                )}
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
                      const currentAddons = settings?.stremio_addons || [];
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
                    <span className="font-medium">Popular addons:</span> Torrentio, MediaFusion, Autostream, Sootio, Stremthru
                  </div>
                </div>
                
                <div className="space-y-3">
                  {(settings?.stremio_addons || []).map((addon: any, index: number) => (
                    <div key={index} className="bg-[#2a2a2a]/50 border border-white/10 rounded-lg p-4">
                      <div className="flex items-start gap-3">
                        {/* Enable/Disable Toggle */}
                        <div className="flex items-center pt-2">
                          <input
                            type="checkbox"
                            checked={addon.enabled}
                            onChange={(e) => {
                              const newAddons = [...(settings?.stremio_addons || [])];
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
                                  const newAddons = [...(settings?.stremio_addons || [])];
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
                                  const newAddons = [...(settings?.stremio_addons || [])];
                                  newAddons[index] = { ...addon, url: e.target.value };
                                  updateSetting('stremio_addons', newAddons);
                                }}
                                className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white"
                                placeholder="https://addon.example.com"
                              />
                            </div>
                          </div>
                          <div className="flex items-center gap-2 text-xs text-slate-400">
                            {addon.enabled ? <span className="text-green-400">✓ Enabled</span> : <span className="text-slate-500">Disabled</span>}
                          </div>
                        </div>
                        
                        {/* Delete Button */}
                        <button
                          onClick={() => {
                            const newAddons = (settings?.stremio_addons || []).filter((_: any, i: number) => i !== index);
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
                  
                  {(!settings?.stremio_addons || settings.stremio_addons.length === 0) && (
                    <div className="text-center py-8 text-slate-400">
                      <Layers className="w-12 h-12 mx-auto mb-2 opacity-50" />
                      <p>No addons configured. Click "Add Addon" to get started.</p>
                      <p className="text-xs mt-2">Popular addons: Torrentio, MediaFusion</p>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* CONTENT TAB */}
        {activeTab === 'content' && (
          <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
            <div className="space-y-6">
              {/* Playback Section */}
              <div>
                <h3 className="text-lg font-semibold text-white mb-4">🎞️ Playback</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Maximum Resolution
                    </label>
                    <select
                      value={settings?.max_resolution || 1080}
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
                    <select
                      value={settings?.max_file_size || 0}
                      onChange={(e) => updateSetting('max_file_size', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      {[0,700,1000,1500,2000,3000,5000,10000,12000,20000].map((mb) => (
                        <option key={mb} value={mb}>{mb === 0 ? 'Unlimited' : `${mb} MB`}</option>
                      ))}
                    </select>
                    <p className="text-xs text-slate-500 mt-1">Skip files larger than this. 0 = no limit. Useful for slow connections.</p>
                  </div>

                  <div>
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id="quality_variants"
                        checked={settings?.enable_quality_variants || false}
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
                        checked={settings?.show_full_stream_name || false}
                        onChange={(e) => updateSetting('show_full_stream_name', e.target.checked)}
                        className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                      />
                      <label htmlFor="full_stream_name" className="text-sm text-slate-300">
                        Show Full Stream Names
                      </label>
                    </div>
                    <p className="text-xs text-slate-500 mt-1 ml-6">Display detailed stream info (codec, size, etc.) in player.</p>
                  </div>
                </div>
              </div>

              <hr className="border-white/10" />

              {/* Sorting Section */}
              <div>
                <h3 className="text-lg font-semibold text-white mb-4">↕️ Sorting</h3>
                <p className="text-sm text-slate-400 mb-4">Configure how streams are sorted and which one is selected for playback.</p>
                <div className="space-y-4">

                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Sort Priority Order
                    </label>
                    <select
                      value={settings?.stream_sort_order || 'quality,size,seeders'}
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
                      value={settings?.stream_sort_prefer || 'best'}
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

              <hr className="border-white/10" />

              {/* Localization */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">🌍 Localization</h3>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">Preferred Language</label>
                    <select
                      value={settings?.language || 'english'}
                      onChange={(e) => updateSetting('language', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      {languageOptions.map((lang) => (
                        <option key={lang} value={lang}>{lang}</option>
                      ))}
                    </select>
                    <p className="text-xs text-slate-500 mt-1">Used when fetching content metadata and stream preferences.</p>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">Movies Origin Country</label>
                    <select
                      value={settings?.movies_origin_country || 'US'}
                      onChange={(e) => updateSetting('movies_origin_country', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      {countryOptions.map((c) => (
                        <option key={c.code} value={c.code}>{c.name}</option>
                      ))}
                    </select>
                    <p className="text-xs text-slate-500 mt-1">Bias discovery for domestic releases from this country.</p>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">Series Origin Country</label>
                    <select
                      value={settings?.series_origin_country || 'US'}
                      onChange={(e) => updateSetting('series_origin_country', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      {countryOptions.map((c) => (
                        <option key={c.code} value={c.code}>{c.name}</option>
                      ))}
                    </select>
                    <p className="text-xs text-slate-500 mt-1">Bias discovery for TV shows from this country.</p>
                  </div>
                </div>
              </div>

              <hr className="border-white/10" />

              {/* Collections */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">📦 Collections</h3>
                <div>
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="auto_add_collections"
                      checked={settings?.auto_add_collections || false}
                      onChange={(e) => updateSetting('auto_add_collections', e.target.checked)}
                      className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                    />
                    <label htmlFor="auto_add_collections" className="text-sm text-slate-300">
                      Add Entire Collection
                    </label>
                  </div>
                  <p className="text-xs text-slate-500 mt-1 ml-6">When adding a movie that belongs to a collection (e.g., The Dark Knight Trilogy), automatically add all other movies from that collection.</p>
                </div>
              </div>

              <hr className="border-white/10" />

              {/* Content Filters */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">🔍 Content Filters</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Minimum Year
                    </label>
                    <select
                      value={settings?.min_year || 1900}
                      onChange={(e) => updateSetting('min_year', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      {[1900,1950,1970,1980,1990,2000,2005,2010,2015,2020,2022,2024].map((y) => (
                        <option key={y} value={y}>{y}+</option>
                      ))}
                    </select>
                    <p className="text-xs text-slate-500 mt-1">Exclude movies/series released before this year.</p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">
                      Minimum Runtime (minutes)
                    </label>
                    <select
                      value={settings?.min_runtime || 0}
                      onChange={(e) => updateSetting('min_runtime', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      {[0,30,45,60,75,90,120].map((m) => (
                        <option key={m} value={m}>{m} minutes</option>
                      ))}
                    </select>
                    <p className="text-xs text-slate-500 mt-1">Exclude short content (trailers, clips). 60+ recommended for movies.</p>
                  </div>

                  <div>
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id="hide_unavailable"
                        checked={settings?.hide_unavailable_content || false}
                        onChange={(e) => updateSetting('hide_unavailable_content', e.target.checked)}
                        className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                      />
                      <label htmlFor="hide_unavailable" className="text-sm font-medium text-slate-300">
                        Hide Content Without Streams
                      </label>
                    </div>
                    <p className="text-xs text-slate-500 mt-1 ml-6">
                      Only show movies and episodes in IPTV apps if they have at least one stream available.
                    </p>
                  </div>

                  <div className="p-3 bg-yellow-900/20 border border-yellow-800 rounded-lg">
                    <p className="text-xs text-yellow-400 font-medium mb-2">💡 How It Works:</p>
                    <ul className="text-xs text-slate-400 space-y-1">
                      <li>• The "Stream Search" service periodically checks for available streams</li>
                      <li>• Movies and episodes are marked as available or unavailable</li>
                      <li>• When enabled above, unavailable content is filtered from IPTV apps</li>
                      <li>• Streams are re-checked every 7 days for items without streams</li>
                    </ul>
                  </div>
                </div>
              </div>

              <hr className="border-white/10" />

              {/* Adult Content */}
              <div>
                <h3 className="text-lg font-medium text-white mb-4">👨‍🔞 Adult Content (18+)</h3>
                <div className="space-y-4">
                  <div>
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id="include_adult"
                        checked={settings?.include_adult_vod || false}
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

                  <div className="p-4 bg-red-900/10 border border-red-800 rounded-lg">
                    <div className="flex items-center gap-2 mb-2">
                      <input
                        type="checkbox"
                        id="import_adult_vod"
                        checked={settings?.import_adult_vod_from_github || false}
                        onChange={(e) => updateSetting('import_adult_vod_from_github', e.target.checked)}
                        className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                      />
                      <label htmlFor="import_adult_vod" className="text-sm font-medium text-slate-300">
                        Import Adult VOD from GitHub
                      </label>
                    </div>
                    <p className="text-xs text-slate-500 mt-1 ml-6 mb-3">
                      Enable importing adult VOD content from public-files GitHub repository. Separate from TMDB adult content.
                    </p>
                    {settings?.import_adult_vod_from_github && (
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

                  <div>
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id="only_released"
                        checked={settings?.only_released_content || false}
                        onChange={(e) => updateSetting('only_released_content', e.target.checked)}
                        className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                      />
                      <label htmlFor="only_released" className="text-sm font-medium text-slate-300">
                        Only Include Released Content in Playlist
                      </label>
                    </div>
                    <p className="text-xs text-slate-500 mt-1 ml-6">
                      Only include movies/series in the IPTV playlist that are already released. Unreleased items remain in your library but won't appear in the playlist.
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* TV & IPTV TAB */}
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
                      checked={settings?.include_live_tv || false}
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
                      value={settings?.iptv_import_mode || 'live_only'}
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
                      checked={settings?.duplicate_vod_per_provider || false}
                      onChange={(e) => {
                        const val = e.target.checked;
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
                    <select
                      id="iptv_vod_sync_interval_hours"
                      value={Number(settings?.iptv_vod_sync_interval_hours || 6)}
                      onChange={(e) => updateSetting('iptv_vod_sync_interval_hours', Number(e.target.value))}
                      className="w-40 p-2 bg-[#2a2a2a] border border-white/10 rounded text-white"
                    >
                      {[1,3,6,12,24,48,72,168].map(h => (
                        <option key={h} value={h}>{h} hours</option>
                      ))}
                    </select>
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
                      checked={settings?.balkan_vod_enabled || false}
                      onChange={(e) => updateSetting('balkan_vod_enabled', e.target.checked)}
                      className="sr-only peer"
                    />
                    <div className="w-11 h-6 bg-gray-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
                  </label>
                </div>

                {settings?.balkan_vod_enabled && (
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
                        ✅ {newXtreamCategories.length} categories selected
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
                                <span>🏷️</span> {source.selected_categories.length} categories: {source.selected_categories.slice(0, 3).join(', ')}
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

              <hr className="border-white/10" />

              {/* Xtream Codes API */}
              <div>
                <div className="mb-4 p-4 bg-purple-900/30 border border-purple-800 rounded-lg">
                  <h3 className="text-purple-400 font-medium mb-2">📡 Xtream Codes API</h3>
                  <p className="text-sm text-slate-300">
                    StreamArr exposes an Xtream Codes compatible API that can be used with IPTV players like TiviMate, XCIPTV, or OTT Navigator.
                  </p>
                </div>

                <div>
                  <h3 className="text-lg font-medium text-white mb-4">🔐 Xtream API Credentials</h3>
                  <div className="bg-[#2a2a2a] rounded-lg p-4 space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="block text-sm text-slate-300 mb-2">Xtream Username</label>
                        <input
                          type="text"
                          value={settings?.xtream_username || 'streamarr'}
                          onChange={(e) => updateSetting('xtream_username', e.target.value)}
                          className="w-full p-3 bg-gray-700 border border-gray-600 rounded-lg text-white"
                          placeholder="streamarr"
                        />
                      </div>
                      <div>
                        <label className="block text-sm text-slate-300 mb-2">Xtream Password</label>
                        <input
                          type="text"
                          value={settings?.xtream_password || 'streamarr'}
                          onChange={(e) => updateSetting('xtream_password', e.target.value)}
                          className="w-full p-3 bg-gray-700 border border-gray-600 rounded-lg text-white"
                          placeholder="streamarr"
                        />
                      </div>
                    </div>
                    <p className="text-xs text-slate-500">
                      Use these credentials in your IPTV player, not your web app password. Click "Save Changes" at the top after modifying.
                    </p>
                  </div>
                </div>

                <div className="mt-6">
                  <h3 className="text-lg font-medium text-white mb-4">🔗 Connection Details</h3>
                  <p className="text-sm text-slate-400 mb-4">
                    Use these credentials in your IPTV player, not your web app password. Click "Save Changes" at the top after modifying.
                  </p>
                  <div className="bg-[#2a2a2a] rounded-lg p-4 space-y-4">
                    <div>
                      <label className="block text-sm text-slate-400 mb-1">Public Server URL (for IPTV Players)</label>
                      <p className="text-xs text-slate-500 mb-2">Enter your server's public IP or domain name. This will be used in the connection details below.</p>
                      <input
                        type="text"
                        value={settings?.user_set_host || ''}
                        onChange={(e) => updateSetting('user_set_host', e.target.value)}
                        placeholder="e.g., 77.42.16.119 or mydomain.com"
                        className="w-full p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm focus:outline-none focus:border-red-500"
                      />
                    </div>

                    <div>
                      <label className="block text-sm text-slate-400 mb-1">Server URL</label>
                      <div className="flex gap-2">
                        <input
                          type="text"
                          value={`http://${settings?.user_set_host || settings?.host || window.location.hostname}:${settings?.server_port || 8080}`}
                          readOnly
                          className="flex-1 p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                        />
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(`http://${settings?.user_set_host || settings?.host || window.location.hostname}:${settings?.server_port || 8080}`);
                            setMessage('Server URL copied to clipboard');
                            setTimeout(() => setMessage(''), 2000);
                          }}
                          className="px-3 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
                        >
                          Copy
                        </button>
                      </div>
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="block text-sm text-slate-400 mb-1">Username</label>
                        <div className="flex gap-2">
                          <input
                            type="text"
                            value={settings?.xtream_username || 'streamarr'}
                            readOnly
                            className="flex-1 p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                          />
                          <button
                            onClick={() => {
                              navigator.clipboard.writeText(settings?.xtream_username || 'streamarr');
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
                            value={settings?.xtream_password || 'streamarr'}
                            readOnly
                            className="flex-1 p-3 bg-gray-700 border border-gray-600 rounded-lg text-white font-mono text-sm"
                          />
                          <button
                            onClick={() => {
                              navigator.clipboard.writeText(settings?.xtream_password || 'streamarr');
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

                <div className="mt-6 p-4 bg-blue-900/20 border border-blue-800 rounded-lg">
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
                          {`http://${settings?.user_set_host || settings?.host || window.location.hostname}:${settings?.server_port || 8080}/get.php?username=${settings?.xtream_username || 'streamarr'}&password=${settings?.xtream_password || 'streamarr'}&type=m3u_plus&output=ts`}
                        </code>
                        <button
                          onClick={() => {
                            navigator.clipboard.writeText(`http://${settings?.user_set_host || settings?.host || window.location.hostname}:${settings?.server_port || 8080}/get.php?username=${settings?.xtream_username || 'streamarr'}&password=${settings?.xtream_password || 'streamarr'}&type=m3u_plus&output=ts`);
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

                <div className="mt-6 p-4 bg-green-900/20 border border-green-800 rounded-lg">
                  <h4 className="text-green-400 font-medium mb-2">📺 EPG (Electronic Program Guide)</h4>
                  <p className="text-sm text-slate-300 mb-2">
                    EPG data is available for Live TV channels. Use this URL in your IPTV player:
                  </p>
                  <div className="flex gap-2">
                    <code className="flex-1 text-xs bg-[#2a2a2a] px-2 py-1 rounded overflow-x-auto text-slate-300">
                      {`http://${settings?.user_set_host || settings?.host || window.location.hostname}:${settings?.server_port || 8080}/xmltv.php?username=${settings?.xtream_username || 'streamarr'}&password=${settings?.xtream_password || 'streamarr'}`}
                    </code>
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(`http://${settings?.user_set_host || settings?.host || window.location.hostname}:${settings?.server_port || 8080}/xmltv.php?username=${settings?.xtream_username || 'streamarr'}&password=${settings?.xtream_password || 'streamarr'}`);
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

        {/* SERVICES TAB */}
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

              {/* Database Tab Content - Integrated */}
              <div className="pt-6 border-t border-white/10">
                <h2 className="text-2xl font-bold text-white mb-6">🗄️ Database Management</h2>
                
                <div className="p-4 bg-[#2a2a2a]/50 rounded-lg border border-white/10 mb-6">
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

                {/* Quick Database Actions */}
                <p className="text-slate-400 mb-4 text-sm">
                  📌 Database stats shown above. Use the main "Save Changes" button at the top to apply all settings changes before database operations.
                </p>
              </div>

              {/* Notifications Tab Content - Integrated */}
              <div className="pt-6 border-t border-white/10">
                <h2 className="text-2xl font-bold text-white mb-6">🔔 Notifications</h2>
                
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

              {/* Blacklist Tab Content - Integrated */}
              <div className="pt-6 border-t border-white/10">
                <h2 className="text-2xl font-bold text-white mb-6">🗑️ Blacklist</h2>
                
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
            </div>
          </div>
        )}

        {/* SYSTEM TAB */}
        {activeTab === 'system' && (
          <div className="space-y-6">
            <div className="bg-[#1e1e1e] rounded-xl p-6 border border-white/10">
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
                      value={settings?.server_port || 8080}
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
                      value={settings?.host || '0.0.0.0'}
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
                    <select
                      value={settings?.auto_cache_interval_hours || 6}
                      onChange={(e) => updateSetting('auto_cache_interval_hours', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      {[1, 3, 6, 12, 24, 48, 72, 168].map((h) => (
                        <option key={h} value={h}>{h} hours</option>
                      ))}
                    </select>
                    <p className="text-xs text-slate-500 mt-1">How often to refresh library metadata and sync MDBLists</p>
                  </div>

                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="debug"
                      checked={settings?.debug || false}
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

              {/* Proxy Settings */}
              <div className="bg-gray-900 rounded-lg p-4 mb-6">
                <h4 className="text-slate-300 font-medium mb-4">Proxy</h4>
                <div className="space-y-4">
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="use_http_proxy"
                      checked={settings?.use_http_proxy || false}
                      onChange={(e) => updateSetting('use_http_proxy', e.target.checked)}
                      className="w-4 h-4 bg-[#2a2a2a] border-white/10 rounded"
                    />
                    <label htmlFor="use_http_proxy" className="text-sm text-slate-300">Use HTTP Proxy</label>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">Proxy Address</label>
                    <input
                      type="text"
                      value={settings?.http_proxy || ''}
                      onChange={(e) => updateSetting('http_proxy', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                      placeholder="http://user:pass@host:port"
                    />
                    <p className="text-xs text-slate-500 mt-1">Example: http://127.0.0.1:8080 or http://user:pass@proxy.local:3128</p>
                  </div>
                </div>
              </div>

              {/* Headless VidX */}
              <div className="bg-gray-900 rounded-lg p-4 mb-6">
                <h4 className="text-slate-300 font-medium mb-4">Headless VidX</h4>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">Service Address</label>
                    <input
                      type="text"
                      value={settings?.headless_vidx_address || ''}
                      onChange={(e) => updateSetting('headless_vidx_address', e.target.value)}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500 font-mono text-sm"
                      placeholder="http://localhost:9000"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-slate-300 mb-2">Max Threads</label>
                    <select
                      value={settings?.headless_vidx_max_threads || 4}
                      onChange={(e) => updateSetting('headless_vidx_max_threads', Number(e.target.value))}
                      className="w-full px-3 py-2 bg-[#2a2a2a] border border-white/10 rounded-lg text-white focus:outline-none focus:border-blue-500"
                    >
                      {[1,2,4,8,16,24,32].map(t => (
                        <option key={t} value={t}>{t}</option>
                      ))}
                    </select>
                    <p className="text-xs text-slate-500 mt-1">Tune concurrency for processing; higher values use more CPU.</p>
                  </div>
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
                    <div className="mt-3">
                      <label className="block text-xs text-slate-400 mb-1">Update Channel</label>
                      <select
                        value={settings?.update_branch || 'main'}
                        onChange={(e) => updateSetting('update_branch', e.target.value)}
                        className="px-3 py-2 bg-[#1f1f1f] border border-white/10 rounded text-white text-sm"
                      >
                        <option value="main">main</option>
                        <option value="dev">dev</option>
                        <option value="tag">tag (stable releases)</option>
                      </select>
                    </div>
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
                    : 'Checking updates from your configured branch.'}
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
                  <div>• Streaming via <a href="https://real-debrid.com" target="_blank" rel="noopener noreferrer" className="text-red-400 hover:underline">Real-Debrid</a> and Stremio addons</div>
                  <div>• Live TV channels from various free sources</div>
                </div>
                <div className="text-xs text-slate-500 mt-4 pt-4 border-t border-white/10">
                  StreamArr is open source software licensed under MIT. Use responsibly.
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
