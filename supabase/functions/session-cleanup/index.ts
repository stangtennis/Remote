import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

serve(async (req) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    // Create Supabase client with service role for admin access
    const supabaseUrl = Deno.env.get('SUPABASE_URL')!
    const supabaseKey = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!
    const supabase = createClient(supabaseUrl, supabaseKey)

    const now = new Date()
    const oneMinuteAgo = new Date(now.getTime() - 60 * 1000)
    const fifteenMinutesAgo = new Date(now.getTime() - 15 * 60 * 1000)
    const twentyFourHoursAgo = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    console.log('🧹 Starting session cleanup...')

    // 1. Clean up old signaling messages (older than 1 minute)
    const { data: oldSignals, error: signalError } = await supabase
      .from('session_signaling')
      .delete()
      .lt('created_at', oneMinuteAgo.toISOString())

    if (signalError) {
      console.error('Error cleaning signaling:', signalError)
    } else {
      console.log(`✅ Cleaned up signaling messages older than 1 minute`)
    }

    // 2. Expire old pending/active sessions (older than 15 minutes)
    const { data: expiredSessions, error: sessionError } = await supabase
      .from('remote_sessions')
      .update({ 
        status: 'expired', 
        ended_at: now.toISOString() 
      })
      .in('status', ['pending', 'active'])
      .lt('created_at', fifteenMinutesAgo.toISOString())
      .select('id')

    if (sessionError) {
      console.error('Error expiring sessions:', sessionError)
    } else {
      console.log(`✅ Expired ${expiredSessions?.length || 0} old sessions`)
    }

    // 3. Delete really old expired/ended sessions (older than 24 hours) to keep DB clean
    const { data: deletedSessions, error: deleteError } = await supabase
      .from('remote_sessions')
      .delete()
      .in('status', ['expired', 'ended'])
      .lt('created_at', twentyFourHoursAgo.toISOString())
      .select('id')

    if (deleteError) {
      console.error('Error deleting old sessions:', deleteError)
    } else {
      console.log(`✅ Deleted ${deletedSessions?.length || 0} old completed sessions`)
    }

    // 4. Update offline status for devices that haven't been seen in 2 minutes
    const twoMinutesAgo = new Date(now.getTime() - 2 * 60 * 1000)
    const { data: offlineDevices, error: deviceError } = await supabase
      .from('remote_devices')
      .update({ is_online: false })
      .eq('is_online', true)
      .lt('last_seen', twoMinutesAgo.toISOString())
      .select('device_id')

    if (deviceError) {
      console.error('Error updating offline devices:', deviceError)
    } else {
      console.log(`✅ Marked ${offlineDevices?.length || 0} devices as offline`)
    }

    const summary = {
      timestamp: now.toISOString(),
      signaling_cleaned: true,
      sessions_expired: expiredSessions?.length || 0,
      sessions_deleted: deletedSessions?.length || 0,
      devices_offline: offlineDevices?.length || 0
    }

    console.log('✅ Cleanup completed:', summary)

    return new Response(
      JSON.stringify({ success: true, ...summary }),
      { 
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
        status: 200 
      }
    )

  } catch (error) {
    console.error('❌ Cleanup error:', error)
    return new Response(
      JSON.stringify({ error: error.message }),
      { 
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
        status: 500
      }
    )
  }
})
