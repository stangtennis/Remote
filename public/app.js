// Remote Desktop Application - Frontend JavaScript

class RemoteDesktopApp {
    constructor() {
        this.socket = null;
        this.currentUser = null;
        this.currentSession = null;
        this.devices = [];
        this.sessions = [];
        this.authToken = localStorage.getItem('authToken');
        
        this.init();
    }

    init() {
        this.setupEventListeners();
        this.checkAuthentication();
        this.initializeSocket();
    }

    setupEventListeners() {
        // Navigation
        document.querySelectorAll('.nav-link').forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const section = e.target.closest('.nav-link').dataset.section;
                this.showSection(section);
            });
        });

        // Auth forms
        document.getElementById('login-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.handleLogin();
        });

        document.getElementById('register-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.handleRegister();
        });

        // Auth tabs
        document.querySelectorAll('.auth-tab').forEach(tab => {
            tab.addEventListener('click', (e) => {
                const tabName = e.target.dataset.tab;
                this.showAuthTab(tabName);
            });
        });

        // Add device form
        document.getElementById('add-device-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.handleAddDevice();
        });

        // Session filters
        document.getElementById('session-filter').addEventListener('change', (e) => {
            this.filterSessions(e.target.value);
        });
    }

    checkAuthentication() {
        if (this.authToken) {
            this.validateToken();
        } else {
            this.showAuthModal();
        }
    }

    async validateToken() {
        try {
            const response = await fetch('/api/health', {
                headers: {
                    'Authorization': `Bearer ${this.authToken}`
                }
            });

            if (response.ok) {
                this.hideAuthModal();
                this.loadUserData();
            } else {
                localStorage.removeItem('authToken');
                this.showAuthModal();
            }
        } catch (error) {
            console.error('Token validation error:', error);
            this.showAuthModal();
        }
    }

    async handleLogin() {
        const username = document.getElementById('login-username').value;
        const password = document.getElementById('login-password').value;

        try {
            const response = await fetch('/api/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ username, password })
            });

            const data = await response.json();

            if (response.ok) {
                this.authToken = data.token;
                this.currentUser = data.user;
                localStorage.setItem('authToken', this.authToken);
                
                this.hideAuthModal();
                this.loadUserData();
                this.showNotification('Login successful!', 'success');
            } else {
                this.showNotification(data.error || 'Login failed', 'error');
            }
        } catch (error) {
            console.error('Login error:', error);
            this.showNotification('Login failed. Please try again.', 'error');
        }
    }

    async handleRegister() {
        const email = document.getElementById('register-email').value;
        const username = document.getElementById('register-username').value;
        const fullName = document.getElementById('register-fullname').value;
        const password = document.getElementById('register-password').value;

        try {
            const response = await fetch('/api/register', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ email, username, fullName, password })
            });

            const data = await response.json();

            if (response.ok) {
                this.authToken = data.token;
                this.currentUser = data.user;
                localStorage.setItem('authToken', this.authToken);
                
                this.hideAuthModal();
                this.loadUserData();
                this.showNotification('Registration successful!', 'success');
            } else {
                this.showNotification(data.error || 'Registration failed', 'error');
            }
        } catch (error) {
            console.error('Registration error:', error);
            this.showNotification('Registration failed. Please try again.', 'error');
        }
    }

    logout() {
        this.authToken = null;
        this.currentUser = null;
        localStorage.removeItem('authToken');
        this.showAuthModal();
        if (this.socket) {
            this.socket.disconnect();
        }
    }

    async loadUserData() {
        if (this.currentUser) {
            document.getElementById('username-display').textContent = this.currentUser.username;
            document.getElementById('user-info').style.display = 'flex';
        }

        await this.loadDevices();
        await this.loadSessions();
        this.updateDashboard();
    }

    async loadDevices() {
        try {
            const response = await fetch('/api/devices', {
                headers: {
                    'Authorization': `Bearer ${this.authToken}`
                }
            });

            if (response.ok) {
                this.devices = await response.json();
                this.renderDevices();
            }
        } catch (error) {
            console.error('Load devices error:', error);
        }
    }

    async loadSessions() {
        try {
            const response = await fetch('/api/sessions', {
                headers: {
                    'Authorization': `Bearer ${this.authToken}`
                }
            });

            if (response.ok) {
                this.sessions = await response.json();
                this.renderSessions();
            }
        } catch (error) {
            console.error('Load sessions error:', error);
        }
    }

    async handleAddDevice() {
        const deviceName = document.getElementById('device-name').value;
        const deviceType = document.getElementById('device-type').value;
        const operatingSystem = document.getElementById('operating-system').value;
        const ipAddress = document.getElementById('ip-address').value;
        const port = document.getElementById('port').value;

        try {
            const response = await fetch('/api/devices', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${this.authToken}`
                },
                body: JSON.stringify({
                    deviceName,
                    deviceType,
                    operatingSystem,
                    ipAddress,
                    port: parseInt(port)
                })
            });

            const data = await response.json();

            if (response.ok) {
                this.closeModal('add-device-modal');
                this.loadDevices();
                this.showNotification('Device added successfully!', 'success');
                document.getElementById('add-device-form').reset();
            } else {
                this.showNotification(data.error || 'Failed to add device', 'error');
            }
        } catch (error) {
            console.error('Add device error:', error);
            this.showNotification('Failed to add device. Please try again.', 'error');
        }
    }

    async connectToDevice(deviceId) {
        try {
            const response = await fetch('/api/sessions/start', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${this.authToken}`
                },
                body: JSON.stringify({ deviceId })
            });

            const data = await response.json();

            if (response.ok) {
                this.currentSession = data.session;
                this.showRemoteViewer(data.device);
                this.initializeRemoteConnection(data.session.session_token);
            } else {
                this.showNotification(data.error || 'Failed to connect to device', 'error');
            }
        } catch (error) {
            console.error('Connect to device error:', error);
            this.showNotification('Failed to connect to device. Please try again.', 'error');
        }
    }

    initializeSocket() {
        this.socket = io();

        this.socket.on('connect', () => {
            console.log('Connected to server');
        });

        this.socket.on('disconnect', () => {
            console.log('Disconnected from server');
        });

        this.socket.on('screen-capture', (data) => {
            this.updateRemoteScreen(data);
        });
    }

    initializeRemoteConnection(sessionToken) {
        if (this.socket) {
            this.socket.emit('join-session', sessionToken);
            
            // Set up remote screen event listeners
            const canvas = document.getElementById('remote-screen');
            const ctx = canvas.getContext('2d');

            // Mouse events
            canvas.addEventListener('mousemove', (e) => {
                const rect = canvas.getBoundingClientRect();
                const x = (e.clientX - rect.left) * (canvas.width / rect.width);
                const y = (e.clientY - rect.top) * (canvas.height / rect.height);
                
                this.socket.emit('mouse-move', {
                    sessionToken,
                    x: Math.round(x),
                    y: Math.round(y)
                });
            });

            canvas.addEventListener('click', (e) => {
                const rect = canvas.getBoundingClientRect();
                const x = (e.clientX - rect.left) * (canvas.width / rect.width);
                const y = (e.clientY - rect.top) * (canvas.height / rect.height);
                
                this.socket.emit('mouse-click', {
                    sessionToken,
                    x: Math.round(x),
                    y: Math.round(y),
                    button: e.button
                });
            });

            // Keyboard events
            document.addEventListener('keydown', (e) => {
                if (document.getElementById('remote-viewer-modal').classList.contains('active')) {
                    e.preventDefault();
                    this.socket.emit('key-press', {
                        sessionToken,
                        key: e.key,
                        keyCode: e.keyCode,
                        type: 'keydown'
                    });
                }
            });

            document.addEventListener('keyup', (e) => {
                if (document.getElementById('remote-viewer-modal').classList.contains('active')) {
                    e.preventDefault();
                    this.socket.emit('key-press', {
                        sessionToken,
                        key: e.key,
                        keyCode: e.keyCode,
                        type: 'keyup'
                    });
                }
            });
        }
    }

    updateRemoteScreen(data) {
        const canvas = document.getElementById('remote-screen');
        const ctx = canvas.getContext('2d');
        
        // In a real implementation, you would decode the screen capture data
        // and draw it on the canvas. For now, we'll show a placeholder.
        ctx.fillStyle = '#1e293b';
        ctx.fillRect(0, 0, canvas.width, canvas.height);
        
        ctx.fillStyle = '#64748b';
        ctx.font = '24px Arial';
        ctx.textAlign = 'center';
        ctx.fillText('Remote Desktop Screen', canvas.width / 2, canvas.height / 2);
        ctx.fillText('(Simulated View)', canvas.width / 2, canvas.height / 2 + 30);
    }

    renderDevices() {
        const grid = document.getElementById('devices-grid');
        grid.innerHTML = '';

        this.devices.forEach(device => {
            const deviceCard = document.createElement('div');
            deviceCard.className = 'device-card';
            deviceCard.innerHTML = `
                <div class="device-header">
                    <div class="device-info">
                        <h3>${device.device_name}</h3>
                        <p>${device.device_type} • ${device.operating_system}</p>
                    </div>
                    <span class="device-status ${device.is_online ? 'online' : 'offline'}">
                        ${device.is_online ? 'Online' : 'Offline'}
                    </span>
                </div>
                <div class="device-details">
                    <div class="device-detail">
                        <span>IP Address:</span>
                        <span>${device.ip_address || 'N/A'}</span>
                    </div>
                    <div class="device-detail">
                        <span>Port:</span>
                        <span>${device.port}</span>
                    </div>
                    <div class="device-detail">
                        <span>Last Seen:</span>
                        <span>${device.last_seen ? new Date(device.last_seen).toLocaleDateString() : 'Never'}</span>
                    </div>
                </div>
                <div class="device-actions">
                    <button class="btn-connect" onclick="app.connectToDevice('${device.id}')" ${!device.is_online ? 'disabled' : ''}>
                        <i class="fas fa-play"></i> Connect
                    </button>
                    <button class="btn-secondary" onclick="app.editDevice('${device.id}')">
                        <i class="fas fa-edit"></i> Edit
                    </button>
                </div>
            `;
            grid.appendChild(deviceCard);
        });
    }

    renderSessions() {
        const tbody = document.getElementById('sessions-tbody');
        tbody.innerHTML = '';

        this.sessions.forEach(session => {
            const row = document.createElement('tr');
            const duration = session.duration_seconds ? 
                this.formatDuration(session.duration_seconds) : 
                (session.status === 'active' ? 'Active' : '-');

            row.innerHTML = `
                <td>
                    <div>
                        <strong>${session.remote_devices?.device_name || 'Unknown'}</strong>
                        <br>
                        <small>${session.remote_devices?.device_type || ''}</small>
                    </div>
                </td>
                <td>${new Date(session.started_at).toLocaleString()}</td>
                <td>${duration}</td>
                <td>
                    <span class="session-status ${session.status}">
                        ${session.status.charAt(0).toUpperCase() + session.status.slice(1)}
                    </span>
                </td>
                <td>${session.connection_quality || 'N/A'}</td>
                <td>
                    ${session.status === 'active' ? 
                        `<button class="btn-secondary" onclick="app.disconnectSession('${session.id}')">
                            <i class="fas fa-stop"></i> Disconnect
                        </button>` : 
                        `<button class="btn-secondary" onclick="app.viewSessionDetails('${session.id}')">
                            <i class="fas fa-eye"></i> Details
                        </button>`
                    }
                </td>
            `;
            tbody.appendChild(row);
        });
    }

    updateDashboard() {
        const totalDevices = this.devices.length;
        const onlineDevices = this.devices.filter(d => d.is_online).length;
        const activeSessions = this.sessions.filter(s => s.status === 'active').length;
        const totalSessions = this.sessions.length;

        document.getElementById('total-devices').textContent = totalDevices;
        document.getElementById('online-devices').textContent = onlineDevices;
        document.getElementById('active-sessions').textContent = activeSessions;
        document.getElementById('total-sessions').textContent = totalSessions;

        this.renderRecentActivity();
    }

    renderRecentActivity() {
        const activityList = document.getElementById('recent-activity');
        activityList.innerHTML = '';

        // Get recent sessions and device additions
        const recentSessions = this.sessions
            .slice(0, 5)
            .map(session => ({
                type: 'session',
                title: `Connected to ${session.remote_devices?.device_name || 'Unknown Device'}`,
                time: session.started_at,
                icon: 'fas fa-play'
            }));

        const recentDevices = this.devices
            .slice(0, 3)
            .map(device => ({
                type: 'device',
                title: `Added device: ${device.device_name}`,
                time: device.created_at,
                icon: 'fas fa-plus'
            }));

        const allActivity = [...recentSessions, ...recentDevices]
            .sort((a, b) => new Date(b.time) - new Date(a.time))
            .slice(0, 5);

        allActivity.forEach(activity => {
            const activityItem = document.createElement('div');
            activityItem.className = 'activity-item';
            activityItem.innerHTML = `
                <div class="activity-icon">
                    <i class="${activity.icon}"></i>
                </div>
                <div class="activity-content">
                    <h4>${activity.title}</h4>
                    <p>${activity.type === 'session' ? 'Remote session' : 'Device management'}</p>
                </div>
                <div class="activity-time">
                    ${this.formatTimeAgo(activity.time)}
                </div>
            `;
            activityList.appendChild(activityItem);
        });
    }

    formatDuration(seconds) {
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        const secs = seconds % 60;

        if (hours > 0) {
            return `${hours}h ${minutes}m ${secs}s`;
        } else if (minutes > 0) {
            return `${minutes}m ${secs}s`;
        } else {
            return `${secs}s`;
        }
    }

    formatTimeAgo(dateString) {
        const date = new Date(dateString);
        const now = new Date();
        const diffMs = now - date;
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMins / 60);
        const diffDays = Math.floor(diffHours / 24);

        if (diffMins < 1) return 'Just now';
        if (diffMins < 60) return `${diffMins}m ago`;
        if (diffHours < 24) return `${diffHours}h ago`;
        return `${diffDays}d ago`;
    }

    filterSessions(status) {
        const rows = document.querySelectorAll('#sessions-tbody tr');
        rows.forEach(row => {
            const statusElement = row.querySelector('.session-status');
            if (status === 'all' || statusElement.textContent.toLowerCase().includes(status)) {
                row.style.display = '';
            } else {
                row.style.display = 'none';
            }
        });
    }

    showSection(sectionName) {
        // Update navigation
        document.querySelectorAll('.nav-link').forEach(link => {
            link.classList.remove('active');
        });
        document.querySelector(`[data-section="${sectionName}"]`).classList.add('active');

        // Show section
        document.querySelectorAll('.content-section').forEach(section => {
            section.classList.remove('active');
        });
        document.getElementById(`${sectionName}-section`).classList.add('active');

        // Load section-specific data
        if (sectionName === 'devices') {
            this.loadDevices();
        } else if (sectionName === 'sessions') {
            this.loadSessions();
        }
    }

    showAuthModal() {
        document.getElementById('auth-modal').classList.add('active');
    }

    hideAuthModal() {
        document.getElementById('auth-modal').classList.remove('active');
    }

    showAuthTab(tabName) {
        document.querySelectorAll('.auth-tab').forEach(tab => {
            tab.classList.remove('active');
        });
        document.querySelector(`[data-tab="${tabName}"]`).classList.add('active');

        document.querySelectorAll('.auth-form').forEach(form => {
            form.classList.remove('active');
        });
        document.getElementById(`${tabName}-form`).classList.add('active');
    }

    showAddDeviceModal() {
        document.getElementById('add-device-modal').classList.add('active');
    }

    showRemoteViewer(device) {
        document.getElementById('device-info').textContent = `${device.device_name} (${device.ip_address})`;
        document.getElementById('connection-status').textContent = 'Connected';
        document.getElementById('remote-viewer-modal').classList.add('active');
        
        // Hide connection overlay after a delay (simulate connection process)
        setTimeout(() => {
            document.getElementById('connection-overlay').style.display = 'none';
        }, 2000);
    }

    closeModal(modalId) {
        document.getElementById(modalId).classList.remove('active');
    }

    disconnectSession() {
        if (this.currentSession) {
            this.socket.emit('disconnect-session', this.currentSession.session_token);
            this.currentSession = null;
        }
        this.closeModal('remote-viewer-modal');
        document.getElementById('connection-overlay').style.display = 'flex';
    }

    toggleFullscreen() {
        const modal = document.getElementById('remote-viewer-modal');
        if (document.fullscreenElement) {
            document.exitFullscreen();
        } else {
            modal.requestFullscreen();
        }
    }

    showNotification(message, type = 'info') {
        const notification = document.createElement('div');
        notification.className = `notification ${type}`;
        notification.innerHTML = `
            <div>
                <strong>${type.charAt(0).toUpperCase() + type.slice(1)}</strong>
                <p>${message}</p>
            </div>
        `;

        document.body.appendChild(notification);
        
        setTimeout(() => {
            notification.classList.add('show');
        }, 100);

        setTimeout(() => {
            notification.classList.remove('show');
            setTimeout(() => {
                document.body.removeChild(notification);
            }, 300);
        }, 3000);
    }
}

// Global functions
function logout() {
    app.logout();
}

function showAddDeviceModal() {
    app.showAddDeviceModal();
}

function closeModal(modalId) {
    app.closeModal(modalId);
}

function toggleFullscreen() {
    app.toggleFullscreen();
}

function disconnectSession() {
    app.disconnectSession();
}

// Initialize app
const app = new RemoteDesktopApp();
