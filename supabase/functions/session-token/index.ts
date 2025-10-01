// session-token Edge Function
// Purpose: Create a new remote session with token, PIN, and TURN credentials

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

interface SessionRequest {
  device_id: string;
  use_pin?: boolean;
}

serve(async (req) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    // Get authenticated user
    const authHeader = req.headers.get('Authorization')
    if (!authHeader) {
      throw new Error('Missing authorization header')
    }

    const supabaseClient = createClient(
      Deno.env.get('SUPABASE_URL') ?? '',
      Deno.env.get('SUPABASE_ANON_KEY') ?? '',
      {
        global: {
          headers: { Authorization: authHeader },
        },
      }
    )

    const {
      data: { user },
      error: userError,
    } = await supabaseClient.auth.getUser()

    if (userError || !user) {
      throw new Error('Unauthorized')
    }

    // Parse request body
    const { device_id, use_pin = true }: SessionRequest = await req.json()

    if (!device_id) {
      throw new Error('device_id is required')
    }

    // Verify device exists and user owns it
    const { data: device, error: deviceError } = await supabaseClient
      .from('remote_devices')
      .select('device_id, is_online, owner_id')
      .eq('device_id', device_id)
      .single()

    if (deviceError || !device) {
      throw new Error('Device not found')
    }

    if (device.owner_id !== user.id) {
      throw new Error('You do not own this device')
    }

    if (!device.is_online) {
      throw new Error('Device is offline')
    }

    // Generate session token (JWT-style random string)
    const token = crypto.randomUUID() + '-' + Date.now()
    
    // Generate PIN (6 digits)
    const pin = use_pin 
      ? Math.floor(100000 + Math.random() * 900000).toString()
      : null

    // Session expires in 15 minutes
    const expires_at = new Date(Date.now() + 15 * 60 * 1000).toISOString()

    // Create session
    const { data: session, error: sessionError } = await supabaseClient
      .from('remote_sessions')
      .insert({
        device_id,
        created_by: user.id,
        status: 'pending',
        token,
        pin,
        expires_at,
      })
      .select()
      .single()

    if (sessionError) {
      throw sessionError
    }

    // Get TURN credentials (Twilio example)
    const turnConfig = await getTurnCredentials()

    // Log audit event
    await supabaseClient.rpc('log_audit_event', {
      p_session_id: session.id,
      p_device_id: device_id,
      p_event: 'SESSION_CREATED',
      p_details: { pin_used: use_pin },
      p_severity: 'info',
    })

    return new Response(
      JSON.stringify({
        session_id: session.id,
        token,
        pin,
        expires_at,
        turn_config: turnConfig,
      }),
      {
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
        status: 200,
      }
    )
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

async function getTurnCredentials() {
  const provider = Deno.env.get('TURN_PROVIDER') || 'twilio'

  if (provider === 'twilio') {
    // Twilio TURN credentials
    const accountSid = Deno.env.get('TWILIO_ACCOUNT_SID')
    const authToken = Deno.env.get('TWILIO_AUTH_TOKEN')

    if (!accountSid || !authToken) {
      console.warn('TURN credentials not configured, returning STUN only')
      return {
        iceServers: [
          { urls: 'stun:stun.l.google.com:19302' },
          { urls: 'stun:stun1.l.google.com:19302' },
        ],
      }
    }

    try {
      const response = await fetch(
        `https://api.twilio.com/2010-04-01/Accounts/${accountSid}/Tokens.json`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
            'Authorization': 'Basic ' + btoa(`${accountSid}:${authToken}`),
          },
        }
      )

      if (!response.ok) {
        throw new Error('Failed to get Twilio TURN credentials')
      }

      const data = await response.json()
      return { iceServers: data.ice_servers }
    } catch (error) {
      console.error('Error getting Twilio TURN credentials:', error)
      // Fallback to STUN only
      return {
        iceServers: [
          { urls: 'stun:stun.l.google.com:19302' },
        ],
      }
    }
  }

  // Default STUN servers
  return {
    iceServers: [
      { urls: 'stun:stun.l.google.com:19302' },
      { urls: 'stun:stun1.l.google.com:19302' },
    ],
  }
}
