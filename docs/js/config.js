// Centralized Supabase Configuration
// Single source of truth for Supabase connection settings

const SUPABASE_CONFIG = {
  url: 'https://supabase.hawkeye123.dk',
  anonKey: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE'
};

// Debug mode: enable via ?debug=true in URL or localStorage.setItem('debug','true')
const DEBUG = new URLSearchParams(window.location.search).get('debug') === 'true'
  || localStorage.getItem('debug') === 'true';

function debug(...args) {
  if (DEBUG) console.log(...args);
}

// Initialize Supabase client and export to window
window.SUPABASE_CONFIG = SUPABASE_CONFIG;
window.DEBUG = DEBUG;
window.debug = debug;
window.supabase = window.supabase.createClient(SUPABASE_CONFIG.url, SUPABASE_CONFIG.anonKey);
