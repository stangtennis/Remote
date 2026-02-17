// Support Signal Edge Function
// POST, no auth required - validates via support token.
// Actions: validate, ready, turn

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

// TURN server configuration
const TURN_SERVER = Deno.env.get('TURN_SERVER') || ''
const TURN_SECRET = Deno.env.get('TURN_SECRET') || ''
const TURN_TTL = parseInt(Deno.env.get('TURN_TTL') || '3600')

serve(async (req) => {
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    const { action, token, pin } = await req.json()

    if (!action) {
      throw new Error('action is required')
    }

    // Use service role to bypass RLS
    const supabase = createClient(
      Deno.env.get('SUPABASE_URL')!,
      Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!,
    )

    // Validate token or PIN
    let session
    if (token) {
      const { data, error } = await supabase
        .from('support_sessions')
        .select('*')
        .eq('token', token)
        .in('status', ['pending', 'active'])
        .single()

      if (error || !data) {
        throw new Error('Invalid or expired support token')
      }

      // Check expiry
      if (new Date(data.expires_at) < new Date()) {
        await supabase
          .from('support_sessions')
          .update({ status: 'expired' })
          .eq('id', data.id)
        throw new Error('Support session has expired')
      }

      session = data
    } else if (pin) {
      // PIN-based lookup: find most recent pending/active session with this PIN
      const { data, error } = await supabase
        .from('support_sessions')
        .select('*')
        .eq('pin', pin)
        .in('status', ['pending', 'active'])
        .order('created_at', { ascending: false })
        .limit(1)
        .single()

      if (error || !data) {
        throw new Error('Invalid PIN or no active session found')
      }

      if (new Date(data.expires_at) < new Date()) {
        await supabase
          .from('support_sessions')
          .update({ status: 'expired' })
          .eq('id', data.id)
        throw new Error('Support session has expired')
      }

      session = data
    } else {
      throw new Error('token or pin is required')
    }

    switch (action) {
      case 'validate': {
        return new Response(
          JSON.stringify({
            session_id: session.id,
            token: session.token,
            status: session.status,
            expires_at: session.expires_at,
          }),
          {
            headers: { ...corsHeaders, 'Content-Type': 'application/json' },
            status: 200,
          }
        )
      }

      case 'ready': {
        // Update session status to active
        await supabase
          .from('support_sessions')
          .update({ status: 'active' })
          .eq('id', session.id)

        // Insert ready signal so dashboard knows sharer is ready
        await supabase
          .from('session_signaling')
          .insert({
            session_id: session.id,
            from_side: 'support',
            msg_type: 'answer',
            payload: { type: 'ready' },
          })

        return new Response(
          JSON.stringify({ ok: true, session_id: session.id }),
          {
            headers: { ...corsHeaders, 'Content-Type': 'application/json' },
            status: 200,
          }
        )
      }

      case 'turn': {
        // Generate TURN credentials (same logic as turn-credentials/index.ts)
        if (!TURN_SERVER || !TURN_SECRET) {
          return new Response(
            JSON.stringify({
              iceServers: [
                { urls: 'stun:stun.l.google.com:19302' },
                { urls: 'stun:stun1.l.google.com:19302' },
              ],
              ttl: TURN_TTL,
              expires: Math.floor(Date.now() / 1000) + TURN_TTL,
            }),
            {
              headers: { ...corsHeaders, 'Content-Type': 'application/json' },
              status: 200,
            }
          )
        }

        const timestamp = Math.floor(Date.now() / 1000) + TURN_TTL
        const username = `${timestamp}:support-${session.id}`

        const encoder = new TextEncoder()
        const key = encoder.encode(TURN_SECRET)
        const message = encoder.encode(username)
        const cryptoKey = await crypto.subtle.importKey(
          'raw', key, { name: 'HMAC', hash: 'SHA-1' }, false, ['sign']
        )
        const sig = await crypto.subtle.sign('HMAC', cryptoKey, message)
        const credential = btoa(String.fromCharCode(...new Uint8Array(sig)))

        const iceServers = [
          { urls: 'stun:stun.l.google.com:19302' },
          { urls: 'stun:stun1.l.google.com:19302' },
          {
            urls: TURN_SERVER,
            username: username,
            credential: credential,
          },
          {
            urls: `${TURN_SERVER}?transport=tcp`,
            username: username,
            credential: credential,
          },
        ]

        return new Response(
          JSON.stringify({ iceServers, ttl: TURN_TTL, expires: timestamp }),
          {
            headers: { ...corsHeaders, 'Content-Type': 'application/json' },
            status: 200,
          }
        )
      }

      default:
        throw new Error(`Unknown action: ${action}`)
    }
  } catch (error) {
    return new Response(
      JSON.stringify({ error: error.message }),
      {
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
        status: 400,
      }
    )
  }
})
