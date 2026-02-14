// device-register Edge Function
// Purpose: Register a new device and await approval

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type, x-device-key',
}

interface DeviceRegisterRequest {
  device_id: string;
  device_name?: string;
  platform: string;
  arch: string;
  cpu_count?: number;
  ram_bytes?: number;
}

serve(async (req) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    const supabaseClient = createClient(
      Deno.env.get('SUPABASE_URL') ?? '',
      Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? '',
    )

    // Parse request body
    const {
      device_id,
      device_name,
      platform,
      arch,
      cpu_count,
      ram_bytes,
    }: DeviceRegisterRequest = await req.json()

    if (!device_id || !platform || !arch) {
      throw new Error('device_id, platform, and arch are required')
    }

    // Check if device already exists
    const { data: existingDevice } = await supabaseClient
      .from('remote_devices')
      .select('device_id, api_key, approved_at')
      .eq('device_id', device_id)
      .single()

    if (existingDevice) {
      // Device already registered
      if (!existingDevice.approved_at) {
        return new Response(
          JSON.stringify({
            status: 'pending_approval',
            message: 'Device registration pending approval',
            device_id: existingDevice.device_id,
          }),
          {
            headers: { ...corsHeaders, 'Content-Type': 'application/json' },
            status: 202,
          }
        )
      }

      // Device approved — only return API key if device proves identity
      const deviceKey = req.headers.get('x-device-key')
      if (deviceKey && deviceKey === existingDevice.api_key) {
        return new Response(
          JSON.stringify({
            status: 'approved',
            device_id: existingDevice.device_id,
            api_key: existingDevice.api_key,
          }),
          {
            headers: { ...corsHeaders, 'Content-Type': 'application/json' },
            status: 200,
          }
        )
      }

      // No valid key presented — confirm approval without leaking the key
      return new Response(
        JSON.stringify({
          status: 'approved',
          device_id: existingDevice.device_id,
        }),
        {
          headers: { ...corsHeaders, 'Content-Type': 'application/json' },
          status: 200,
        }
      )
    }

    // Generate API key for new device
    const api_key = crypto.randomUUID() + '-' + Date.now()

    // Register new device (unapproved by default)
    const { data: newDevice, error: insertError } = await supabaseClient
      .from('remote_devices')
      .insert({
        device_id,
        device_name: device_name || `${platform}-${arch}`,
        platform,
        arch,
        cpu_count,
        ram_bytes,
        api_key,
        is_online: true,
        last_seen: new Date().toISOString(),
      })
      .select()
      .single()

    if (insertError) {
      throw insertError
    }

    // Log audit event
    await supabaseClient.rpc('log_audit_event', {
      p_session_id: null,
      p_device_id: device_id,
      p_event: 'DEVICE_REGISTERED',
      p_details: { platform, arch },
      p_severity: 'info',
    })

    return new Response(
      JSON.stringify({
        status: 'pending_approval',
        message: 'Device registered. Awaiting owner approval.',
        device_id: newDevice.device_id,
      }),
      {
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
        status: 201,
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
