const { createClient } = require('@supabase/supabase-js');
const fs = require('fs');
const path = require('path');

// Supabase configuration
const supabaseUrl = 'https://ptrtibzwokjcjjxvjpin.supabase.co';
const supabaseKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3MjU1NTI2NzIsImV4cCI6MjA0MTEyODY3Mn0.LfJVxKBKJRNBPnfJKxANZXBOLJCqWKnNZBKjGdKkL6E';

const supabase = createClient(supabaseUrl, supabaseKey);

async function uploadAgent() {
    try {
        console.log('🚀 Starting agent upload to Supabase Storage...');
        
        // Read the agent file
        const agentPath = path.join(__dirname, 'public', 'RemoteDesktopAgent.exe');
        const agentFile = fs.readFileSync(agentPath);
        
        console.log(`📁 Reading agent file: ${agentPath}`);
        console.log(`📊 File size: ${(agentFile.length / 1024 / 1024).toFixed(2)} MB`);
        
        // Upload to Supabase Storage
        const { data, error } = await supabase.storage
            .from('agents')
            .upload('RemoteDesktopAgent.exe', agentFile, {
                cacheControl: '3600',
                upsert: true,
                contentType: 'application/octet-stream'
            });
        
        if (error) {
            console.error('❌ Upload failed:', error.message);
            return;
        }
        
        console.log('✅ Agent uploaded successfully!');
        console.log('📍 Storage path:', data.path);
        console.log('🌍 Public URL: https://ptrtibzwokjcjjxvjpin.supabase.co/storage/v1/object/public/agents/RemoteDesktopAgent.exe');
        
        // Verify the upload
        const { data: listData, error: listError } = await supabase.storage
            .from('agents')
            .list('', { limit: 10 });
            
        if (listError) {
            console.error('❌ Failed to verify upload:', listError.message);
        } else {
            const agentFile = listData.find(file => file.name === 'RemoteDesktopAgent.exe');
            if (agentFile) {
                console.log('✅ Upload verified - file exists in storage');
                console.log(`📊 Stored file size: ${(agentFile.metadata?.size / 1024 / 1024).toFixed(2)} MB`);
            } else {
                console.log('⚠️ File not found in storage listing');
            }
        }
        
    } catch (error) {
        console.error('❌ Upload script failed:', error.message);
    }
}

// Run the upload
uploadAgent();
