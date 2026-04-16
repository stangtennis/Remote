// TURN Credentials Edge Function
// Generates time-limited TURN credentials for WebRTC connections

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'
import { hmac } from "https://deno.land/x/hmac@v2.0.1/mod.ts"

const corsHeaders = {
  'Access-Control-Allow-Origin': 'https://dashboard.hawkeye123.dk',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

// TURN server configuration - read from environment (no hardcoded fallbacks)
const TURN_SERVER = Deno.env.get('TURN_SERVER') || ''
const TURN_SECRET = Deno.env.get('TURN_SECRET') || ''
const TURN_TTL = parseInt(Deno.env.get('TURN_TTL') || '3600') // 1 hour default

serve(async (req) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    // Verify user is authenticated
    const authHeader = req.headers.get('Authorization')
    if (!authHeader) {
      return new Response(
        JSON.stringify({ error: 'Missing authorization header' }),
        { status: 401, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Create Supabase client to verify token
    const supabaseUrl = Deno.env.get('SUPABASE_URL') || ''
    const supabaseKey = Deno.env.get('SUPABASE_ANON_KEY') || ''
    const supabase = createClient(supabaseUrl, supabaseKey, {
      global: { headers: { Authorization: authHeader } }
    })

    // Verify user session
    const { data: { user }, error: userError } = await supabase.auth.getUser()
    if (userError || !user) {
      return new Response(
        JSON.stringify({ error: 'Invalid or expired token' }),
        { status: 401, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Try Cloudflare TURN first (managed, 1000 GB/month free)
    const CF_TURN_KEY_ID = Deno.env.get('CF_TURN_KEY_ID') || ''
    const CF_TURN_API_TOKEN = Deno.env.get('CF_TURN_API_TOKEN') || ''

    // Collect all ICE servers (Cloudflare + coturn)
    let allIceServers: any[] = [
      { urls: ['stun:stun.cloudflare.com:3478', 'stun:stun.l.google.com:19302'] },
    ]
    let provider = 'stun-only'

    // 1) Cloudflare managed TURN
    if (CF_TURN_KEY_ID && CF_TURN_API_TOKEN) {
      try {
        const cfResp = await fetch(
          `https://rtc.live.cloudflare.com/v1/turn/keys/${CF_TURN_KEY_ID}/credentials/generate-ice-servers`,
          {
            method: 'POST',
            headers: {
              'Authorization': `Bearer ${CF_TURN_API_TOKEN}`,
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ ttl: 86400 }),
          }
        )
        if (cfResp.ok) {
          const cfData = await cfResp.json()
          // Add Cloudflare ICE servers (skip their STUN, we already have it)
          for (const srv of cfData.iceServers || []) {
            const urls = Array.isArray(srv.urls) ? srv.urls : [srv.urls]
            const turnUrls = urls.filter((u: string) => u.startsWith('turn'))
            if (turnUrls.length > 0 && srv.username) {
              allIceServers.push({ urls: turnUrls, username: srv.username, credential: srv.credential })
            }
          }
          provider = 'cloudflare'
        } else {
          console.error('Cloudflare TURN failed:', cfResp.status)
        }
      } catch (e) {
        console.error('Cloudflare TURN error:', e)
      }
    }

    // 2) Coturn fallback — disabled for now (relay ports 49200-49300 not forwarded on router)
    // Re-enable when router port forwarding is set up
    // if (TURN_SERVER && TURN_SECRET) { ... }

    return new Response(
      JSON.stringify({
        iceServers: allIceServers,
        ttl: TURN_TTL,
        expires: Math.floor(Date.now() / 1000) + TURN_TTL,
        provider,
      }),
      {
        status: 200,
        headers: { ...corsHeaders, 'Content-Type': 'application/json' }
      }
    )

  } catch (error) {
    console.error('Error generating TURN credentials:', error)
    return new Response(
      JSON.stringify({ error: 'Internal server error' }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
})
