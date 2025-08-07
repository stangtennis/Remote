// Global Supabase Configuration for Remote Desktop System
// This replaces the local Socket.IO server with global Supabase connectivity

const { createClient } = require('@supabase/supabase-js');

// Supabase configuration
const SUPABASE_CONFIG = {
    url: 'https://ptrtibzwokjcjjxvjpin.supabase.co',
    anonKey: 'sb_publishable_OwJ04BrJ6gZZ57fSeT-q5Q_mSsVaNia',
    
    // Realtime configuration for global performance
    realtime: {
        params: {
            eventsPerSecond: 10,
            heartbeatIntervalMs: 30000,
            reconnectDelayMs: 2000,
            timeoutMs: 10000
        }
    },

    // Database configuration
    db: {
        schema: 'public'
    },

    // Global settings
    global: {
        autoReconnect: true,
        maxReconnectAttempts: 10,
        presenceHeartbeatInterval: 30000, // 30 seconds
        deviceRegistrationTimeout: 10000, // 10 seconds
        sessionTimeout: 300000 // 5 minutes
    }
};

// Create global Supabase client
function createGlobalSupabaseClient() {
    const client = createClient(SUPABASE_CONFIG.url, SUPABASE_CONFIG.anonKey, {
        realtime: SUPABASE_CONFIG.realtime,
        db: SUPABASE_CONFIG.db
    });

    // Add global error handling
    client.realtime.onError = (error) => {
        console.error('🔥 Supabase Realtime Error:', error);
    };

    client.realtime.onClose = () => {
        console.log('🔌 Supabase Realtime Connection Closed');
    };

    client.realtime.onOpen = () => {
        console.log('✅ Supabase Realtime Connection Opened');
    };

    return client;
}

// Global device registration function
async function registerDeviceGlobally(supabase, deviceInfo) {
    try {
        console.log('🌍 Registering device globally:', deviceInfo.deviceId);

        // Register device in remote_devices table
        const { data: device, error: deviceError } = await supabase
            .from('remote_devices')
            .upsert({
                device_id: deviceInfo.deviceId,
                device_name: deviceInfo.deviceName,
                operating_system: deviceInfo.operatingSystem,
                status: 'online',
                last_seen: new Date().toISOString(),
                metadata: {
                    version: deviceInfo.version,
                    capabilities: deviceInfo.capabilities || ['screen_share', 'remote_input'],
                    screen_resolution: deviceInfo.screenResolution,
                    connection_type: 'global_supabase',
                    registered_at: new Date().toISOString()
                }
            })
            .select()
            .single();

        if (deviceError) {
            console.error('❌ Device registration failed:', deviceError);
            throw deviceError;
        }

        // Update presence status
        const { error: presenceError } = await supabase
            .from('device_presence')
            .upsert({
                device_id: deviceInfo.deviceId,
                status: 'online',
                last_seen: new Date().toISOString(),
                connection_info: {
                    ip_address: await getPublicIP(),
                    user_agent: getUserAgent(),
                    connection_time: new Date().toISOString()
                },
                metadata: {
                    client_version: deviceInfo.version,
                    platform: process.platform,
                    arch: process.arch
                }
            });

        if (presenceError) {
            console.error('⚠️ Presence update failed:', presenceError);
            // Don't throw - presence is not critical for basic functionality
        }

        console.log('✅ Device registered globally successfully');
        return { success: true, device };

    } catch (error) {
        console.error('🔥 Global device registration failed:', error);
        return { success: false, error: error.message };
    }
}

// Global presence management
class GlobalPresenceManager {
    constructor(supabase, deviceId) {
        this.supabase = supabase;
        this.deviceId = deviceId;
        this.presenceChannel = null;
        this.heartbeatInterval = null;
        this.isOnline = false;
    }

