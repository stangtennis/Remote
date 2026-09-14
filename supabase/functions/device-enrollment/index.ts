// One-time enrollment for an already approved dashboard user.
// Raw enrollment tokens are returned only to the dashboard that created them;
// clients exchange them once for stable device credentials. AI-support
// enrollment mints a separate agent token so the two purposes remain isolated.

import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'
import { serve } from 'https://deno.land/std@0.168.0/http/server.ts'
import { evaluatePurposeAuthorization } from './purpose_auth.ts'

const allowedOrigins = ['https://dashboard.hawkeye123.dk', 'https://stangtennis.github.io']

function headers(req: Request) {
  const origin = req.headers.get('origin') || ''
  return {
    'Access-Control-Allow-Origin': allowedOrigins.includes(origin) ? origin : allowedOrigins[0],
    'Access-Control-Allow-Headers': 'authorization, apikey, content-type, x-device-key',
    'Access-Control-Allow-Methods': 'POST, OPTIONS',
    'Content-Type': 'application/json',
  }
}

function response(req: Request, body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { headers: headers(req), status })
}

function randomToken() {
  const bytes = new Uint8Array(32)
  crypto.getRandomValues(bytes)
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

async function sha256(value: string) {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(value))
  return [...new Uint8Array(digest)].map((item) => item.toString(16).padStart(2, '0')).join('')
}

// Returns the authenticated user plus their user_approvals role (or null)
// only when the user is approved. The role is read from the same approval
// row that gates access; it is never logged.
async function authenticatedUser(req: Request, serviceClient: any) {
  const authHeader = req.headers.get('authorization')
  if (!authHeader?.startsWith('Bearer ')) return null
  const authClient = createClient(
    Deno.env.get('SUPABASE_URL') ?? '',
    Deno.env.get('SUPABASE_ANON_KEY') ?? '',
    { global: { headers: { Authorization: authHeader } } },
  )
  const { data: { user }, error } = await authClient.auth.getUser()
  if (error || !user) return null
  const { data: approval } = await serviceClient
    .from('user_approvals')
    .select('approved, role')
    .eq('user_id', user.id)
    .maybeSingle()
  if (approval?.approved !== true) return null
  return { user, role: typeof approval.role === 'string' ? approval.role : null }
}

