import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': 'https://dashboard.hawkeye123.dk',
  'Access-Control-Allow-Headers': 'authorization, apikey, content-type, x-ai-controller-key',
}

async function digest(value: string): Promise<Uint8Array> {
  const bytes = new TextEncoder().encode(value)
  return new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
}

function equalBytes(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false
  let diff = 0
  for (let i = 0; i < a.length; i++) diff |= a[i] ^ b[i]
  return diff === 0
}

serve(async (req) => {
  if (req.method === 'OPTIONS') return new Response('ok', { headers: corsHeaders })

  try {
    const configuredKey = Deno.env.get('AI_CONTROLLER_KEY') || ''
    const providedKey = req.headers.get('x-ai-controller-key') || ''
    if (!configuredKey || !providedKey || !equalBytes(await digest(configuredKey), await digest(providedKey))) {
      return new Response(JSON.stringify({ error: 'Invalid AI controller key' }), {
        status: 401,
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
      })
    }

    const authHeader = req.headers.get('Authorization') || ''
    if (!authHeader.startsWith('Bearer ')) throw new Error('Authentication required')

    const url = Deno.env.get('SUPABASE_URL')!
    const anonKey = Deno.env.get('SUPABASE_ANON_KEY')!
    const userClient = createClient(url, anonKey, { global: { headers: { Authorization: authHeader } } })
    const { data: { user }, error: userError } = await userClient.auth.getUser()
    if (userError || !user) throw new Error('Invalid token')

    const { data: approval, error: approvalError } = await userClient
      .from('user_approvals')
      .select('role, approved')
      .eq('user_id', user.id)
      .single()
    if (approvalError || !approval?.approved || !['admin', 'super_admin'].includes(approval.role)) {
      throw new Error('Approved admin access required')
    }

    const body = await req.json().catch(() => ({}))
    const deviceId = typeof body.device_id === 'string' ? body.device_id.trim() : ''
    const controllerId = typeof body.controller_id === 'string' ? body.controller_id.trim() : ''
    if (!deviceId || !controllerId || controllerId.length > 160) {
      throw new Error('device_id and controller_id are required')
    }

    const { data: hasAccess, error: accessError } = await userClient.rpc('user_has_device_access', {
      p_device_id: deviceId,
    })
    if (accessError) throw accessError
    if (hasAccess !== true) throw new Error('Access denied for device')

    const serviceClient = createClient(url, Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!)
    const { data: result, error: claimError } = await serviceClient.rpc('claim_ai_device_connection', {
      p_device_id: deviceId,
      p_controller_id: controllerId,
      p_actor_id: user.id,
    })
    if (claimError) throw claimError

    return new Response(JSON.stringify({
      session_id: result?.session_id,
      device_id: deviceId,
      controller_type: 'ai',
    }), { headers: { ...corsHeaders, 'Content-Type': 'application/json' } })
  } catch (error) {
    return new Response(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }), {
      status: 400,
      headers: { ...corsHeaders, 'Content-Type': 'application/json' },
    })
  }
})
