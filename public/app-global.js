// Global Supabase Remote Desktop Dashboard
// This version connects directly to Supabase for worldwide accessibility

import { createClient } from 'https://cdn.skypack.dev/@supabase/supabase-js@2';

class GlobalRemoteDesktopDashboard {
    constructor() {
        this.supabase = null;
        this.currentUser = null;
        this.devices = new Map();
        this.activeSessions = new Map();
        this.selectedDevice = null;
        this.isStreaming = false;
        this.streamChannel = null;
        this.agentManager = null;
        
        // UI Elements
        this.loginSection = document.getElementById('login-section');
        this.dashboardSection = document.getElementById('dashboard-section');
        this.devicesList = document.getElementById('devices-list');
        this.sessionsList = document.getElementById('sessions-list');
        this.controlSection = document.getElementById('control-section');
        this.screenDisplay = document.getElementById('screen-display');
        this.statusDisplay = document.getElementById('status-display');
        
        console.log('🌍 Global Remote Desktop Dashboard initializing...');
    }

    async initialize() {
        try {
            // Initialize Supabase client
            this.supabase = createClient(
                'https://ptrtibzwokjcjjxvjpin.supabase.co',
                'sb_publishable_OwJ04BrJ6gZZ57fSeT-q5Q_mSsVaNia'
            );

            console.log('✅ Global Supabase client initialized');

            // Set up event listeners
            this.setupEventListeners();

            // Check if user is already logged in
            const { data: { session } } = await this.supabase.auth.getSession();
            if (session) {
                this.currentUser = session.user;
                await this.showDashboard();
            } else {
                this.showLogin();
            }

            // Set up real-time subscriptions
            this.setupRealtimeSubscriptions();
            
            // Initialize agent manager
            this.initializeAgentManager();

        } catch (error) {
            console.error('❌ Failed to initialize dashboard:', error);
            this.showError('Initialization failed: ' + error.message);
        }
    }