    async startPresenceTracking() {
        try {
            console.log('📡 Starting global presence tracking for:', this.deviceId);

            // Subscribe to presence channel
            this.presenceChannel = this.supabase
                .channel('device_presence')
                .on('presence', { event: 'sync' }, () => {
                    console.log('🔄 Presence state synced globally');
                })
                .on('presence', { event: 'join' }, ({ key, newPresences }) => {
                    console.log('👋 Device joined globally:', key);
                })
                .on('presence', { event: 'leave' }, ({ key, leftPresences }) => {
                    console.log('👋 Device left globally:', key);
                })
                .subscribe();

            // Track this device's presence
            await this.presenceChannel.track({
                device_id: this.deviceId,
                status: 'online',
                timestamp: new Date().toISOString(),
                location: 'global'
            });

            this.isOnline = true;

            // Start heartbeat
            this.startHeartbeat();

            console.log('✅ Global presence tracking started');
            return true;

        } catch (error) {
            console.error('❌ Failed to start global presence tracking:', error);
            return false;
        }
    }

    startHeartbeat() {
        this.heartbeatInterval = setInterval(async () => {
            try {
                await this.updatePresence();
            } catch (error) {
                console.error('💓 Heartbeat failed:', error);
            }
        }, SUPABASE_CONFIG.global.presenceHeartbeatInterval);
    }

    async updatePresence() {
        if (!this.isOnline) return;

        await this.supabase
            .from('device_presence')
            .upsert({
                device_id: this.deviceId,
                status: 'online',
                last_seen: new Date().toISOString()
            });
    }

    async setStatus(status) {
        await this.supabase
            .from('device_presence')
            .upsert({
                device_id: this.deviceId,
                status: status,
                last_seen: new Date().toISOString()
            });
    }

    async stopPresenceTracking() {
        console.log('🛑 Stopping global presence tracking');

        this.isOnline = false;

        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
            this.heartbeatInterval = null;
        }

        if (this.presenceChannel) {
            await this.presenceChannel.unsubscribe();
            this.presenceChannel = null;
        }

        // Mark as offline
        await this.supabase
            .from('device_presence')
            .upsert({
                device_id: this.deviceId,
                status: 'offline',
                last_seen: new Date().toISOString()
            });
    }
}

// Global device communication manager
class GlobalDeviceCommunication {
    constructor(supabase, deviceId) {
        this.supabase = supabase;
        this.deviceId = deviceId;
        this.channels = new Map();
    }

    async subscribeToControlEvents() {
        const controlChannel = this.supabase
            .channel(`control:${this.deviceId}`)
            .on('broadcast', { event: 'control_request' }, (payload) => {
                this.handleControlRequest(payload.payload);
            })
            .on('broadcast', { event: 'control_end' }, (payload) => {
                this.handleControlEnd(payload.payload);
            })
            .on('broadcast', { event: 'remote_input' }, (payload) => {
                this.handleRemoteInput(payload.payload);
            })
            .subscribe();

        this.channels.set('control', controlChannel);
        console.log('🎮 Subscribed to global control events');
    }

    async handleControlRequest(data) {
        console.log('🎯 Global control request received:', data);
        // This will be implemented in Phase 2
        // For now, just log the request
    }

    async handleControlEnd(data) {
        console.log('🛑 Global control session ended:', data);
        // This will be implemented in Phase 2
    }

    async handleRemoteInput(data) {
        console.log('🖱️ Global remote input received:', data);
        // This will be implemented in Phase 2
    }

    async sendResponse(channelName, event, data) {
        const channel = this.channels.get(channelName);
        if (channel) {
            await channel.send({
                type: 'broadcast',
                event: event,
                payload: data
            });
        }
    }

    async cleanup() {
        for (const [name, channel] of this.channels) {
            await channel.unsubscribe();
        }
        this.channels.clear();
    }
}

// Utility functions
async function getPublicIP() {
    try {
        const response = await fetch('https://api.ipify.org?format=json');
        const data = await response.json();
        return data.ip;
    } catch (error) {
        return 'unknown';
    }
}

function getUserAgent() {
    if (typeof navigator !== 'undefined') {
        return navigator.userAgent;
    }
    return `Node.js/${process.version} (${process.platform}; ${process.arch})`;
}

// Export configuration and classes
module.exports = {
    SUPABASE_CONFIG,
    createGlobalSupabaseClient,
    registerDeviceGlobally,
    GlobalPresenceManager,
    GlobalDeviceCommunication
};
