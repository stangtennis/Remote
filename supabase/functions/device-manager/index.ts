import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

serve(async (req) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }
  
  try {
    // For API endpoints, create authenticated Supabase client
    const supabaseClient = createClient(
      'https://ptrtibzwokjcjjxvjpin.supabase.co',
      'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk',
      {
        global: {
          headers: {
            Authorization: req.headers.get('Authorization') || ''
          }
        }
      }
    )

    const url = new URL(req.url)
    const path = url.pathname

    // Handle API endpoints (require authentication)
    if (path.includes('/api/')) {
      const endpoint = path.split('/api/')[1]
      
      switch (endpoint) {
        case 'devices':
          if (req.method === 'GET') {
            const { data, error } = await supabaseClient
              .from('remote_devices')
              .select('*')
              .order('last_seen', { ascending: false })
            
            if (error) throw error
            
            return new Response(JSON.stringify(data), {
              headers: { ...corsHeaders, 'Content-Type': 'application/json' }
            })
          }
          break
          
        case 'device-stats':
          if (req.method === 'GET') {
            const { data, error } = await supabaseClient
              .from('remote_devices')
              .select('is_online, created_at')
            
            if (error) throw error
            
            const stats = {
              total: data.length,
              online: data.filter(d => d.is_online).length,
              offline: data.filter(d => !d.is_online).length,
              today: data.filter(d => {
                const today = new Date()
                const deviceDate = new Date(d.created_at)
                return deviceDate.toDateString() === today.toDateString()
              }).length
            }
            
            return new Response(JSON.stringify(stats), {
              headers: { ...corsHeaders, 'Content-Type': 'application/json' }
            })
          }
          break
      }
    }

    // Serve device manager HTML
    const deviceManagerHtml = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🖥️ Device Manager - Supabase Hosted</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #1e3c72 0%, #2a5298 100%);
            min-height: 100vh; color: #333;
        }
        
        .header {
            background: rgba(255,255,255,0.95); backdrop-filter: blur(10px);
            padding: 20px 0; box-shadow: 0 2px 20px rgba(0,0,0,0.1);
        }
        .header-content {
            max-width: 1400px; margin: 0 auto; padding: 0 20px;
            display: flex; justify-content: space-between; align-items: center;
        }
        .header h1 { color: #2a5298; font-size: 2rem; font-weight: 700; }
        .header-stats {
            display: flex; gap: 30px; align-items: center;
        }
        .stat-item {
            text-align: center; padding: 10px 20px;
            background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
            border-radius: 10px; color: white; min-width: 120px;
        }
        .stat-number { font-size: 1.8rem; font-weight: 700; }
        .stat-label { font-size: 0.9rem; opacity: 0.9; }
        
        .main-content {
            max-width: 1400px; margin: 0 auto; padding: 30px 20px;
        }
        
        .controls-bar {
            background: rgba(255,255,255,0.95); backdrop-filter: blur(10px);
            border-radius: 15px; padding: 20px; margin-bottom: 30px;
            display: flex; justify-content: space-between; align-items: center;
            box-shadow: 0 8px 32px rgba(0,0,0,0.1);
        }
        .search-box {
            flex: 1; max-width: 400px; position: relative;
        }
        .search-box input {
            width: 100%; padding: 12px 45px 12px 15px; border: 2px solid #e1e5e9;
            border-radius: 10px; font-size: 16px; background: white;
        }
        .search-box::after {
            content: '🔍'; position: absolute; right: 15px; top: 50%;
            transform: translateY(-50%); font-size: 18px;
        }
        .filter-buttons {
            display: flex; gap: 10px;
        }
        .filter-btn {
            padding: 10px 20px; border: 2px solid #e1e5e9; background: white;
            border-radius: 8px; cursor: pointer; transition: all 0.3s ease;
            font-weight: 600;
        }
        .filter-btn.active { background: #4f46e5; color: white; border-color: #4f46e5; }
        .refresh-btn {
            background: linear-gradient(135deg, #10b981 0%, #059669 100%);
            color: white; border: none; padding: 12px 25px; border-radius: 10px;
            cursor: pointer; font-weight: 600; transition: all 0.3s ease;
        }
        .refresh-btn:hover { transform: translateY(-2px); box-shadow: 0 8px 25px rgba(16, 185, 129, 0.3); }
        
        .devices-grid {
            display: grid; grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
            gap: 25px; margin-bottom: 30px;
        }
        
        .device-card {
            background: rgba(255,255,255,0.95); backdrop-filter: blur(10px);
            border-radius: 20px; padding: 25px; box-shadow: 0 8px 32px rgba(0,0,0,0.1);
            transition: all 0.3s ease; border: 2px solid transparent;
        }
        .device-card:hover {
            transform: translateY(-5px); box-shadow: 0 20px 40px rgba(0,0,0,0.15);
            border-color: #4f46e5;
        }
        .device-card.online { border-left: 5px solid #10b981; }
        .device-card.offline { border-left: 5px solid #ef4444; }
        
        .device-header {
            display: flex; justify-content: space-between; align-items: center;
            margin-bottom: 20px;
        }
        .device-name {
            font-size: 1.4rem; font-weight: 700; color: #1f2937;
        }
        .device-status {
            padding: 6px 12px; border-radius: 20px; font-size: 0.85rem;
            font-weight: 600; text-transform: uppercase;
        }
        .status-online { background: #d1fae5; color: #065f46; }
        .status-offline { background: #fee2e2; color: #991b1b; }
        
        .device-info {
            display: grid; grid-template-columns: 1fr 1fr; gap: 15px;
            margin-bottom: 20px;
        }
        .info-item {
            display: flex; flex-direction: column;
        }
        .info-label {
            font-size: 0.8rem; color: #6b7280; font-weight: 600;
            text-transform: uppercase; margin-bottom: 4px;
        }
        .info-value {
            font-size: 1rem; color: #1f2937; font-weight: 500;
        }
        
        .device-actions {
            display: flex; gap: 10px; flex-wrap: wrap;
        }
        .action-btn {
            flex: 1; min-width: 120px; padding: 12px 15px; border: none;
            border-radius: 10px; cursor: pointer; font-weight: 600;
            transition: all 0.3s ease; text-align: center;
        }
        .btn-connect {
            background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
            color: white;
        }
        .btn-connect:hover {
            transform: translateY(-2px); box-shadow: 0 8px 25px rgba(79, 70, 229, 0.3);
        }
        .btn-info {
            background: linear-gradient(135deg, #06b6d4 0%, #0891b2 100%);
            color: white;
        }
        .btn-disabled {
            background: #e5e7eb; color: #9ca3af; cursor: not-allowed;
        }
        
        .empty-state {
            text-align: center; padding: 60px 20px;
            background: rgba(255,255,255,0.95); backdrop-filter: blur(10px);
            border-radius: 20px; box-shadow: 0 8px 32px rgba(0,0,0,0.1);
        }
        .empty-state h3 { color: #6b7280; font-size: 1.5rem; margin-bottom: 15px; }
        .empty-state p { color: #9ca3af; font-size: 1.1rem; }
        
        .back-link {
            display: inline-block; margin-top: 20px; color: white;
            text-decoration: none; font-weight: 600; background: rgba(255,255,255,0.2);
            padding: 10px 20px; border-radius: 10px; backdrop-filter: blur(10px);
        }
        .back-link:hover { background: rgba(255,255,255,0.3); }
        
        @media (max-width: 768px) {
            .devices-grid { grid-template-columns: 1fr; }
            .header-content { flex-direction: column; gap: 20px; }
            .controls-bar { flex-direction: column; gap: 20px; }
            .device-info { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="header-content">
            <h1>🖥️ Device Manager</h1>
            <div class="header-stats">
                <div class="stat-item">
                    <div class="stat-number" id="onlineCount">-</div>
                    <div class="stat-label">Online</div>
                </div>
                <div class="stat-item">
                    <div class="stat-number" id="totalCount">-</div>
                    <div class="stat-label">Total</div>
                </div>
                <div class="stat-item">
                    <div class="stat-number" id="todayCount">-</div>
                    <div class="stat-label">Today</div>
                </div>
            </div>
        </div>
    </div>

    <div class="main-content">
        <div class="controls-bar">
            <div class="search-box">
                <input type="text" id="searchInput" placeholder="Search devices by name, IP, or platform...">
            </div>
            <div class="filter-buttons">
                <button class="filter-btn active" data-filter="all">All</button>
                <button class="filter-btn" data-filter="online">Online</button>
                <button class="filter-btn" data-filter="offline">Offline</button>
            </div>
            <button class="refresh-btn" onclick="refreshDevices()">🔄 Refresh</button>
        </div>

        <div id="devicesContainer">
            <div class="devices-grid" id="devicesGrid">
                <!-- Devices will be populated here -->
            </div>
            
            <div class="empty-state" id="emptyState">
                <h3>🖥️ No Devices Connected</h3>
                <p>Devices will appear here when agents connect to the system.</p>
                <p style="margin-top: 10px; font-size: 0.9rem;">
                    Download agents from: 
                    <a href="/functions/v1/agent-generator" style="color: #4f46e5; font-weight: 600;">Agent Generator</a>
                </p>
            </div>
        </div>
        
        <a href="/functions/v1/dashboard" class="back-link">← Back to Dashboard</a>
    </div>

    <script src="https://unpkg.com/@supabase/supabase-js@2"></script>
    <script>
        console.log('🖥️ Supabase-hosted Device Manager loaded');
        
        // Supabase configuration
        const SUPABASE_URL = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
        const SUPABASE_ANON_KEY = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk';
        
        const { createClient } = supabase;
        const supabaseClient = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
        
        let devices = [];
        let currentFilter = 'all';
        
        // Initialize device manager
        async function initDeviceManager() {
            console.log('🚀 Initializing device manager...');
            await refreshDevices();
            await updateStats();
            setupEventListeners();
            setupRealtimeSubscription();
            
            // Auto-refresh every 30 seconds
            setInterval(refreshDevices, 30000);
        }
        
        // Setup event listeners
        function setupEventListeners() {
            document.getElementById('searchInput').addEventListener('input', filterDevices);
            
            document.querySelectorAll('.filter-btn').forEach(btn => {
                btn.addEventListener('click', (e) => {
                    document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
                    e.target.classList.add('active');
                    currentFilter = e.target.dataset.filter;
                    filterDevices();
                });
            });
        }
        
        // Setup realtime subscription
        function setupRealtimeSubscription() {
            console.log('📡 Setting up realtime subscription...');
            
            supabaseClient
                .channel('devices')
                .on('postgres_changes', 
                    { event: '*', schema: 'public', table: 'remote_devices' },
                    (payload) => {
                        console.log('📨 Device update received:', payload);
                        refreshDevices();
                        updateStats();
                    }
                )
                .subscribe();
        }
        
        // Refresh devices
        async function refreshDevices() {
            try {
                const { data, error } = await supabaseClient
                    .from('remote_devices')
                    .select('*')
                    .order('last_seen', { ascending: false });
                
                if (error) throw error;
                
                devices = data || [];
                console.log('✅ Loaded ' + devices.length + ' devices');
                renderDevices();
                
            } catch (error) {
                console.error('❌ Failed to refresh devices:', error);
            }
        }
        
        // Update statistics
        async function updateStats() {
            try {
                const response = await fetch('/functions/v1/device-manager/api/device-stats');
                const stats = await response.json();
                
                document.getElementById('onlineCount').textContent = stats.online;
                document.getElementById('totalCount').textContent = stats.total;
                document.getElementById('todayCount').textContent = stats.today;
                
            } catch (error) {
                console.error('❌ Failed to update stats:', error);
            }
        }
        
        // Render devices
        function renderDevices() {
            const grid = document.getElementById('devicesGrid');
            const emptyState = document.getElementById('emptyState');
            
            if (devices.length === 0) {
                grid.style.display = 'none';
                emptyState.style.display = 'block';
                return;
            }
            
            grid.style.display = 'grid';
            emptyState.style.display = 'none';
            
            const filteredDevices = getFilteredDevices();
            grid.innerHTML = filteredDevices.map(device => createDeviceCard(device)).join('');
        }
        
        // Get filtered devices
        function getFilteredDevices() {
            const searchTerm = document.getElementById('searchInput').value.toLowerCase();
            
            return devices.filter(device => {
                const matchesSearch = !searchTerm || 
                    device.device_name.toLowerCase().includes(searchTerm) ||
                    (device.ip_address && device.ip_address.includes(searchTerm)) ||
                    (device.operating_system && device.operating_system.toLowerCase().includes(searchTerm));
                
                const matchesFilter = currentFilter === 'all' || 
                    (currentFilter === 'online' && device.is_online) ||
                    (currentFilter === 'offline' && !device.is_online);
                
                return matchesSearch && matchesFilter;
            });
        }
        
        // Create device card
        function createDeviceCard(device) {
            const isOnline = device.is_online;
            const lastSeen = device.last_seen ? new Date(device.last_seen) : null;
            const onlineTime = lastSeen ? formatTimeDifference(lastSeen) : 'Never';
            
            const status = isOnline ? 'online' : 'offline';
            const statusText = isOnline ? 'Online' : 'Offline';
            
            return '<div class="device-card ' + status + '">' +
                '<div class="device-header">' +
                    '<div class="device-name">' + escapeHtml(device.device_name) + '</div>' +
                    '<div class="device-status status-' + status + '">' + statusText + '</div>' +
                '</div>' +
                '<div class="device-info">' +
                    '<div class="info-item">' +
                        '<div class="info-label">Device ID</div>' +
                        '<div class="info-value">' + (device.access_key || device.id) + '</div>' +
                    '</div>' +
                    '<div class="info-item">' +
                        '<div class="info-label">IP Address</div>' +
                        '<div class="info-value">' + (device.ip_address || 'Unknown') + '</div>' +
                    '</div>' +
                    '<div class="info-item">' +
                        '<div class="info-label">Platform</div>' +
                        '<div class="info-value">' + (device.operating_system || 'Unknown') + '</div>' +
                    '</div>' +
                    '<div class="info-item">' +
                        '<div class="info-label">Last Seen</div>' +
                        '<div class="info-value">' + onlineTime + '</div>' +
                    '</div>' +
                '</div>' +
                '<div class="device-actions">' +
                    (isOnline ? 
                        '<button class="action-btn btn-connect" onclick="connectToDevice(\'' + device.id + '\')">🖥️ Connect</button>' :
                        '<button class="action-btn btn-disabled" disabled>🖥️ Offline</button>'
                    ) +
                    '<button class="action-btn btn-info" onclick="showDeviceInfo(\'' + device.id + '\')">ℹ️ Details</button>' +
                '</div>' +
            '</div>';
        }
        
        // Device actions
        function connectToDevice(deviceId) {
            console.log('🔌 Connecting to device:', deviceId);
            alert('Remote connection will be implemented in the next version.\\n\\nThis will provide full remote desktop capabilities.');
        }
        
        function showDeviceInfo(deviceId) {
            const device = devices.find(d => d.id === deviceId);
            if (!device) return;
            
            alert('Device Information:\n\nName: ' + device.device_name + '\nID: ' + device.id + '\nPlatform: ' + (device.operating_system || 'Unknown') + '\nStatus: ' + (device.is_online ? 'Online' : 'Offline') + '\nLast Seen: ' + (device.last_seen ? new Date(device.last_seen).toLocaleString() : 'Never'));
        }
        
        function filterDevices() {
            renderDevices();
        }
        
        // Utility functions
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
        
        function formatTimeDifference(date) {
            const now = new Date();
            const diff = now - date;
            const minutes = Math.floor(diff / 60000);
            const hours = Math.floor(minutes / 60);
            const days = Math.floor(hours / 24);
            
            if (days > 0) return days + 'd ago';
            if (hours > 0) return hours + 'h ago';
            if (minutes > 0) return minutes + 'm ago';
            return 'Just now';
        }
        
        // Initialize when page loads
        document.addEventListener('DOMContentLoaded', initDeviceManager);
    </script>
</body>
</html>`;

    return new Response(deviceManagerHtml, {
      headers: { 
        ...corsHeaders, 
        'Content-Type': 'text/html; charset=utf-8',
        'X-Frame-Options': 'SAMEORIGIN',
        'X-Content-Type-Options': 'nosniff',
        'Cache-Control': 'public, max-age=300'
      }
    })

  } catch (error) {
    return new Response(JSON.stringify({ error: error.message }), {
      status: 500,
      headers: { ...corsHeaders, 'Content-Type': 'application/json' }
    })
  }
})
