// Centralized Supabase Configuration
// Single source of truth for Supabase connection settings

const SUPABASE_CONFIG = {
  url: 'https://supabase.hawkeye123.dk',
  anonKey: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYW5vbiIsImlzcyI6InN1cGFiYXNlIiwiaWF0IjoxNzg2NDk3MDQ5LCJleHAiOjQ5NDAwOTcwNDl9.Inmhq9QxPXQKburj2RzRS-bZROeYfYT_k8A9ti-faVo'
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
