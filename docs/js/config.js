// Centralized Supabase Configuration
// Single source of truth for Supabase connection settings

const SUPABASE_CONFIG = {
  url: 'http://192.168.1.92:8888',
  anonKey: 'REDACTED_JWT'
};

// Initialize Supabase client and export to window
window.SUPABASE_CONFIG = SUPABASE_CONFIG;
window.supabase = window.supabase.createClient(SUPABASE_CONFIG.url, SUPABASE_CONFIG.anonKey);
