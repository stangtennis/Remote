// Support Signal Edge Function
// POST, no auth required - validates via support token.
// Actions: validate, consent, ready, turn, end, revoke, check-public, create-public, toggle-public

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  // Support links are served from GitHub Pages; no cookies are used by this
  // endpoint, so wildcard CORS is safe for the token/grant-based API.
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

// TURN server configuration
const TURN_SERVER = Deno.env.get('TURN_SERVER') || ''
const TURN_SECRET = Deno.env.get('TURN_SECRET') || ''
const TURN_TTL = parseInt(Deno.env.get('TURN_TTL') || '3600')

const AI_SCOPES = ['screen', 'input', 'files', 'terminal', 'process', 'admin']

function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    headers: { ...corsHeaders, 'Content-Type': 'application/json' },
    status,
  })
}

function redactText(value: string, maxLength: number) {
  return value
    .replace(/(password|passwd|token|secret|credential|apikey)\s*[:=]\s*[^\s,;]+/gi, '$1=[REDACTED]')
    .slice(0, maxLength)
}

function requiredActionScope(actionType: string) {
  if (actionType.startsWith('SCREEN_')) return 'screen'
  if (actionType.startsWith('SHELL_') || actionType.startsWith('TERMINAL_')) return 'terminal'
  if (actionType.startsWith('PROCESS_')) return 'process'
  if (actionType.startsWith('FILE_')) return 'files'
  if (actionType.startsWith('INPUT_') || actionType.startsWith('CLIPBOARD_')) return 'input'
  if (actionType.startsWith('ADMIN_')) return 'admin'
  return ''
}

function requiredActionScopes(actionType: string) {
  const scopes = []
  const primary = requiredActionScope(actionType)
  if (primary) scopes.push(primary)
  if (actionType.startsWith('SHELL_') || actionType.startsWith('TERMINAL_') || actionType === 'PROCESS_KILL') {
    scopes.push('admin')
  }
  return scopes
}

const ALLOWED_SUPPORT_ACTIONS = new Set([
  'SCREEN_SCREENSHOT', 'INPUT_CLICK', 'INPUT_TYPE', 'INPUT_KEY', 'INPUT_SCROLL',
  'INPUT_MOUSE_CLICK', 'INPUT_MOUSE_SCROLL', 'SHELL_EXEC', 'FILE_UPLOAD',
  'FILE_DOWNLOAD', 'FILE_OPERATION', 'TERMINAL_INPUT', 'TERMINAL_START',
  'TERMINAL_CLOSE', 'PROCESS_PS', 'PROCESS_KILL', 'PROCESS_SYSINFO',
  'ADMIN_REMOTE_LOGIN', 'ADMIN_FORCE_UPDATE',
])

function safeActionDetails(value: unknown) {
  const allowed = new Set([
    'action_id', 'exit_code', 'duration_ms', 'bytes', 'items', 'result', 'error',
    'scope', 'path', 'operation', 'reason', 'as_user', 'length', 'command_length', 'command_sha256',
  ])
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  return Object.fromEntries(Object.entries(value)
    .filter(([key]) => allowed.has(key))
    .slice(0, 20)
    .map(([key, item]) => [key, typeof item === 'string' ? redactText(item, 500) : item]))
}

async function auditSupportEvent(supabase: any, event: Record<string, unknown>) {
  const { error } = await supabase.from('support_action_audit').insert(event)
  if (error) {
    console.error('Support audit write failed:', error)
    return false
  }
  return true
}

async function getCloudflareIceServers(): Promise<any[] | null> {
  const keyId = Deno.env.get('CF_TURN_KEY_ID') || ''
  const apiToken = Deno.env.get('CF_TURN_API_TOKEN') || ''
  if (!keyId || !apiToken) return null

  const cfResp = await fetch(
    `https://rtc.live.cloudflare.com/v1/turn/keys/${keyId}/credentials/generate-ice-servers`,
    {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${apiToken}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ ttl: Math.min(TURN_TTL, 86400) }),
    },
  )
  if (!cfResp.ok) return null

  const data = await cfResp.json()
  const servers = (data.iceServers || []).flatMap((server: any) => {
    const urls = Array.isArray(server.urls) ? server.urls : [server.urls]
    const turnUrls = urls.filter((url: string) => url.startsWith('turn'))
    return turnUrls.length && server.username
      ? [{ urls: turnUrls, username: server.username, credential: server.credential }]
      : []
  })
  return servers.length ? servers : null
}

