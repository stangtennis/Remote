import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const allowedOrigins = ['https://dashboard.hawkeye123.dk', 'https://supabase.hawkeye123.dk']

function getCorsHeaders(req: Request) {
  const origin = req.headers.get('origin') || ''
  return {
    'Access-Control-Allow-Origin': allowedOrigins.includes(origin) ? origin : allowedOrigins[0],
    'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
  }
}

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

// Beacon endpoint for reliable session cleanup during page unload.
// No auth required — sendBeacon cannot send custom headers.
// Session IDs are validated as UUIDs to prevent injection.
serve(async (req) => {
  const corsHeaders = getCorsHeaders(req)

  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    const { session_ids } = await req.json()

    if (!Array.isArray(session_ids) || session_ids.length === 0) {
      return new Response(
        JSON.stringify({ error: 'session_ids array required' }),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 400 }
      )
    }

    // Limit to 20 session IDs per request and validate UUID format
    const ids = session_ids.slice(0, 20).filter((id: string) => UUID_REGEX.test(id))
    if (ids.length === 0) {
      return new Response(
        JSON.stringify({ error: 'no valid UUIDs provided' }),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 400 }
      )
    }

    const supabaseUrl = Deno.env.get('SUPABASE_URL')!
    const supabaseKey = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!
    const supabase = createClient(supabaseUrl, supabaseKey)

    const now = new Date().toISOString()

    // Mark sessions as ended (only if currently pending/active)
    const { data, error } = await supabase
      .from('remote_sessions')
      .update({ status: 'ended', ended_at: now })
      .in('id', ids)
      .in('status', ['pending', 'active'])
      .select('id')

    if (error) {
      console.error('Beacon cleanup error:', error)
      return new Response(
        JSON.stringify({ error: error.message }),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 500 }
      )
    }

    console.log(`🧹 Beacon cleanup: ended ${data?.length || 0} sessions`)

    return new Response(
      JSON.stringify({ success: true, ended: data?.length || 0 }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 200 }
    )

  } catch (error) {
    console.error('Beacon cleanup error:', error)
    return new Response(
      JSON.stringify({ error: error.message }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 500 }
    )
  }
})
