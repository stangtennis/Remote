// TURN Credentials Edge Function
// Generates time-limited TURN credentials for WebRTC connections

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'
import { hmac } from "https://deno.land/x/hmac@v2.0.1/mod.ts"

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
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

    // If TURN server is not configured, return STUN-only
    if (!TURN_SERVER || !TURN_SECRET) {
      return new Response(
        JSON.stringify({
          iceServers: [
            { urls: 'stun:stun.l.google.com:19302' },
            { urls: 'stun:stun1.l.google.com:19302' },
          ],
          ttl: TURN_TTL,
          expires: Math.floor(Date.now() / 1000) + TURN_TTL
        }),
        { 
          status: 200, 
          headers: { ...corsHeaders, 'Content-Type': 'application/json' } 
        }
      )
    }

    // Generate time-limited credentials using TURN REST API format
    // Username format: timestamp:username (coturn style)
    const timestamp = Math.floor(Date.now() / 1000) + TURN_TTL
    const username = `${timestamp}:${user.id}`
    
    // Generate HMAC-SHA1 credential (coturn compatible)
    const encoder = new TextEncoder()
    const key = encoder.encode(TURN_SECRET)
    const message = encoder.encode(username)
    const signature = await crypto.subtle.importKey(
      'raw', key, { name: 'HMAC', hash: 'SHA-1' }, false, ['sign']
    )
    const sig = await crypto.subtle.sign('HMAC', signature, message)
    const credential = btoa(String.fromCharCode(...new Uint8Array(sig)))

    // Return ICE servers configuration
    const iceServers = [
      { urls: 'stun:stun.l.google.com:19302' },
      { urls: 'stun:stun1.l.google.com:19302' },
      {
        urls: TURN_SERVER,
        username: username,
        credential: credential
      },
      {
        urls: `${TURN_SERVER}?transport=tcp`,
        username: username,
        credential: credential
      }
    ]

    return new Response(
      JSON.stringify({
        iceServers,
        ttl: TURN_TTL,
        expires: timestamp
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
