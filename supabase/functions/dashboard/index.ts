import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

serve(async (req) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }
  
  // This is a public dashboard - no authentication required
  try {

  const dashboardHtml = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>🌍 Remote Desktop Control Center - Supabase Hosted</title>
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
        .success-banner {
            background: linear-gradient(135deg, #10b981 0%, #059669 100%);
            color: white; padding: 15px; text-align: center; font-weight: 600;
            box-shadow: 0 4px 20px rgba(16, 185, 129, 0.3);
        }
        .main-content {
            max-width: 1200px; margin: 0 auto; padding: 40px 20px;
            text-align: center;
        }
        .migration-card {
            background: rgba(255,255,255,0.95); backdrop-filter: blur(10px);
            border-radius: 20px; padding: 40px; box-shadow: 0 8px 32px rgba(0,0,0,0.1);
            margin-bottom: 30px;
        }
        .feature-grid {
            display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 20px; margin-top: 30px;
        }
        .feature-item {
            background: #f8fafc; border-radius: 15px; padding: 20px;
            border-left: 4px solid #4f46e5;
        }
        .feature-item h4 { color: #1f2937; margin-bottom: 10px; }
        .feature-item p { color: #6b7280; font-size: 0.9rem; }
        .cta-buttons {
            display: flex; gap: 20px; justify-content: center; margin-top: 30px;
            flex-wrap: wrap;
        }
        .btn {
            padding: 15px 30px; border: none; border-radius: 10px; cursor: pointer;
            font-size: 16px; font-weight: 600; transition: all 0.3s ease;
            text-decoration: none; display: inline-block;
        }
        .btn-primary {
            background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
            color: white;
        }
        .btn-secondary {
            background: linear-gradient(135deg, #10b981 0%, #059669 100%);
            color: white;
        }
        .btn:hover { transform: translateY(-2px); box-shadow: 0 10px 25px rgba(0,0,0,0.2); }
    </style>
</head>
<body>
    <div class="success-banner">
        🎉 Successfully Migrated to Supabase! Fully Serverless Remote Desktop System
    </div>
    
    <div class="header">
        <div class="header-content">
            <h1>🌍 Remote Desktop Control Center</h1>
            <div style="color: #10b981; font-weight: 600;">
                ✅ Hosted on Supabase Edge Functions
            </div>
        </div>
    </div>

    <div class="main-content">
        <div class="migration-card">
            <h2 style="color: #1f2937; margin-bottom: 20px;">
                🚀 Full Supabase Migration Complete!
            </h2>
            <p style="color: #6b7280; font-size: 1.1rem; margin-bottom: 30px;">
                Your remote desktop system is now fully serverless and globally accessible.
                No more local servers - everything runs on Supabase infrastructure!
            </p>
            
            <div class="feature-grid">
                <div class="feature-item">
                    <h4>🌐 Global Dashboard</h4>
                    <p>Hosted on Supabase Edge Functions with worldwide CDN</p>
                </div>
                <div class="feature-item">
                    <h4>📊 Real-time Database</h4>
                    <p>PostgreSQL with real-time subscriptions for live updates</p>
                </div>
                <div class="feature-item">
                    <h4>⚡ Serverless Backend</h4>
                    <p>Edge Functions handle all API logic without servers</p>
                </div>
                <div class="feature-item">
                    <h4>📁 Cloud Storage</h4>
                    <p>EXE files and assets hosted on Supabase Storage</p>
                </div>
                <div class="feature-item">
                    <h4>🔐 Built-in Auth</h4>
                    <p>Supabase Authentication for secure admin access</p>
                </div>
                <div class="feature-item">
                    <h4>🔄 Auto-scaling</h4>
                    <p>Handles unlimited devices with automatic scaling</p>
                </div>
            </div>
            
            <div class="cta-buttons">
                <a href="/functions/v1/agent-generator" class="btn btn-primary">
                    📥 Download Agents
                </a>
                <a href="/functions/v1/device-manager" class="btn btn-secondary">
                    🖥️ Manage Devices
                </a>
            </div>
        </div>
        
        <div style="background: rgba(255,255,255,0.1); border-radius: 15px; padding: 20px; color: white;">
            <h3 style="margin-bottom: 15px;">🎯 System Status</h3>
            <p>✅ Dashboard: Hosted on Supabase Edge Functions</p>
            <p>✅ Database: PostgreSQL with real-time subscriptions</p>
            <p>✅ Storage: EXE files hosted on Supabase Storage</p>
            <p>✅ Authentication: Supabase Auth integration</p>
            <p>✅ Global CDN: Worldwide fast access</p>
        </div>
    </div>

    <script src="https://unpkg.com/@supabase/supabase-js@2"></script>
    <script>
        console.log('🌍 Supabase-hosted Remote Desktop Control Center loaded');
        console.log('✅ Migration to full serverless architecture complete');
        
        // Initialize Supabase client
        const SUPABASE_URL = window.location.origin.replace('/functions/v1/dashboard', '');
        const SUPABASE_ANON_KEY = '${Deno.env.get('SUPABASE_ANON_KEY') || 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InB0cnRpYnp3b2tqY2pqeHZqcGluIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTQ0MzE1NzEsImV4cCI6MjA3MDAwNzU3MX0.DPNxkQul1-13tqJ89mqYJAx7NSJjabOP4q8c6KgOnWk'}';
        
        const { createClient } = supabase;
        const supabaseClient = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
        
        console.log('🔗 Supabase client initialized:', SUPABASE_URL);
        
        // Test connection
        async function testConnection() {
            try {
                const { data, error } = await supabaseClient
                    .from('remote_devices')
                    .select('count')
                    .limit(1);
                
                if (error) {
                    console.error('❌ Connection test failed:', error);
                } else {
                    console.log('✅ Supabase connection successful');
                }
            } catch (err) {
                console.error('❌ Connection error:', err);
            }
        }
        
        testConnection();
    </script>
</body>
</html>`;

    return new Response(dashboardHtml, {
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
