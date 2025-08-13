const { createClient } = require('@supabase/supabase-js');
const os = require('os');
const crypto = require('crypto');

const supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const supabaseKey = 'REDACTED_JWT';

function generateDeviceId() {
    const hardwareInfo = [
        os.hostname(),
        os.platform(),
        os.arch(),
        os.cpus()[0].model,
        os.totalmem().toString()
    ].join('|');
    
    const hash = crypto.createHash('sha256').update(hardwareInfo).digest('hex');
    const uuid = [
        hash.substr(0, 8),
        hash.substr(8, 4),
        hash.substr(12, 4),
        hash.substr(16, 4),
        hash.substr(20, 12)
    ].join('-');
    
    return uuid;
}

async function testRegistration() {
    console.log('🔍 DEBUG: Testing direct database registration...');
    
    const supabase = createClient(supabaseUrl, supabaseKey);
    const deviceName = os.hostname() || 'TestAgent';
    const deviceId = generateDeviceId();
    
    console.log(`📱 Device Name: ${deviceName}`);
    console.log(`🆔 Device ID: ${deviceId}`);
    
    // Test minimal device data with correct column names and required fields
    const deviceData = {
        device_name: deviceName,
        device_id: deviceId,
        is_online: true,
        last_seen: new Date().toISOString()
    };
    
    console.log('📊 Registration data:', JSON.stringify(deviceData, null, 2));
    
    try {
        console.log('🔄 Attempting direct database insert...');
        const { data, error } = await supabase
            .from('remote_devices')
            .insert(deviceData)
            .select();
            
        if (error) {
            console.error('❌ INSERT FAILED:');
            console.error('❌ Error message:', error.message);
            console.error('❌ Error details:', JSON.stringify(error, null, 2));
            console.error('❌ Error code:', error.code);
            console.error('❌ Error hint:', error.hint);
        } else {
            console.log('✅ INSERT SUCCESS!');
            console.log('📊 Inserted data:', JSON.stringify(data, null, 2));
        }
        
    } catch (exception) {
        console.error('❌ EXCEPTION CAUGHT:');
        console.error('❌ Exception message:', exception.message);
        console.error('❌ Exception stack:', exception.stack);
    }
    
    // Test database query to see what's actually in there
    console.log('🔍 Checking database contents...');
    try {
        const { data: allDevices, error: queryError } = await supabase
            .from('remote_devices')
            .select('*');
            
        if (queryError) {
            console.error('❌ QUERY FAILED:', queryError.message);
        } else {
            console.log('📊 Database contents:', JSON.stringify(allDevices, null, 2));
        }
    } catch (exception) {
        console.error('❌ QUERY EXCEPTION:', exception.message);
    }
}

testRegistration().then(() => {
    console.log('🔍 DEBUG: Registration test completed');
    process.exit(0);
}).catch((error) => {
    console.error('❌ FATAL ERROR:', error.message);
    process.exit(1);
});
