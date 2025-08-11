// Supabase Integration Module for WebRTC Remote Desktop
console.log('🔗 Loading Supabase Integration Module...');

// Supabase Configuration
const SUPABASE_URL = 'https://ptrtizbwokjcjjxvjpin.supabase.co';
const SUPABASE_ANON_KEY = 'REDACTED_JWT';

// Global Supabase client
let supabaseClient = null;
let realtimeChannel = null;

// Initialize Supabase client
async function initializeSupabase() {
    try {
        console.log('🔄 Initializing Supabase client...');
        
        // Create Supabase client
        supabaseClient = supabase.createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
        
        // Test connection
        const { data, error } = await supabaseClient
            .from('remote_devices')
            .select('count', { count: 'exact', head: true });
        
        if (error) {
            console.error('❌ Supabase connection test failed:', error);
            console.warn('⚠️ Continuing in offline mode with limited functionality');
            // Return client anyway to allow partial functionality
            return supabaseClient;
        }
        
        console.log('✅ Supabase client initialized successfully');
        return supabaseClient;
    } catch (error) {
        console.error('❌ Failed to initialize Supabase:', error);
        console.warn('⚠️ Continuing in offline mode with limited functionality');
        // Create a mock client for offline mode
        return createOfflineClient();
    }
}

// Create offline client with mock functions for testing
function createOfflineClient() {
    console.log('🔄 Creating offline mock client for testing');
    return {
        // Mock functions that return empty data
        from: () => ({
            select: () => Promise.resolve({ data: [], error: null }),
            insert: () => Promise.resolve({ data: {}, error: null }),
            update: () => Promise.resolve({ data: {}, error: null }),
            delete: () => Promise.resolve({ error: null })
        }),
        storage: {
            from: () => ({
                upload: () => Promise.resolve({ data: {}, error: null }),
                download: () => Promise.resolve({ data: new Blob(), error: null })
            })
        },
        channel: () => ({
            on: () => ({
                subscribe: () => {}
            })
        }),
        _isOfflineMode: true
    };
}

// Device Management Functions
async function loadDevices() {
    try {
        console.log('📱 Loading devices from Supabase...');
        
        if (!supabaseClient) {
            await initializeSupabase();
        }
        
        // Check if we're in offline mode
        if (supabaseClient._isOfflineMode) {
            console.log('⚠️ Using mock devices in offline mode');
            return getMockDevices();
        }
        
        const { data: devices, error } = await supabaseClient
            .from('remote_devices')
            .select('*')
            .order('last_seen', { ascending: false });
        
        if (error) {
            console.error('❌ Failed to load devices:', error);
            console.warn('⚠️ Using mock devices instead');
            return getMockDevices();
        }
        
        console.log(`✅ Loaded ${devices.length} devices`);
        return devices;
    } catch (error) {
        console.error('❌ Error in loadDevices:', error);
        console.warn('⚠️ Using mock devices instead');
        return getMockDevices();
    }
}

// Generate mock devices for testing when offline
function getMockDevices() {
    const mockDevices = [
        {
            id: 'mock-device-1',
            device_id: 'device_660df7a9d7015dc8',
            device_name: 'Test Windows PC',
            os_type: 'windows',
            status: 'online',
            ip_address: '192.168.1.100',
            last_seen: new Date().toISOString()
        },
        {
            id: 'mock-device-2',
            device_id: 'device_a7b3c9d8e2f1g5h6',
            device_name: 'Test Mac',
            os_type: 'macos',
            status: 'offline',
            ip_address: '192.168.1.101',
            last_seen: new Date(Date.now() - 86400000).toISOString() // 1 day ago
        },
        {
            id: 'mock-device-3',
            device_id: 'device_j8k7l6m5n4o3p2q1',
            device_name: 'Test Linux Server',
            os_type: 'linux',
            status: 'online',
            ip_address: '192.168.1.102',
            last_seen: new Date().toISOString()
        }
    ];
    
    console.log(`✅ Created ${mockDevices.length} mock devices for testing`);
    return mockDevices;
}

async function getDevice(deviceId) {
    try {
        console.log(`🔍 Getting device with ID: ${deviceId}`);
        
        if (!supabaseClient) {
            await initializeSupabase();
        }
        
        // Check if we're in offline mode
        if (supabaseClient._isOfflineMode) {
            console.log(`⚠️ Using mock device in offline mode for ID: ${deviceId}`);
            const mockDevices = getMockDevices();
            const device = mockDevices.find(d => d.device_id === deviceId) || mockDevices[0];
            return device;
        }
        
        const { data: device, error } = await supabaseClient
            .from('remote_devices')
            .select('*')
            .eq('device_id', deviceId)
            .single();

        if (error) {
            console.error(`❌ Failed to get device ${deviceId}:`, error);
            console.warn('⚠️ Using mock device instead');
            const mockDevices = getMockDevices();
            return mockDevices.find(d => d.device_id === deviceId) || mockDevices[0];
        }

        console.log(`✅ Got device: ${device.device_name}`);
        return device;
    } catch (error) {
        console.error(`❌ Error getting device ${deviceId}:`, error);
        console.warn('⚠️ Using mock device instead');
        const mockDevices = getMockDevices();
        return mockDevices.find(d => d.device_id === deviceId) || mockDevices[0];
    }
}

