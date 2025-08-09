const { createClient } = require('@supabase/supabase-js');

// Supabase configuration
const supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const supabaseServiceKey = 'REDACTED_JWT'; // Service role key

const supabase = createClient(supabaseUrl, supabaseServiceKey);

async function cleanupDevices() {
    try {
        console.log('🧹 Starting device cleanup...');
        
        // First, let's see what devices exist
        console.log('📋 Checking current devices in database...');
        const { data: devices, error: fetchError } = await supabase
            .from('remote_devices')
            .select('*');
            
        if (fetchError) {
            console.error('❌ Error fetching devices:', fetchError);
            return;
        }
        
        console.log(`📊 Found ${devices.length} devices in database:`);
        devices.forEach(device => {
            console.log(`   - ${device.device_name} (${device.id}) - Status: ${device.status} - Last seen: ${device.last_seen}`);
        });
        
        // Remove all old devices (they all have old random UUIDs)
        console.log('\n🗑️ Removing all old devices...');
        const { data: deletedDevices, error: deleteError } = await supabase
            .from('remote_devices')
            .delete()
            .in('id', devices.map(d => d.id)) // Delete all existing devices
            .select();
            
        if (deleteError) {
            console.error('❌ Error deleting devices:', deleteError);
            return;
        }
        
        console.log(`✅ Successfully removed ${deletedDevices ? deletedDevices.length : 0} old devices`);
        
        // Check final state
        console.log('\n📋 Final device list:');
        const { data: finalDevices, error: finalError } = await supabase
            .from('remote_devices')
            .select('*');
            
        if (finalError) {
            console.error('❌ Error fetching final devices:', finalError);
            return;
        }
        
        console.log(`📊 Remaining devices: ${finalDevices.length}`);
        finalDevices.forEach(device => {
            console.log(`   - ${device.device_name} (${device.id}) - Status: ${device.status}`);
        });
        
        console.log('\n🎉 Device cleanup completed successfully!');
        
    } catch (error) {
        console.error('❌ Cleanup failed:', error.message);
    }
}

// Run cleanup
cleanupDevices();