    setupEventListeners() {
        // Navigation links
        document.querySelectorAll('.nav-link').forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const section = link.getAttribute('data-section');
                this.showSection(section);
            });
        });

        // Login form - check if exists
        const loginForm = document.getElementById('login-form');
        if (loginForm) {
            loginForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.handleLogin();
            });
        }

        // Logout button - check if exists
        const logoutBtn = document.getElementById('logout-btn');
        if (logoutBtn) {
            logoutBtn.addEventListener('click', () => {
                this.handleLogout();
            });
        }

        // Refresh devices button - check if exists
        const refreshDevices = document.getElementById('refresh-devices');
        if (refreshDevices) {
            refreshDevices.addEventListener('click', () => {
                this.loadDevices();
            });
        }

        // Control buttons - check if exists
        const startControl = document.getElementById('start-control');
        if (startControl) {
            startControl.addEventListener('click', () => {
                this.startRemoteControl();
            });
        }

        const endControl = document.getElementById('end-control');
        if (endControl) {
            endControl.addEventListener('click', () => {
                this.endRemoteControl();
            });
        }

        // Agent generation form - check if exists
        const agentForm = document.getElementById('agent-form');
        if (agentForm) {
            agentForm.addEventListener('submit', (e) => {
                e.preventDefault();
                this.generateAgent();
            });
        }

        // Screen display click handler for remote input
        if (this.screenDisplay) {
            this.screenDisplay.addEventListener('click', (e) => {
                if (this.isStreaming) {
                    this.sendRemoteInput('click', e);
                }
            });

            this.screenDisplay.addEventListener('mousemove', (e) => {
                if (this.isStreaming) {
                    this.sendRemoteInput('mousemove', e);
                }
            });
        }

        // Keyboard input handler
        document.addEventListener('keydown', (e) => {
            if (this.isStreaming && this.controlSection.style.display !== 'none') {
                e.preventDefault();
                this.sendRemoteInput('keydown', e);
            }
        });
    }

    initializeAgentManager() {
        try {
            // Initialize agent manager if the class is available
            if (typeof AgentManager !== 'undefined') {
                this.agentManager = new AgentManager(this.supabase);
                console.log('✅ Agent manager initialized');
                
                // Load initial statistics
                this.agentManager.loadAgentStatistics();
                
                // Update platform info for default selection
                this.agentManager.updatePlatformInfo('windows');
            } else {
                console.warn('⚠️ AgentManager class not found - agent management features disabled');
            }
        } catch (error) {
            console.error('❌ Failed to initialize agent manager:', error);
        }
    }

    setupRealtimeSubscriptions() {
        // Subscribe to device presence changes
        this.supabase
            .channel('device_presence')
            .on('postgres_changes', {
                event: '*',
                schema: 'public',
                table: 'device_presence'
            }, (payload) => {
                console.log('📱 Device presence changed:', payload);
                this.handleDevicePresenceChange(payload);
            })
            .subscribe();

        // Subscribe to device changes
        this.supabase
            .channel('devices')
            .on('postgres_changes', {
                event: '*',
                schema: 'public',
                table: 'remote_devices'
            }, (payload) => {
                console.log('🔄 Device updated:', payload);
                this.handleDeviceChange(payload);
            })
            .subscribe();

        // Subscribe to session changes
        this.supabase
            .channel('sessions')
            .on('postgres_changes', {
                event: '*',
                schema: 'public',
                table: 'remote_sessions'
            }, (payload) => {
                console.log('🎯 Session updated:', payload);
                this.handleSessionChange(payload);
            })
            .subscribe();
            
        // Subscribe to agent generation changes
        this.supabase
            .channel('agent_generations')
            .on('postgres_changes', {
                event: '*',
                schema: 'public',
                table: 'agent_generations'
            }, (payload) => {
                console.log('🔧 Agent generation updated:', payload);
                if (this.agentManager) {
                    this.agentManager.loadGeneratedAgents();
                }
            })
            .subscribe();
    }

    async handleLogin() {
        try {
            const email = document.getElementById('email').value;
            const password = document.getElementById('password').value;

            this.updateStatus('Signing in globally...');

            const { data, error } = await this.supabase.auth.signInWithPassword({
                email: email,
                password: password
            });

            if (error) {
                throw error;
            }

            this.currentUser = data.user;
            console.log('✅ Global login successful:', this.currentUser.email);
            
            await this.showDashboard();

        } catch (error) {
            console.error('❌ Login failed:', error);
            this.showError('Login failed: ' + error.message);
        }
    }

    async handleLogout() {
        try {
            await this.supabase.auth.signOut();
            this.currentUser = null;
            this.showLogin();
            console.log('✅ Logged out successfully');
        } catch (error) {
            console.error('❌ Logout failed:', error);
        }
    }

    showLogin() {
        this.loginSection.style.display = 'block';
        this.dashboardSection.style.display = 'none';
        this.updateStatus('Please sign in to access the global dashboard');
    }

    async showDashboard() {
        this.loginSection.style.display = 'none';
        this.dashboardSection.style.display = 'block';
        
        this.updateStatus(`Welcome, ${this.currentUser.email} - Loading global devices...`);
        
        // Load initial data
        await this.loadDevices();
        await this.loadSessions();
        
        this.updateStatus('Global dashboard ready');
    }

    async loadDevices() {
        try {
            console.log('🔄 Loading devices globally...');

            // Load devices with presence information
            const { data: devices, error } = await this.supabase
                .from('online_devices')
                .select('*')
                .order('last_seen', { ascending: false });

            if (error) {
                throw error;
            }

            this.devices.clear();
            devices.forEach(device => {
                this.devices.set(device.device_id, device);
            });

            this.renderDevices();
            console.log(`✅ Loaded ${devices.length} devices globally`);

        } catch (error) {
            console.error('❌ Failed to load devices:', error);
            this.showError('Failed to load devices: ' + error.message);
        }
    }

    async loadSessions() {
        try {
            console.log('🔄 Loading sessions globally...');

            const { data: sessions, error } = await this.supabase
                .from('active_sessions_view')
                .select('*')
                .order('created_at', { ascending: false });

            if (error) {
                throw error;
            }

            this.activeSessions.clear();
            sessions.forEach(session => {
                this.activeSessions.set(session.session_id, session);
            });

            this.renderSessions();
            console.log(`✅ Loaded ${sessions.length} active sessions globally`);

        } catch (error) {
            console.error('❌ Failed to load sessions:', error);
            this.showError('Failed to load sessions: ' + error.message);
        }
    }

    renderDevices() {
        this.devicesList.innerHTML = '';

        if (this.devices.size === 0) {
            this.devicesList.innerHTML = '<p class="no-data">No devices online globally</p>';
            return;
        }

        this.devices.forEach(device => {
            const deviceElement = document.createElement('div');
            deviceElement.className = 'device-item';
            deviceElement.innerHTML = `
                <div class="device-info">
                    <h3>${device.device_name}</h3>
                    <p><strong>ID:</strong> ${device.device_id}</p>
                    <p><strong>OS:</strong> ${device.operating_system}</p>
                    <p><strong>Status:</strong> <span class="status-${device.status}">${device.status}</span></p>
                    <p><strong>Last Seen:</strong> ${new Date(device.last_seen).toLocaleString()}</p>
                </div>
                <div class="device-actions">
                    <button onclick="dashboard.selectDevice('${device.device_id}')" 
                            ${device.status !== 'online' ? 'disabled' : ''}>
                        ${device.status === 'online' ? 'Connect' : 'Unavailable'}
                    </button>
                </div>
            `;

            this.devicesList.appendChild(deviceElement);
        });
    }

    renderSessions() {
        this.sessionsList.innerHTML = '';

        if (this.activeSessions.size === 0) {
            this.sessionsList.innerHTML = '<p class="no-data">No active sessions globally</p>';
            return;
        }

        this.activeSessions.forEach(session => {
            const sessionElement = document.createElement('div');
            sessionElement.className = 'session-item';
            sessionElement.innerHTML = `
                <div class="session-info">
                    <h4>Session ${session.session_id.substring(0, 8)}...</h4>
                    <p><strong>Device:</strong> ${session.device_name}</p>
                    <p><strong>Status:</strong> ${session.status}</p>
                    <p><strong>Started:</strong> ${new Date(session.started_at).toLocaleString()}</p>
                </div>
                <div class="session-actions">
                    <button onclick="dashboard.endSession('${session.session_id}')">
                        End Session
                    </button>
                </div>
            `;

            this.sessionsList.appendChild(sessionElement);
        });
    }

    async selectDevice(deviceId) {
        try {
            const device = this.devices.get(deviceId);
            if (!device) {
                throw new Error('Device not found');
            }

            if (device.status !== 'online') {
                throw new Error('Device is not online');
            }

            this.selectedDevice = device;
            console.log('🎯 Selected device:', deviceId);
            
            this.updateStatus(`Selected device: ${device.device_name} (${deviceId})`);
            
            // Show control section
            this.controlSection.style.display = 'block';
            document.getElementById('selected-device-info').innerHTML = `
                <h3>${device.device_name}</h3>
                <p>Device ID: ${deviceId}</p>
                <p>OS: ${device.operating_system}</p>
            `;

        } catch (error) {
            console.error('❌ Failed to select device:', error);
            this.showError('Failed to select device: ' + error.message);
        }
    }

    async startRemoteControl() {
        try {
            if (!this.selectedDevice) {
                throw new Error('No device selected');
            }

            console.log('🎮 Starting remote control session...');
            this.updateStatus('Starting remote control session...');

            // Create a new session
            const { data: session, error } = await this.supabase
                .from('remote_sessions')
                .insert({
                    device_id: this.selectedDevice.device_id,
                    created_by: this.currentUser.id,
                    status: 'pending'
                })
                .select()
                .single();

            if (error) {
                throw error;
            }

            console.log('✅ Remote control session created:', session.session_id);
            this.updateStatus('Waiting for device permission...');

            // Wait for session approval (this will be handled by realtime subscription)
            this.waitForSessionApproval(session.session_id);

        } catch (error) {
            console.error('❌ Failed to start remote control:', error);
            this.showError('Failed to start remote control: ' + error.message);
        }
    }

    async waitForSessionApproval(sessionId) {
        // Set up a timeout for approval
        const timeout = setTimeout(() => {
            this.updateStatus('Session request timed out');
            this.endRemoteControl();
        }, 30000); // 30 seconds timeout

        // The session approval will be handled by the realtime subscription
        // When the session status changes to 'active', we'll start streaming
        this.sessionApprovalTimeout = timeout;
    }

    async startScreenStream(sessionId) {
        try {
            console.log('📺 Starting screen stream...');
            this.isStreaming = true;

            // Subscribe to screen stream channel for this session
            this.streamChannel = this.supabase
                .channel(`screen_stream_${sessionId}`)
                .on('broadcast', { event: 'screen_frame' }, (payload) => {
                    this.displayScreenFrame(payload.payload);
                })
                .subscribe();

            this.updateStatus('Screen streaming active - Click to control remotely');
            
            // Update UI
            document.getElementById('start-control').style.display = 'none';
            document.getElementById('end-control').style.display = 'inline-block';
            this.screenDisplay.style.display = 'block';

        } catch (error) {
            console.error('❌ Failed to start screen stream:', error);
            this.showError('Failed to start screen stream: ' + error.message);
        }
    }

    displayScreenFrame(frameData) {
        if (frameData && frameData.image_data) {
            this.screenDisplay.innerHTML = `
                <img src="${frameData.image_data}" 
                     alt="Remote Screen" 
                     style="max-width: 100%; height: auto; cursor: crosshair;" />
                <div class="stream-info">
                    Device: ${frameData.device_id} | 
                    Time: ${new Date(frameData.timestamp).toLocaleTimeString()}
                </div>
            `;
        }
    }

    async sendRemoteInput(type, event) {
        try {
            if (!this.selectedDevice || !this.isStreaming) return;

            const inputData = {
                type: type,
                x: event.offsetX || 0,
                y: event.offsetY || 0,
                button: event.button || 0,
                key: event.key || '',
                keyCode: event.keyCode || 0,
                timestamp: new Date().toISOString()
            };

            // Send input via Supabase Realtime
            await this.supabase
                .channel(`control_${this.selectedDevice.device_id}`)
                .send({
                    type: 'broadcast',
                    event: 'remote_input',
                    payload: inputData
                });

            console.log('🖱️ Remote input sent:', inputData);

        } catch (error) {
            console.error('❌ Failed to send remote input:', error);
        }
    }

    async endRemoteControl() {
        try {
            console.log('🛑 Ending remote control session...');

            this.isStreaming = false;

            if (this.streamChannel) {
                await this.streamChannel.unsubscribe();
                this.streamChannel = null;
            }

            if (this.sessionApprovalTimeout) {
                clearTimeout(this.sessionApprovalTimeout);
                this.sessionApprovalTimeout = null;
            }

            // Update UI
            document.getElementById('start-control').style.display = 'inline-block';
            document.getElementById('end-control').style.display = 'none';
            this.screenDisplay.style.display = 'none';
            this.screenDisplay.innerHTML = '';

            this.updateStatus('Remote control session ended');

        } catch (error) {
            console.error('❌ Failed to end remote control:', error);
        }
    }

    async endSession(sessionId) {
        try {
            await this.supabase
                .from('remote_sessions')
                .update({
                    status: 'ended',
                    ended_at: new Date().toISOString()
                })
                .eq('session_id', sessionId);

            console.log('✅ Session ended:', sessionId);
            await this.loadSessions();

        } catch (error) {
            console.error('❌ Failed to end session:', error);
            this.showError('Failed to end session: ' + error.message);
        }
    }

    // Event handlers for realtime subscriptions
    handleDevicePresenceChange(payload) {
        const { eventType, new: newRecord, old: oldRecord } = payload;
        
        if (eventType === 'INSERT' || eventType === 'UPDATE') {
            // Update device in local cache
            if (this.devices.has(newRecord.device_id)) {
                const device = this.devices.get(newRecord.device_id);
                device.status = newRecord.status;
                device.last_seen = newRecord.last_seen;
                this.renderDevices();
            }
        }
    }

    handleDeviceChange(payload) {
        const { eventType, new: newRecord } = payload;
        
        if (eventType === 'INSERT' || eventType === 'UPDATE') {
            this.loadDevices(); // Reload all devices
        }
    }

    handleSessionChange(payload) {
        const { eventType, new: newRecord } = payload;
        
        if (eventType === 'UPDATE' && newRecord.status === 'active') {
            // Session was approved, start streaming
            if (this.sessionApprovalTimeout) {
                clearTimeout(this.sessionApprovalTimeout);
                this.sessionApprovalTimeout = null;
            }
            this.startScreenStream(newRecord.session_id);
        }
        
        this.loadSessions(); // Reload sessions
    }

    updateStatus(message) {
        this.statusDisplay.textContent = message;
        console.log(`📊 Status: ${message}`);
    }

    showError(message) {
        this.statusDisplay.textContent = `❌ Error: ${message}`;
        this.statusDisplay.style.color = '#ff4444';
        setTimeout(() => {
            this.statusDisplay.style.color = '';
        }, 5000);
    }
}

// Initialize dashboard when page loads
const dashboard = new GlobalRemoteDesktopDashboard();
window.dashboard = dashboard; // Make it globally accessible

document.addEventListener('DOMContentLoaded', () => {
    dashboard.initialize();
});

console.log('🌍 Global Remote Desktop Dashboard loaded');