// Realtime Functions
async function setupRealtimeSubscription(deviceId, onScreenFrame, onCommand) {
    try {
        console.log(`📡 Setting up realtime subscription for device: ${deviceId}`);
        
        if (!supabaseClient) {
            await initializeSupabase();
        }
        
        // Create realtime channel
        realtimeChannel = supabaseClient.channel(`device_${deviceId}`);
        
        // Subscribe to screen frames
        realtimeChannel.on('broadcast', { event: 'screen_frame' }, (payload) => {
            console.log('📺 Received screen frame');
            if (onScreenFrame) {
                onScreenFrame(payload.payload);
            }
        });
        
        // Subscribe to commands
        realtimeChannel.on('broadcast', { event: 'command' }, (payload) => {
            console.log('⌨️ Received command');
            if (onCommand) {
                onCommand(payload.payload);
            }
        });
        
        // Subscribe to channel
        await realtimeChannel.subscribe((status) => {
            console.log(`📡 Realtime subscription status: ${status}`);
        });
        
        console.log('✅ Realtime subscription established');
        return realtimeChannel;
    } catch (error) {
        console.error('❌ Failed to setup realtime subscription:', error);
        throw error;
    }
}

async function sendCommand(deviceId, command) {
    try {
        console.log(`📤 Sending command to device ${deviceId}:`, command);
        
        if (!realtimeChannel) {
            throw new Error('Realtime channel not established');
        }
        
        await realtimeChannel.send({
            type: 'broadcast',
            event: 'command',
            payload: {
                device_id: deviceId,
                command: command,
                timestamp: Date.now()
            }
        });
        
        console.log('✅ Command sent successfully');
    } catch (error) {
        console.error('❌ Failed to send command:', error);
        throw error;
    }
}

// Session Management Functions
async function startSession(deviceId, sessionType = 'remote_control') {
    try {
        console.log(`🎮 Starting ${sessionType} session with device: ${deviceId}`);
        
        if (!supabaseClient) {
            await initializeSupabase();
        }
        
        const sessionId = `session_${Date.now()}`;
        
        // Insert session record
        const { data: session, error } = await supabaseClient
            .from('remote_sessions')
            .insert({
                session_id: sessionId,
                device_id: deviceId,
                session_type: sessionType,
                status: 'active',
                started_at: new Date().toISOString()
            })
            .select()
            .single();
        
        if (error) {
            console.error('❌ Failed to start session:', error);
            throw error;
        }
        
        console.log(`✅ Session started: ${sessionId}`);
        return session;
    } catch (error) {
        console.error('❌ Error starting session:', error);
        throw error;
    }
}

async function endSession(sessionId) {
    try {
        console.log(`🛑 Ending session: ${sessionId}`);
        
        if (!supabaseClient) {
            await initializeSupabase();
        }
        
        const { error } = await supabaseClient
            .from('remote_sessions')
            .update({
                status: 'ended',
                ended_at: new Date().toISOString()
            })
            .eq('session_id', sessionId);
        
        if (error) {
            console.error('❌ Failed to end session:', error);
            throw error;
        }
        
        console.log('✅ Session ended successfully');
    } catch (error) {
        console.error('❌ Error ending session:', error);
        throw error;
    }
}

// File Transfer Functions
async function uploadFile(deviceId, file, onProgress) {
    try {
        console.log(`📤 Uploading file to device ${deviceId}: ${file.name}`);
        
        if (!supabaseClient) {
            await initializeSupabase();
        }
        
        const fileName = `${deviceId}/${Date.now()}_${file.name}`;
        
        // Upload file to Supabase Storage
        const { data, error } = await supabaseClient.storage
            .from('file-transfers')
            .upload(fileName, file, {
                onUploadProgress: (progress) => {
                    if (onProgress) {
                        onProgress(progress);
                    }
                }
            });
        
        if (error) {
            console.error('❌ Failed to upload file:', error);
            throw error;
        }
        
        // Record file transfer
        const { error: dbError } = await supabaseClient
            .from('file_transfers')
            .insert({
                device_id: deviceId,
                file_name: file.name,
                file_path: data.path,
                file_size: file.size,
                transfer_type: 'upload',
                status: 'completed'
            });
        
        if (dbError) {
            console.error('❌ Failed to record file transfer:', dbError);
        }
        
        console.log('✅ File uploaded successfully');
        return data;
    } catch (error) {
        console.error('❌ Error uploading file:', error);
        throw error;
    }
}

async function downloadFile(deviceId, fileName) {
    try {
        console.log(`📥 Downloading file from device ${deviceId}: ${fileName}`);
        
        if (!supabaseClient) {
            await initializeSupabase();
        }
        
        const filePath = `${deviceId}/${fileName}`;
        
        // Download file from Supabase Storage
        const { data, error } = await supabaseClient.storage
            .from('file-transfers')
            .download(filePath);
        
        if (error) {
            console.error('❌ Failed to download file:', error);
            throw error;
        }
        
        console.log('✅ File downloaded successfully');
        return data;
    } catch (error) {
        console.error('❌ Error downloading file:', error);
        throw error;
    }
}

// Cleanup function
function cleanup() {
    console.log('🧹 Cleaning up Supabase connections...');
    
    if (realtimeChannel) {
        realtimeChannel.unsubscribe();
        realtimeChannel = null;
    }
    
    console.log('✅ Cleanup completed');
}

// Export functions for use in other modules
window.SupabaseIntegration = {
    initializeSupabase,
    loadDevices,
    getDevice,
    setupRealtimeSubscription,
    sendCommand,
    startSession,
    endSession,
    uploadFile,
    downloadFile,
    cleanup
};

console.log('✅ Supabase Integration Module loaded successfully');