// --- PIN rate-limiting (brute-force protection for support PINs) ---
const PIN_MAX_ATTEMPTS = 10
const PIN_WINDOW_SEC = 600 // 10 minutes

function getClientIP(req: Request): string {
  return (
    req.headers.get('cf-connecting-ip') ||
    req.headers.get('x-real-ip') ||
    (req.headers.get('x-forwarded-for') || '').split(',')[0].trim() ||
    'unknown'
  )
}

async function checkPinRateLimit(supabase: any, source: string): Promise<boolean> {
  const since = new Date(Date.now() - PIN_WINDOW_SEC * 1000).toISOString()
  const { count, error } = await supabase
    .from('support_pin_attempts')
    .select('*', { count: 'exact', head: true })
    .eq('source', source)
    .gte('created_at', since)
  if (error) {
    console.error('PIN rate-limit lookup failed:', error)
    return true
  }
  return !!count && count >= PIN_MAX_ATTEMPTS
}

async function recordPinAttempt(supabase: any, source: string): Promise<boolean> {
  const { error } = await supabase.from('support_pin_attempts').insert({ source })
  if (error) {
    console.error('PIN attempt audit failed:', error)
    return false
  }
  return true
}

serve(async (req) => {
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    const {
      action,
      token,
      pin,
      client_grant_token: clientGrantToken,
      approved,
      scopes,
      client_label: clientLabel,
      session_id: requestedSessionId,
      reason,
      action_type: requestedActionType,
      action_status: requestedActionStatus,
      action_summary: requestedActionSummary,
      action_target: requestedActionTarget,
      action_details: requestedActionDetails,
      signal_type: signalType,
      signal_payload: signalPayload,
      controller_id: requestedControllerID,
    } = await req.json()

    if (!action) {
      throw new Error('action is required')
    }

    // Use service role to bypass RLS
    const supabase = createClient(
      Deno.env.get('SUPABASE_URL')!,
      Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!,
    )

    // --- Public support actions (no token/pin needed) ---

    if (action === 'check-public') {
      const { data } = await supabase
        .from('support_settings')
        .select('public_link_enabled')
        .limit(1)
        .single()

      return new Response(
        JSON.stringify({ enabled: data?.public_link_enabled ?? false }),
        {
          headers: { ...corsHeaders, 'Content-Type': 'application/json' },
          status: 200,
        }
      )
    }

    if (action === 'create-public') {
      // Check if public link is enabled
      const { data: settings } = await supabase
        .from('support_settings')
        .select('public_link_enabled')
        .limit(1)
        .single()

      if (!settings?.public_link_enabled) {
        return new Response(
          JSON.stringify({ error: 'Public support link is not enabled' }),
          {
            headers: { ...corsHeaders, 'Content-Type': 'application/json' },
            status: 403,
          }
        )
      }

      // Generate PIN (not used for public, but column is required).
      // Use cryptographically strong randomness instead of Math.random().
      const pinBuf = new Uint32Array(1)
      crypto.getRandomValues(pinBuf)
      const pin = (100000 + (pinBuf[0] % 900000)).toString()

      // Create public support session
      const { data: newSession, error: insertError } = await supabase
        .from('support_sessions')
        .insert({
          created_by: null,
          status: 'pending',
          pin,
          expires_at: new Date(Date.now() + 30 * 60 * 1000).toISOString(),
          is_public: true,
        })
        .select()
        .single()

      if (insertError || !newSession) {
        throw new Error('Failed to create public session: ' + (insertError?.message || 'unknown'))
      }

      return new Response(
        JSON.stringify({
          session_id: newSession.id,
          token: newSession.token,
        }),
        {
          headers: { ...corsHeaders, 'Content-Type': 'application/json' },
          status: 200,
        }
      )
    }

    if (action === 'toggle-public') {
      // Requires auth - extract Bearer token
      const authHeader = req.headers.get('Authorization')
      if (!authHeader?.startsWith('Bearer ')) {
        return new Response(
          JSON.stringify({ error: 'Authentication required' }),
          {
            headers: { ...corsHeaders, 'Content-Type': 'application/json' },
            status: 401,
          }
        )
      }

      const jwt = authHeader.replace('Bearer ', '')
      // Create authenticated client to verify user
      const authClient = createClient(
        Deno.env.get('SUPABASE_URL')!,
        Deno.env.get('SUPABASE_ANON_KEY')!,
        { global: { headers: { Authorization: `Bearer ${jwt}` } } }
      )
      const { data: { user }, error: userError } = await authClient.auth.getUser()
      if (userError || !user) {
        return new Response(
          JSON.stringify({ error: 'Invalid token' }),
          {
            headers: { ...corsHeaders, 'Content-Type': 'application/json' },
            status: 401,
          }
        )
      }

      // Check admin role (and approved)
      const { data: approval } = await supabase
        .from('user_approvals')
        .select('role, approved')
        .eq('user_id', user.id)
        .single()

      if (!approval || approval.approved !== true ||
          !['admin', 'super_admin'].includes(approval.role)) {
        return new Response(
          JSON.stringify({ error: 'Admin access required' }),
          {
            headers: { ...corsHeaders, 'Content-Type': 'application/json' },
            status: 403,
          }
        )
      }

      // Get current state
      const { data: current } = await supabase
        .from('support_settings')
        .select('id, public_link_enabled')
        .limit(1)
        .single()

      if (!current) {
        throw new Error('support_settings not found')
      }

      const newEnabled = !current.public_link_enabled

      await supabase
        .from('support_settings')
        .update({
          public_link_enabled: newEnabled,
          updated_by: user.id,
          updated_at: new Date().toISOString(),
        })
        .eq('id', current.id)

      return new Response(
        JSON.stringify({ enabled: newEnabled }),
        {
          headers: { ...corsHeaders, 'Content-Type': 'application/json' },
          status: 200,
        }
      )
    }

    if (action === 'revoke') {
      const authHeader = req.headers.get('Authorization')
      if (!authHeader?.startsWith('Bearer ')) return response({ error: 'Authentication required' }, 401)

      const authClient = createClient(
        Deno.env.get('SUPABASE_URL')!,
        Deno.env.get('SUPABASE_ANON_KEY')!,
        { global: { headers: { Authorization: authHeader } } },
      )
      const { data: { user }, error: userError } = await authClient.auth.getUser()
      if (userError || !user) return response({ error: 'Invalid token' }, 401)

      const { data: approval } = await supabase
        .from('user_approvals')
        .select('role, approved')
        .eq('user_id', user.id)
        .single()
      if (!approval?.approved || !['admin', 'super_admin'].includes(approval.role)) {
        return response({ error: 'Admin access required' }, 403)
      }

      const { data: session } = await supabase
        .from('support_sessions')
        .select('id, created_by, is_public, status')
        .eq('id', requestedSessionId)
        .single()
      if (!session || (!session.is_public && session.created_by !== user.id)) {
        return response({ error: 'Support session not found' }, 404)
      }

      const now = new Date().toISOString()
      const { data: revoked, error: revokeError } = await supabase.rpc('revoke_support_session', {
        p_session_id: session.id,
        p_admin_id: user.id,
        p_reason: reason || 'Revoked by admin',
      })
      if (revokeError) throw revokeError
      if (!revoked) return response({ error: 'Support session is already closed' }, 409)
      const auditWritten = await auditSupportEvent(supabase, {
        support_session_id: session.id,
        actor_type: 'admin',
        actor_id: user.id,
        action_type: 'SUPPORT_SESSION_REVOKED',
        status: 'succeeded',
        summary: 'Support session revoked by admin',
         details: { reason: redactText(reason || 'Revoked by admin', 500) },
        completed_at: now,
      })
      if (!auditWritten) return response({ error: 'Session revoked, but audit storage is unavailable' }, 503)
      return response({ ok: true, session_id: session.id })
    }

    // Authenticated controller-side audit for AI commands. The agent also
    // reports execution results separately; this row records the AI intent.
    if (action === 'record-admin-action') {
      const authHeader = req.headers.get('Authorization')
      if (!authHeader?.startsWith('Bearer ')) return response({ error: 'Authentication required' }, 401)
      const authClient = createClient(
        Deno.env.get('SUPABASE_URL')!,
        Deno.env.get('SUPABASE_ANON_KEY')!,
        { global: { headers: { Authorization: authHeader } } },
      )
      const { data: { user }, error: userError } = await authClient.auth.getUser()
      if (userError || !user) return response({ error: 'Invalid token' }, 401)
      const { data: approval } = await supabase
        .from('user_approvals')
        .select('role, approved')
        .eq('user_id', user.id)
        .single()
      if (!approval?.approved || !['admin', 'super_admin'].includes(approval.role)) {
        return response({ error: 'Admin access required' }, 403)
      }
      const { data: session } = await supabase
        .from('support_sessions')
        .select('id, created_by, status, expires_at, client_consent_scopes')
        .eq('id', requestedSessionId)
        .single()
      if (!session || session.created_by !== user.id || session.status !== 'active') {
        return response({ error: 'Active support session not found' }, 409)
      }
      if (new Date(session.expires_at) <= new Date()) {
        return response({ error: 'Support session has expired' }, 409)
      }
      const actionType = typeof requestedActionType === 'string' ? requestedActionType.slice(0, 80) : ''
      const summary = typeof requestedActionSummary === 'string' ? redactText(requestedActionSummary, 500) : ''
        if (!ALLOWED_SUPPORT_ACTIONS.has(actionType) || !summary) {
        return response({ error: 'Unsupported support action' }, 400)
      }
      const consentedScopes = Array.isArray(session.client_consent_scopes) ? session.client_consent_scopes : []
      const missingScope = requiredActionScopes(actionType).find((scope) => !consentedScopes.includes(scope))
      if (missingScope) {
        return response({ error: `Action requires consented scope: ${missingScope}` }, 403)
      }
      const now = new Date().toISOString()
      const { error: auditError } = await supabase.from('support_action_audit').insert({
        support_session_id: session.id,
        actor_type: 'ai',
        actor_id: user.id,
        action_type: actionType,
        target: typeof requestedActionTarget === 'string' ? redactText(requestedActionTarget, 500) : null,
        status: requestedActionStatus || 'started',
        summary,
        details: safeActionDetails(requestedActionDetails),
        verified: false,
        completed_at: requestedActionStatus && requestedActionStatus !== 'started' ? now : null,
      })
      if (auditError) throw auditError
      return response({ ok: true, session_id: session.id })
    }

    if (action === 'request-controller') {
      const authHeader = req.headers.get('Authorization')
      if (!authHeader?.startsWith('Bearer ')) return response({ error: 'Authentication required' }, 401)
      const authClient = createClient(
        Deno.env.get('SUPABASE_URL')!,
        Deno.env.get('SUPABASE_ANON_KEY')!,
        { global: { headers: { Authorization: authHeader } } },
      )
      const { data: { user }, error: userError } = await authClient.auth.getUser()
      if (userError || !user) return response({ error: 'Invalid token' }, 401)
      const { data: approval } = await supabase
        .from('user_approvals')
        .select('role, approved')
        .eq('user_id', user.id)
        .single()
      if (!approval?.approved || !['admin', 'super_admin'].includes(approval.role)) {
        return response({ error: 'Admin access required' }, 403)
      }
      const { data: requested, error } = await supabase.rpc('request_support_controller', {
        p_session_id: requestedSessionId,
        p_admin_id: user.id,
      })
      if (error) throw error
      if (!requested) return response({ error: 'AI support session is not owned by this admin or is already closed' }, 409)
      return response({ ok: true, session_id: requestedSessionId, requested: true })
    }

    if (action === 'claim-controller' || action === 'release-controller') {
      const authHeader = req.headers.get('Authorization')
      if (!authHeader?.startsWith('Bearer ')) return response({ error: 'Authentication required' }, 401)
      const authClient = createClient(
        Deno.env.get('SUPABASE_URL')!,
        Deno.env.get('SUPABASE_ANON_KEY')!,
        { global: { headers: { Authorization: authHeader } } },
      )
      const { data: { user }, error: userError } = await authClient.auth.getUser()
      if (userError || !user) return response({ error: 'Invalid token' }, 401)
      const { data: approval } = await supabase
        .from('user_approvals')
        .select('role, approved')
        .eq('user_id', user.id)
        .single()
      if (!approval?.approved || !['admin', 'super_admin'].includes(approval.role)) {
        return response({ error: 'Admin access required' }, 403)
      }
      if (typeof requestedControllerID !== 'string' || !requestedControllerID.trim()) {
        return response({ error: 'controller_id is required' }, 400)
      }

      if (action === 'release-controller') {
        const { error } = await supabase
          .from('support_sessions')
          .update({ controller_claimed_by: null, controller_claimed_at: null })
          .eq('id', requestedSessionId)
          .eq('created_by', user.id)
          .eq('controller_claimed_by', requestedControllerID.trim())
        if (error) throw error
        return response({ ok: true, session_id: requestedSessionId, released: true })
      }

      const controllerID = requestedControllerID.trim()
      const leaseCutoff = new Date(Date.now() - 120000)
      const { data: current, error: lookupError } = await supabase
        .from('support_sessions')
        .select('id, controller_claimed_by, controller_claimed_at')
        .eq('id', requestedSessionId)
        .eq('created_by', user.id)
        .in('status', ['pending', 'active'])
        .eq('support_mode', 'ai')
        .not('client_consent_at', 'is', null)
        .eq('controller_requested', true)
        .maybeSingle()
      if (lookupError) throw lookupError
      if (!current) return response({ ok: true, session_id: requestedSessionId, claimed: false })

      const existingController = typeof current.controller_claimed_by === 'string'
        ? current.controller_claimed_by.trim()
        : ''
      const existingClaimAt = current.controller_claimed_at ? new Date(current.controller_claimed_at) : null
      const leaseActive = existingController && existingController !== controllerID &&
        existingClaimAt && existingClaimAt > leaseCutoff
      if (leaseActive) return response({ ok: true, session_id: requestedSessionId, claimed: false })

      let claimQuery = supabase
        .from('support_sessions')
        .update({ controller_claimed_by: controllerID, controller_claimed_at: new Date().toISOString() })
        .eq('id', requestedSessionId)
        .eq('created_by', user.id)
        .eq('controller_requested', true)
      if (!existingController) {
        claimQuery = claimQuery.is('controller_claimed_by', null)
      } else if (existingController === controllerID) {
        claimQuery = claimQuery.eq('controller_claimed_by', controllerID)
      } else {
        claimQuery = claimQuery.eq('controller_claimed_by', existingController).lt('controller_claimed_at', leaseCutoff.toISOString())
      }
      const { data: claimed, error: claimError } = await claimQuery.select('id').maybeSingle()
      if (claimError) throw claimError
      return response({ ok: true, session_id: requestedSessionId, claimed: !!claimed })
    }

    // --- Token/PIN/grant-based actions (validate, consent, ready, turn, end) ---

    let session: any = null
    const source = getClientIP(req)
    if (action === 'validate' && pin && await checkPinRateLimit(supabase, source)) {
      return response({ error: 'Too many PIN attempts. Please try again later.' }, 429)
    }

    if (clientGrantToken) {
      const { data, error } = await supabase
        .from('support_sessions')
        .select('*')
        .eq('client_grant_token', clientGrantToken)
        .single()
      if (error || !data || data.support_mode !== 'ai') {
        throw new Error('Invalid or expired client grant')
      }
      if (!['pending', 'active'].includes(data.status)) {
        throw new Error('Support session is no longer active')
      }
      session = data
    } else if (token) {
      const { data, error } = await supabase
        .from('support_sessions')
        .select('*')
        .eq('token', token)
        .in('status', ['pending', 'active'])
        .single()
      if (error || !data) throw new Error('Invalid or expired support token')
      session = data
    } else if (pin) {
      const { data, error } = await supabase
        .from('support_sessions')
        .select('*')
        .eq('pin', pin)
        .in('status', ['pending', 'active'])
        .order('created_at', { ascending: false })
        .limit(1)
        .single()
      if (error || !data) {
        if (!await recordPinAttempt(supabase, source)) {
          return response({ error: 'PIN validation temporarily unavailable' }, 503)
        }
        throw new Error('Invalid PIN or no active session found')
      }
      session = data
    } else {
      throw new Error('token, pin, or client_grant_token is required')
    }

    if (new Date(session.expires_at) < new Date()) {
      await supabase.from('support_sessions').update({ status: 'expired', ended_at: new Date().toISOString() }).eq('id', session.id)
      throw new Error('Support session has expired')
    }

    // AI sessions always require the code, even when the share URL contains a token.
    if (action === 'validate' && session.support_mode === 'ai') {
      if (!pin || pin !== session.pin) {
        if (!await recordPinAttempt(supabase, source)) {
          return response({ error: 'PIN validation temporarily unavailable' }, 503)
        }
        return response({ error: 'Client code required or invalid', code_required: true }, 401)
      }

      const grant = crypto.randomUUID()
      const now = new Date().toISOString()
      const { error: grantError } = await supabase
        .from('support_sessions')
        .update({
          client_code_verified_at: now,
          client_grant_token: grant,
          client_consent_at: null,
          client_consent_scopes: null,
          client_label: null,
          consent_policy_version: null,
        })
        .eq('id', session.id)
      if (grantError) throw grantError

      const auditWritten = await auditSupportEvent(supabase, {
        support_session_id: session.id,
        actor_type: 'client',
        action_type: 'CLIENT_CODE_VERIFIED',
        status: 'succeeded',
        summary: 'Client entered the admin support code',
        details: { requested_scopes: session.requested_scopes },
        completed_at: now,
      })
      if (!auditWritten) {
        await supabase.from('support_sessions').update({
          client_code_verified_at: null,
          client_grant_token: null,
        }).eq('id', session.id).eq('client_grant_token', grant)
        return response({ error: 'Audit service unavailable; consent flow was not opened' }, 503)
      }

      return response({
        session_id: session.id,
        client_grant_token: grant,
        status: session.status,
        support_mode: session.support_mode,
        requested_scopes: session.requested_scopes,
        requires_consent: true,
        expires_at: session.expires_at,
      })
    }

    if (session.support_mode === 'ai' && !clientGrantToken) {
      throw new Error('AI support requires client code and consent')
    }

    switch (action) {
      case 'signal-read': {
        if (session.support_mode === 'ai' && !session.client_consent_at) {
          throw new Error('Client consent is required before signaling')
        }
        const { data: signals, error: signalError } = await supabase
          .from('session_signaling')
          .select('id, session_id, from_side, msg_type, payload, created_at')
          .eq('session_id', session.id)
          .eq('from_side', 'dashboard')
          .order('created_at', { ascending: true })
          .limit(100)
        if (signalError) throw signalError
        return response({ signals: signals || [] })
      }

      case 'signal-write': {
        if (session.support_mode === 'ai' && !session.client_consent_at) {
          throw new Error('Client consent is required before signaling')
        }
        const allowedSignalTypes = ['answer', 'ice', 'bye']
        if (!allowedSignalTypes.includes(signalType) || signalPayload === undefined) {
          throw new Error('Invalid support signal')
        }
        const { error: signalError } = await supabase.rpc('append_support_signal', {
          p_session_id: session.id,
          p_from_side: 'support',
          p_msg_type: signalType,
          p_payload: signalPayload,
        })
        if (signalError) throw signalError
        return response({ ok: true })
      }

      case 'validate': {
        return response({
          session_id: session.id,
          token: session.token,
          status: session.status,
          support_mode: session.support_mode,
          requested_scopes: session.requested_scopes || ['screen'],
          expires_at: session.expires_at,
        })
      }

      case 'consent': {
        if (session.support_mode !== 'ai' || !clientGrantToken) {
          throw new Error('AI client grant required')
        }
        if (approved !== true) {
          throw new Error('Client consent is required')
        }

        const requested = Array.isArray(session.requested_scopes)
          ? session.requested_scopes
          : ['screen']
        const chosen = Array.isArray(scopes)
          ? [...new Set(scopes.filter((scope: unknown) => typeof scope === 'string'))]
          : []
        if (!chosen.includes('screen') || chosen.some((scope: string) =>
          !AI_SCOPES.includes(scope) || !requested.includes(scope)
        )) {
          throw new Error('Consent scopes are not allowed for this session')
        }

        const now = new Date().toISOString()
        const { error: consentError } = await supabase
          .from('support_sessions')
          .update({
            client_consent_at: now,
            client_consent_scopes: chosen,
             client_label: typeof clientLabel === 'string' ? redactText(clientLabel, 120) : null,
            consent_policy_version: 'ai-support-v1',
          })
          .eq('id', session.id)
          .eq('client_grant_token', clientGrantToken)
        if (consentError) throw consentError

        const auditWritten = await auditSupportEvent(supabase, {
          support_session_id: session.id,
          actor_type: 'client',
          action_type: 'CLIENT_CONSENT_GRANTED',
          status: 'succeeded',
          summary: 'Client approved AI support scopes',
           details: { scopes: chosen, client_label: typeof clientLabel === 'string' ? redactText(clientLabel, 120) : null, policy_version: 'ai-support-v1' },
          completed_at: now,
        })
        if (!auditWritten) {
          await supabase.from('support_sessions').update({
            client_consent_at: null,
            client_consent_scopes: null,
            client_label: null,
            consent_policy_version: null,
          }).eq('id', session.id).eq('client_grant_token', clientGrantToken)
          return response({ error: 'Audit service unavailable; consent was not accepted' }, 503)
        }
        return response({ ok: true, session_id: session.id, scopes: chosen })
      }

      case 'ready': {
        if (session.support_mode === 'ai' && !session.client_consent_at) {
          throw new Error('Client consent is required before support can start')
        }
        const now = new Date().toISOString()
        // Update session status to active
        const { error: statusError } = await supabase
          .from('support_sessions')
          .update({ status: 'active' })
          .eq('id', session.id)
          .in('status', ['pending', 'active'])
        if (statusError) throw statusError

        // Insert ready signal so dashboard knows sharer is ready
        const { error: readySignalError } = await supabase.rpc('append_support_signal', {
          p_session_id: session.id,
          p_from_side: 'support',
          p_msg_type: 'answer',
          p_payload: { type: 'ready' },
        })
        if (readySignalError) throw readySignalError
        const auditWritten = await auditSupportEvent(supabase, {
          support_session_id: session.id,
          actor_type: session.support_mode === 'ai' ? 'client' : 'system',
          action_type: 'SUPPORT_SESSION_READY',
          status: 'succeeded',
          summary: 'Support client is ready for WebRTC',
          details: { support_mode: session.support_mode, scopes: session.client_consent_scopes || ['screen'] },
          completed_at: now,
        })
        if (!auditWritten) return response({ error: 'Audit service unavailable; support was not marked ready' }, 503)
        return response({ ok: true, session_id: session.id })
      }

      case 'turn': {
        if (session.support_mode === 'ai' && !session.client_consent_at) {
          throw new Error('Client consent is required before TURN credentials are issued')
        }

        const timestamp = Math.floor(Date.now() / 1000) + TURN_TTL
        const iceServers: any[] = [{ urls: ['stun:stun.cloudflare.com:3478', 'stun:stun.l.google.com:19302'] }]
        const cloudflareServers = await getCloudflareIceServers()
        if (session.support_mode === 'ai' && !cloudflareServers) {
          throw new Error('Cloudflare TURN relay is unavailable for AI support')
        }
        if (cloudflareServers) {
          iceServers.push(...cloudflareServers)
        } else if (TURN_SERVER && TURN_SECRET) {
          const username = `${timestamp}:support-${session.id}`
          const encoder = new TextEncoder()
          const key = encoder.encode(TURN_SECRET)
          const message = encoder.encode(username)
          const cryptoKey = await crypto.subtle.importKey(
            'raw', key, { name: 'HMAC', hash: 'SHA-1' }, false, ['sign']
          )
          const sig = await crypto.subtle.sign('HMAC', cryptoKey, message)
          const credential = btoa(String.fromCharCode(...new Uint8Array(sig)))
          iceServers.push(
            { urls: TURN_SERVER, username, credential },
            { urls: `${TURN_SERVER}?transport=tcp`, username, credential },
          )
        }

        const auditWritten = await auditSupportEvent(supabase, {
          support_session_id: session.id,
          actor_type: 'system',
          action_type: 'TURN_CREDENTIALS_ISSUED',
          status: 'succeeded',
          summary: 'Issued short-lived WebRTC relay credentials',
          details: { provider: cloudflareServers ? 'cloudflare' : 'coturn-or-stun', ttl: TURN_TTL },
          completed_at: new Date().toISOString(),
        })
        if (!auditWritten) return response({ error: 'Audit service unavailable; relay was not issued' }, 503)
        return response({ iceServers, ttl: TURN_TTL, expires: timestamp })
      }

      case 'end': {
        const now = new Date().toISOString()
        const { error: endSignalError } = await supabase.rpc('append_support_signal', {
          p_session_id: session.id,
          p_from_side: 'support',
          p_msg_type: 'bye',
          p_payload: { reason: 'support_client_ended' },
        })
        if (endSignalError) throw endSignalError
        const { error: endError } = await supabase
          .from('support_sessions')
          .update({
            status: 'ended',
            ended_at: now,
            controller_requested: false,
            controller_claimed_by: null,
            controller_claimed_at: null,
          })
          .eq('id', session.id)
          .in('status', ['pending', 'active'])
        if (endError) throw endError
        const auditWritten = await auditSupportEvent(supabase, {
          support_session_id: session.id,
          actor_type: session.support_mode === 'ai' ? 'client' : 'system',
          action_type: 'SUPPORT_SESSION_ENDED',
          status: 'succeeded',
          summary: 'Support session ended by client',
          details: {},
          completed_at: now,
        })
        if (!auditWritten) return response({ error: 'Session ended, but audit storage is unavailable' }, 503)
        return response({ ok: true, session_id: session.id })
      }

      case 'record-action': {
        if (session.support_mode !== 'ai' || !clientGrantToken || !session.client_consent_at) {
          throw new Error('AI consent is required before recording actions')
        }
        const actionType = typeof requestedActionType === 'string'
          ? requestedActionType.slice(0, 80)
          : ''
        const summary = typeof requestedActionSummary === 'string'
          ? redactText(requestedActionSummary, 500)
          : ''
        if (!actionType || !summary) throw new Error('action_type and action_summary are required')
        if (!ALLOWED_SUPPORT_ACTIONS.has(actionType)) {
          throw new Error('Unsupported support action type')
        }
        const consentedScopes = Array.isArray(session.client_consent_scopes)
          ? session.client_consent_scopes
          : []
        const missingScope = requiredActionScopes(actionType).find((scope) => !consentedScopes.includes(scope))
        if (missingScope) {
          throw new Error(`Action requires consented scope: ${missingScope}`)
        }
        const allowedStatuses = ['started', 'succeeded', 'failed', 'cancelled']
        const actionStatus = allowedStatuses.includes(requestedActionStatus)
          ? requestedActionStatus
          : 'succeeded'
        const now = new Date().toISOString()
        const safeDetails = safeActionDetails(requestedActionDetails)
        const auditWritten = await auditSupportEvent(supabase, {
          support_session_id: session.id,
          actor_type: 'agent',
          action_type: actionType,
          target: typeof requestedActionTarget === 'string' ? redactText(requestedActionTarget, 500) : null,
          status: actionStatus,
          summary,
          details: safeDetails,
          // Completed records are executor reports; controller intent remains unverified.
          verified: actionStatus !== 'started',
          completed_at: actionStatus === 'started' ? null : now,
        })
        if (!auditWritten) return response({ error: 'Audit service unavailable; action record was not stored' }, 503)
        return response({ ok: true, session_id: session.id })
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
