const { createClient } = require('@supabase/supabase-js');

const supabase = createClient(
    'https://ptrtibzwokjcjjxvjpin.supabase.co', 
    'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk'
);

async function checkDevices() {
    try {
        console.log('🔍 Checking current devices in database...');
        
        const { data, error } = await supabase
            .from('remote_devices')
            .select('*')
            .order('last_seen', { ascending: false });
            
        if (error) {
            console.log('❌ Error:', error.message);
            return;
        }
        
        console.log(`📊 Found ${data.length} devices in database:`);
        console.log('');
        
        data.forEach((device, index) => {
            const status = device.is_online ? '🟢 Online' : '🔴 Offline';
            const lastSeen = new Date(device.last_seen).toLocaleString();
            const timeDiff = Math.round((Date.now() - new Date(device.last_seen)) / 1000 / 60); // minutes ago
            
            console.log(`${index + 1}. ${device.device_name}`);
            console.log(`   ID: ${device.device_id}`);
            console.log(`   Status: ${status}`);
            console.log(`   Last seen: ${lastSeen} (${timeDiff} minutes ago)`);
            console.log(`   IP: ${device.local_ip || 'N/A'}:${device.local_port || 'N/A'}`);
            console.log('');
        });
        
        // Check for stale online devices (online but not seen for > 5 minutes)
        const staleDevices = data.filter(d => d.is_online && (Date.now() - new Date(d.last_seen)) > 5 * 60 * 1000);
        if (staleDevices.length > 0) {
            console.log('⚠️  STALE DEVICES DETECTED (online but inactive > 5 min):');
            staleDevices.forEach(device => {
                const timeDiff = Math.round((Date.now() - new Date(device.last_seen)) / 1000 / 60);
                console.log(`   - ${device.device_name}: last seen ${timeDiff} minutes ago`);
            });
        }
        
    } catch (error) {
        console.error('❌ Script error:', error.message);
    }
}

checkDevices();
