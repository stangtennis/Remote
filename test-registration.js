#!/usr/bin/env node

/**
 * Minimal Test Agent - Only Registration
 * Purpose: Debug why agent doesn't appear in dashboard
 */

const os = require('os');
const crypto = require('crypto');
const { createClient } = require('@supabase/supabase-js');

// Supabase configuration
const supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const supabaseKey = 'REDACTED_JWT';

// Generate hardware-based device ID
function generateHardwareBasedDeviceId() {
    try {
        const hostname = os.hostname() || 'unknown';
        const platform = os.platform();
        const arch = os.arch();
        const cpus = os.cpus().length.toString();
        const totalMem = Math.round(os.totalmem() / (1024 * 1024 * 1024)).toString();
        
        // Get MAC address
        let macAddress = 'unknown';
        const interfaces = os.networkInterfaces();
        for (const interfaceName in interfaces) {
            const iface = interfaces[interfaceName];
            for (const alias of iface) {
                if (!alias.internal && alias.mac && alias.mac !== '00:00:00:00:00:00') {
                    macAddress = alias.mac;
                    break;
                }
            }
            if (macAddress !== 'unknown') break;
        }
        
        // Create consistent hash
        const hardwareString = `${hostname}-${platform}-${arch}-${cpus}-${totalMem}-${macAddress}`;
        const hash = crypto.createHash('sha256').update(hardwareString).digest('hex');
        
        // Format as UUID
        const uuid = [
            hash.substring(0, 8),
            hash.substring(8, 12),
            hash.substring(12, 16),
            hash.substring(16, 20),
            hash.substring(20, 32)
        ].join('-');
        
        return uuid;
    } catch (error) {
        console.error('❌ Error generating device ID:', error.message);
        return crypto.randomUUID();
    }
}

function getLocalIP() {
    const interfaces = os.networkInterfaces();
    for (const interfaceName in interfaces) {
        const iface = interfaces[interfaceName];
        for (const alias of iface) {
            if (alias.family === 'IPv4' && !alias.internal) {
                return alias.address;
            }
        }
    }
    return '127.0.0.1';
}

async function testRegistration() {
    console.log('🧪 Testing Agent Registration');
    console.log('================================');
    
    // Generate device info
    const deviceId = generateHardwareBasedDeviceId();
    const deviceName = os.hostname() || 'TestPC';
    
    console.log(`📱 Device ID: ${deviceId}`);
    console.log(`📱 Device Name: ${deviceName}`);
    console.log(`📱 Platform: ${os.platform()}`);
    console.log(`📱 IP Address: ${getLocalIP()}`);
    console.log('');
    
    // Initialize Supabase client
    const supabase = createClient(supabaseUrl, supabaseKey);
    console.log('✅ Supabase client initialized');
    
    // Test 1: Check if remote_devices table exists and what's in it
    console.log('🔍 Test 1: Checking current remote_devices table...');
    try {
        const { data: existingDevices, error: selectError } = await supabase
            .from('remote_devices')
            .select('*');
            
        if (selectError) {
            console.error('❌ Error reading remote_devices:', selectError.message);
        } else {
            console.log(`✅ Found ${existingDevices.length} existing devices:`);
            existingDevices.forEach(device => {
                console.log(`   - ${device.device_name} (${device.id}) - Status: ${device.status || device.is_online}`);
            });
        }
    } catch (error) {
        console.error('❌ Test 1 failed:', error.message);
    }
    
    console.log('');
    
    // Test 2: Update existing Dennis device to be our agent
    console.log('🔍 Test 2: Updating existing Dennis device...');
    try {
        // Update only the fields that definitely exist
        const updateData = {
            device_name: deviceName,
            status: 'online',
            last_seen: new Date().toISOString()
        };
        
        console.log('📝 Attempting to update Dennis device with data:');
        console.log(JSON.stringify(updateData, null, 2));
        
        const { data, error } = await supabase
            .from('remote_devices')
            .update(updateData)
            .eq('device_name', 'Dennis')
            .select();
            
        if (error) {
            console.error('❌ Registration failed:', error.message);
            console.error('❌ Error details:', error);
        } else {
            console.log('✅ Registration successful!');
            console.log('✅ Registered data:', JSON.stringify(data, null, 2));
        }
    } catch (error) {
        console.error('❌ Test 2 failed:', error.message);
    }
    
    console.log('');
    
    // Test 3: Verify registration by reading table again
    console.log('🔍 Test 3: Verifying registration...');
    try {
        const { data: updatedDevices, error: verifyError } = await supabase
            .from('remote_devices')
            .select('*')
            .order('last_seen', { ascending: false });
            
        if (verifyError) {
            console.error('❌ Error verifying registration:', verifyError.message);
        } else {
            console.log(`✅ Now found ${updatedDevices.length} devices:`);
            updatedDevices.forEach(device => {
                console.log(`   - ${device.device_name} (${device.id}) - Status: ${device.status || device.is_online}`);
                if (device.id === deviceId) {
                    console.log('   🎉 OUR DEVICE IS REGISTERED!');
                }
            });
        }
    } catch (error) {
        console.error('❌ Test 3 failed:', error.message);
    }
    
    console.log('');
    console.log('🧪 Test completed!');
}

// Run the test
testRegistration().catch(console.error);
