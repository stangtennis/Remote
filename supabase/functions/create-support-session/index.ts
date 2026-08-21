// Create Support Session Edge Function
// POST, requires auth (admin). Generates PIN + token for Quick Support.

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': 'https://dashboard.hawkeye123.dk',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

serve(async (req) => {
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    const body = (await req.json().catch(() => ({}))) || {}

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

    // Check if user is admin (and approved)
    const { data: userApproval } = await supabaseClient
      .from('user_approvals')
      .select('role, approved')
      .eq('user_id', user.id)
      .single()

    const isAdmin = userApproval?.approved === true &&
      (userApproval?.role === 'admin' || userApproval?.role === 'super_admin')
    if (!isAdmin) {
      throw new Error('Admin access required')
    }

    const supportMode = body.support_mode === 'ai' ? 'ai' : 'screen'
    const allowedScopes = ['screen', 'input', 'files', 'terminal', 'process', 'admin']
    const requestedScopes = Array.isArray(body.requested_scopes)
      ? [...new Set(body.requested_scopes.filter((scope: unknown) =>
          typeof scope === 'string' && allowedScopes.includes(scope)
        ))]
      : ['screen']

    if (requestedScopes.length === 0 || !requestedScopes.includes('screen')) {
      throw new Error('A support session must include the screen scope')
    }

    // Generate 6-digit PIN using cryptographically strong randomness
    const pinBuf = new Uint32Array(1)
    crypto.getRandomValues(pinBuf)
    const pin = (100000 + (pinBuf[0] % 900000)).toString()

    // Generate UUID token
    const token = crypto.randomUUID()

    // Session expires in 30 minutes
    const expires_at = new Date(Date.now() + 30 * 60 * 1000).toISOString()

    // Create support session
    const { data: session, error: sessionError } = await supabaseClient
      .from('support_sessions')
      .insert({
        created_by: user.id,
        status: 'pending',
        pin,
        token,
        expires_at,
        support_mode: supportMode,
        requested_scopes: requestedScopes,
        requires_client_code: supportMode === 'ai',
        controller_requested: false,
      })
      .select()
      .single()

    if (sessionError) {
      console.error('Failed to create support session:', sessionError)
      throw sessionError
    }

    const serviceClient = createClient(
      Deno.env.get('SUPABASE_URL') ?? '',
      Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? '',
    )
    const { error: auditError } = await serviceClient.from('support_action_audit').insert({
      support_session_id: session.id,
      actor_type: 'admin',
      actor_id: user.id,
      action_type: 'SUPPORT_SESSION_CREATED',
      status: 'succeeded',
      summary: `Created ${supportMode} support session`,
      details: {
        requested_scopes: requestedScopes,
        requires_client_code: supportMode === 'ai',
        expires_at,
      },
      completed_at: new Date().toISOString(),
    })
    if (auditError) {
      await serviceClient.from('support_sessions').delete().eq('id', session.id)
      throw auditError
    }

    // Build share URL
    const siteUrl = Deno.env.get('SITE_URL') || 'https://stangtennis.github.io/Remote'
    const share_url = `${siteUrl}/support.html?token=${token}`

    return new Response(
      JSON.stringify({
        session_id: session.id,
        pin,
        token,
        share_url,
        expires_at,
        support_mode: supportMode,
        requested_scopes: requestedScopes,
        requires_client_code: supportMode === 'ai',
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