serve(async (req) => {
  if (req.method === 'OPTIONS') return new Response('ok', { headers: headers(req) })
  if (req.method !== 'POST') return response(req, { error: 'Method not allowed' }, 405)

  const serviceClient = createClient(
    Deno.env.get('SUPABASE_URL') ?? '',
    Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? '',
  )
  let body: Record<string, unknown>
  try {
    body = await req.json()
  } catch {
    return response(req, { error: 'Invalid JSON' }, 400)
  }

  if (body.action === 'create') {
    const auth = await authenticatedUser(req, serviceClient)
    if (!auth) return response(req, { error: 'Approved user access required' }, 403)
    // Purpose authorization from the existing approval row's role:
    // ordinary approved users may only mint agent tokens; ai_support tokens
    // require admin/super_admin (403 otherwise). Unknown purposes are 400.
    const decision = evaluatePurposeAuthorization(body.purpose, auth.role)
    if (!decision.ok) return response(req, { error: decision.error }, decision.status)
    const purpose = decision.purpose
    const requestedName = typeof body.device_name === 'string' ? body.device_name.trim() : ''
    const deviceName = requestedName.replace(/[^\p{L}\p{N}\s._-]/gu, '').slice(0, 64).trim()
    if (!deviceName) return response(req, { error: 'device_name is required' }, 400)

    const token = randomToken()
    const tokenHash = await sha256(token)
    const expiresAt = new Date(Date.now() + 30 * 60 * 1000).toISOString()
    const tokens = [{
      token_hash: tokenHash,
      owner_id: auth.user.id,
      device_name: deviceName,
      expires_at: expiresAt,
      purpose: purpose,
    }]
    let agentEnrollmentToken: string | undefined
    if (purpose === 'ai_support') {
      agentEnrollmentToken = randomToken()
      tokens.push({
        token_hash: await sha256(agentEnrollmentToken),
        owner_id: auth.user.id,
        device_name: deviceName,
        expires_at: expiresAt,
        purpose: 'agent',
      })
    }
    const { error } = await serviceClient.from('device_enrollment_tokens').insert(tokens)
    if (error) {
      console.error('Enrollment token creation failed:', error)
      return response(req, { error: 'Could not create enrollment' }, 500)
    }
    return response(req, {
      enrollment_token: token,
      agent_enrollment_token: agentEnrollmentToken,
      device_name: deviceName,
      expires_at: expiresAt,
      purpose: purpose,
    })
  }

  if (body.action === 'enroll') {
    const token = typeof body.enrollment_token === 'string' ? body.enrollment_token.trim() : ''
    const deviceId = typeof body.device_id === 'string' ? body.device_id.trim() : ''
    const platform = typeof body.platform === 'string' ? body.platform : ''
    const arch = typeof body.arch === 'string' ? body.arch : ''
    if (!token || token.length > 200 || !/^device_[a-f0-9]{32}$/.test(deviceId) || !platform || !arch) {
      return response(req, { error: 'Invalid enrollment request' }, 400)
    }
    const { data, error } = await serviceClient.rpc('consume_device_enrollment', {
      p_token_hash: await sha256(token),
      p_device_id: deviceId,
      p_platform: platform.slice(0, 50),
      p_arch: arch.slice(0, 50),
      p_cpu_count: typeof body.cpu_count === 'number' ? Math.floor(body.cpu_count) : null,
      p_ram_bytes: typeof body.ram_bytes === 'number' ? Math.floor(body.ram_bytes) : null,
    })
    if (error || !data?.[0]) {
      console.error('Device enrollment failed:', error)
      return response(req, { error: 'Enrollment token is invalid, expired, or already used' }, 409)
    }
    return response(req, { status: 'enrolled', device_id: data[0].device_id, device_name: data[0].device_name, api_key: data[0].api_key })
  }

  // AI-support client registration (Windows -> Ubuntu SSH clients).
  // Validates strictly bounded metadata, consumes the one-time token via the
  // service-role RPC, and returns only registration status. Never returns
  // keys, passwords, or token material.
  if (body.action === 'enroll-ai-support') {
    const token = typeof body.enrollment_token === 'string' ? body.enrollment_token.trim() : ''
    const clientId = typeof body.client_id === 'string' ? body.client_id.trim() : ''
    const hostname = typeof body.hostname === 'string' ? body.hostname.trim().slice(0, 100) : ''
    const platform = typeof body.platform === 'string' ? body.platform.trim().slice(0, 50) : ''
    const sshHost = typeof body.ssh_host === 'string' ? body.ssh_host.trim() : ''
    const sshPort = typeof body.ssh_port === 'number' && Number.isInteger(body.ssh_port) ? body.ssh_port : 22
    const sshUser = typeof body.ssh_user === 'string' ? body.ssh_user.trim() : ''
    const fingerprint = typeof body.ssh_key_fingerprint === 'string' ? body.ssh_key_fingerprint.trim() : ''
    if (!token || token.length > 200 || !/^ai-[a-z0-9]{8,32}$/.test(clientId)) {
      return response(req, { error: 'Invalid enrollment request' }, 400)
    }
    if (!/^[A-Za-z0-9._:-]{1,100}$/.test(sshHost) || !/^[A-Za-z0-9._-]{1,32}$/.test(sshUser) || sshPort < 1 || sshPort > 65535) {
      return response(req, { error: 'Invalid enrollment request' }, 400)
    }
    if (fingerprint && !/^SHA256:[A-Za-z0-9+/=]{43}$/.test(fingerprint)) {
      return response(req, { error: 'Invalid enrollment request' }, 400)
    }
    const { data, error } = await serviceClient.rpc('consume_ai_support_enrollment', {
      p_token_hash: await sha256(token),
      p_client_id: clientId,
      p_hostname: hostname,
      p_platform: platform,
      p_ssh_host: sshHost,
      p_ssh_port: sshPort,
      p_ssh_user: sshUser,
      p_ssh_key_fingerprint: fingerprint || null,
    })
    if (error || !data?.[0]) {
      console.error('AI-support enrollment failed:', error)
      return response(req, { error: 'Enrollment token is invalid, expired, or already used' }, 409)
    }
    return response(req, { status: 'registered', client_id: data[0].client_id, client_name: data[0].client_name })
  }

  return response(req, { error: 'Unknown action' }, 400)
})
