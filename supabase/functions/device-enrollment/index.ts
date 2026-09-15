// One-time enrollment for an already approved dashboard user.
// Raw enrollment tokens are returned only to the dashboard that created them;
// clients exchange them once for stable device credentials. AI-support
// enrollment is SSH-only and never mints a Remote Desktop agent token.

import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'
import { serve } from 'https://deno.land/std@0.168.0/http/server.ts'
import { evaluatePurposeAuthorization, isAdminRole } from './purpose_auth.ts'

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

function classifySupportOperation(command: string) {
  if (!command.trim() || command.trim() === '[interactive shell]') return 'interactive_shell'
  const patterns: Array<[string, RegExp]> = [
    ['system_diagnostics', /\b(Get-ComputerInfo|Get-CimInstance|Get-WinEvent|systeminfo|whoami|hostname)\b/i],
    ['network_diagnostics', /\b(Get-NetTCPConnection|Test-NetConnection|ipconfig|ping|nslookup|Resolve-DnsName)\b/i],
    ['file_inspection', /\b(Get-ChildItem|Test-Path|Resolve-Path|dir|ls)\b/i],
    ['file_change', /\b(Set-Content|Add-Content|Copy-Item|Move-Item|New-Item|Remove-Item)\b/i],
    ['service_change', /\b(Get-Service|Start-Service|Stop-Service|Restart-Service|Set-Service)\b/i],
    ['process_change', /\b(Get-Process|Start-Process|Stop-Process|Wait-Process)\b/i],
    ['scheduled_task', /\b(Get-ScheduledTask|Start-ScheduledTask|Stop-ScheduledTask|Register-ScheduledTask|Unregister-ScheduledTask|schtasks)\b/i],
    ['account_change', /\b(Get-LocalUser|New-LocalUser|Remove-LocalUser|Add-LocalGroupMember|Remove-LocalGroupMember)\b/i],
    ['remote_access', /\b(ssh|scp|sftp|cloudflared)\b/i],
  ]
  return patterns.find(([, pattern]) => pattern.test(command))?.[0] || 'other_powershell'
}

async function cloudflareRequest(
  method: string,
  url: string,
  apiToken: string,
  operation: string,
  body?: Record<string, string>,
  allowNotFound = false,
) {
  let cloudflareResponse: Response
  try {
    cloudflareResponse = await fetch(url, {
      method,
      headers: {
        Authorization: `Bearer ${apiToken}`,
        'Content-Type': 'application/json',
      },
      body: body ? JSON.stringify(body) : undefined,
    })
  } catch {
    console.error('Cloudflare API request failed', { operation, status: 'network_error' })
    throw new Error('Cloudflare API request failed')
  }

  let payload: any = null
  try {
    payload = await cloudflareResponse.json()
  } catch {
    payload = null
  }
  if (allowNotFound && cloudflareResponse.status === 404) return null
  if (cloudflareResponse.status === 204) return null
  if (!cloudflareResponse.ok || payload?.success !== true) {
    console.error('Cloudflare API request failed', {
      operation,
      status: cloudflareResponse.status,
      success: payload?.success === true,
    })
    throw new Error('Cloudflare API request failed')
  }
  return payload
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
    const { error } = await serviceClient.from('device_enrollment_tokens').insert(tokens)
    if (error) {
      console.error('Enrollment token creation failed:', error)
      return response(req, { error: `Could not create enrollment (${error.code || 'database_error'})` }, 500)
    }
    return response(req, {
      enrollment_token: token,
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

  if (body.action === 'issue-ai-support-cloudflare-token') {
    const token = typeof body.enrollment_token === 'string' ? body.enrollment_token.trim() : ''
    const clientId = typeof body.client_id === 'string' ? body.client_id.trim() : ''
    if (!token || token.length > 200 || !/^ai-[a-z0-9]{8,32}$/.test(clientId)) {
      return response(req, { error: 'Invalid enrollment request' }, 400)
    }

    const { data: enrollment, error: enrollmentError } = await serviceClient
      .from('device_enrollment_tokens')
      .select('id, purpose, used_at, expires_at, cloudflare_service_token_id')
      .eq('token_hash', await sha256(token))
      .maybeSingle()
    if (enrollmentError) {
      console.error('Cloudflare token issuance lookup failed', { code: enrollmentError.code || 'database_error' })
      return response(req, { error: 'Could not validate enrollment token' }, 500)
    }
    if (!enrollment || enrollment.purpose !== 'ai_support' || enrollment.used_at !== null || new Date(enrollment.expires_at).getTime() <= Date.now()) {
      return response(req, { error: 'Enrollment token is invalid, expired, or already used' }, 409)
    }

    const cloudflareApiToken = Deno.env.get('CLOUDFLARE_API_TOKEN')?.trim() || ''
    const cloudflareAccountId = Deno.env.get('CLOUDFLARE_ACCOUNT_ID')?.trim() || ''
    const serviceTokenDuration = (Deno.env.get('CLOUDFLARE_SERVICE_TOKEN_DURATION')?.trim() || '8760h')
    if (!cloudflareApiToken || !cloudflareAccountId || !/^[A-Za-z0-9_-]{1,100}$/.test(cloudflareAccountId) || !/^[A-Za-z0-9_-]{1,64}$/.test(serviceTokenDuration)) {
      console.error('Cloudflare token issuance is not configured')
      return response(req, { error: 'Cloudflare token issuance is not configured' }, 503)
    }

    const serviceTokensUrl = `https://api.cloudflare.com/client/v4/accounts/${encodeURIComponent(cloudflareAccountId)}/access/service_tokens`
    const previousServiceTokenId = typeof enrollment.cloudflare_service_token_id === 'string'
      ? enrollment.cloudflare_service_token_id
      : ''
    if (previousServiceTokenId) {
      try {
        await cloudflareRequest(
          'DELETE',
          `${serviceTokensUrl}/${encodeURIComponent(previousServiceTokenId)}`,
          cloudflareApiToken,
          'delete_service_token',
          undefined,
          true,
        )
      } catch {
        return response(req, { error: 'Could not replace the Cloudflare service token' }, 502)
      }
    }

    let issuedPayload: any = null
    let clientSecret = ''
    let serviceTokenId = ''
    let serviceTokenClientId = ''
    try {
      try {
        issuedPayload = await cloudflareRequest(
          'POST',
          serviceTokensUrl,
          cloudflareApiToken,
          'create_service_token',
          { name: `AI-support ${clientId}`, duration: serviceTokenDuration },
        )
      } catch {
        return response(req, { error: 'Could not issue the Cloudflare service token' }, 502)
      }

      const result = issuedPayload?.result
      serviceTokenId = typeof result?.id === 'string' ? result.id.trim() : ''
      serviceTokenClientId = typeof result?.client_id === 'string' ? result.client_id.trim() : ''
      clientSecret = typeof result?.client_secret === 'string' ? result.client_secret : ''
      if (!/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/.test(serviceTokenId) ||
        !/^[A-Za-z0-9._:-]{1,200}$/.test(serviceTokenClientId) || !clientSecret || clientSecret.length > 2000) {
        console.error('Cloudflare token issuance returned invalid metadata')
        if (serviceTokenId) {
          try {
            await cloudflareRequest('DELETE', `${serviceTokensUrl}/${encodeURIComponent(serviceTokenId)}`, cloudflareApiToken, 'delete_invalid_service_token', undefined, true)
          } catch { }
        }
        return response(req, { error: 'Cloudflare returned an invalid service token' }, 502)
      }

      const { data: updatedEnrollment, error: updateError } = await serviceClient
        .from('device_enrollment_tokens')
        .update({ cloudflare_service_token_id: serviceTokenId })
        .eq('id', enrollment.id)
        .is('used_at', null)
        .select('id')
      if (updateError || !updatedEnrollment?.[0]) {
        console.error('Cloudflare token issuance state update failed', { code: updateError?.code || 'not_updated' })
        try {
          await cloudflareRequest('DELETE', `${serviceTokensUrl}/${encodeURIComponent(serviceTokenId)}`, cloudflareApiToken, 'delete_unrecorded_service_token', undefined, true)
        } catch { }
        return response(req, { error: 'Could not save Cloudflare token issuance state' }, 500)
      }

      return response(req, {
        service_token_id: serviceTokenId,
        client_id: serviceTokenClientId,
        client_secret: clientSecret,
      })
    } finally {
      clientSecret = ''
      issuedPayload = null
    }
  }

  if (body.action === 'revoke-ai-support-cloudflare-token') {
    const clientId = typeof body.client_id === 'string' ? body.client_id.trim() : ''
    if (!/^ai-[a-z0-9]{8,32}$/.test(clientId)) {
      return response(req, { error: 'Invalid client ID' }, 400)
    }

    const auth = await authenticatedUser(req, serviceClient)
    if (!auth) return response(req, { error: 'Approved user access required' }, 403)

    const { data: client, error: clientError } = await serviceClient
      .from('ai_support_clients')
      .select('owner_id')
      .eq('client_id', clientId)
      .maybeSingle()
    if (clientError) {
      console.error('Cloudflare token revocation client lookup failed', { code: clientError.code || 'database_error' })
      return response(req, { error: 'Could not find AI-support client' }, 500)
    }
    if (!client) return response(req, { error: 'AI-support client not found' }, 404)
    if (client.owner_id !== auth.user.id && !isAdminRole(auth.role)) {
      return response(req, { error: 'Only the owner or an admin may revoke this AI-support client' }, 403)
    }

    const { data: enrollment, error: enrollmentError } = await serviceClient
      .from('device_enrollment_tokens')
      .select('cloudflare_service_token_id')
      .eq('device_id', clientId)
      .eq('purpose', 'ai_support')
      .maybeSingle()
    if (enrollmentError) {
      console.error('Cloudflare token revocation lookup failed', { code: enrollmentError.code || 'database_error' })
      return response(req, { error: 'Could not find AI-support enrollment metadata' }, 500)
    }

    const serviceTokenId = typeof enrollment?.cloudflare_service_token_id === 'string'
      ? enrollment.cloudflare_service_token_id.trim()
      : ''
    if (!serviceTokenId) return response(req, { status: 'revoked' })
    if (!/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/.test(serviceTokenId)) {
      console.error('Cloudflare token revocation found invalid metadata')
      return response(req, { error: 'Cloudflare service-token metadata is invalid' }, 500)
    }

    const cloudflareApiToken = Deno.env.get('CLOUDFLARE_API_TOKEN')?.trim() || ''
    const cloudflareAccountId = Deno.env.get('CLOUDFLARE_ACCOUNT_ID')?.trim() || ''
    if (!cloudflareApiToken || !cloudflareAccountId || !/^[A-Za-z0-9_-]{1,100}$/.test(cloudflareAccountId)) {
      console.error('Cloudflare token revocation is not configured')
      return response(req, { error: 'Cloudflare token revocation is not configured' }, 503)
    }

    const serviceTokenUrl = `https://api.cloudflare.com/client/v4/accounts/${encodeURIComponent(cloudflareAccountId)}/access/service_tokens/${encodeURIComponent(serviceTokenId)}`
    try {
      await cloudflareRequest('DELETE', serviceTokenUrl, cloudflareApiToken, 'delete_service_token', undefined, true)
    } catch {
      return response(req, { error: 'Could not revoke the Cloudflare service token' }, 502)
    }

    const { error: clearError } = await serviceClient
      .from('device_enrollment_tokens')
      .update({ cloudflare_service_token_id: null })
      .eq('device_id', clientId)
      .eq('purpose', 'ai_support')
      .eq('cloudflare_service_token_id', serviceTokenId)
    if (clearError) {
      console.error('Cloudflare token revocation state update failed', { code: clearError.code || 'database_error' })
      return response(req, { error: 'Cloudflare token was revoked but metadata cleanup failed' }, 500)
    }
    return response(req, { status: 'revoked' })
  }

  // AI-support client registration (Windows -> Ubuntu persistent SSH tunnel).
  // Validates strictly bounded metadata, consumes the one-time token via the
  // service-role RPC, and returns registration status plus a client-scoped
  // activity-log token. Never returns keys, passwords, or private key material.
  if (body.action === 'enroll-ai-support') {
    const token = typeof body.enrollment_token === 'string' ? body.enrollment_token.trim() : ''
    const clientId = typeof body.client_id === 'string' ? body.client_id.trim() : ''
    const hostname = typeof body.hostname === 'string' ? body.hostname.trim().slice(0, 100) : ''
    const platform = typeof body.platform === 'string' ? body.platform.trim().slice(0, 50) : ''
    const sshHost = typeof body.ssh_host === 'string' ? body.ssh_host.trim() : ''
    const sshPort = typeof body.ssh_port === 'number' && Number.isInteger(body.ssh_port) ? body.ssh_port : 22
    const sshUser = typeof body.ssh_user === 'string' ? body.ssh_user.trim() : ''
    const fingerprint = typeof body.ssh_key_fingerprint === 'string' ? body.ssh_key_fingerprint.trim() : ''
    const tunnelPort = typeof body.tunnel_port === 'number' && Number.isInteger(body.tunnel_port) ? body.tunnel_port : 0
    const windowsSshUser = typeof body.windows_ssh_user === 'string' ? body.windows_ssh_user.trim() : ''
    const windowsSshPort = typeof body.windows_ssh_port === 'number' && Number.isInteger(body.windows_ssh_port) ? body.windows_ssh_port : 0
    if (!token || token.length > 200 || !/^ai-[a-z0-9]{8,32}$/.test(clientId)) {
      return response(req, { error: 'Invalid enrollment request' }, 400)
    }
    if (!/^[A-Za-z0-9._:-]{1,100}$/.test(sshHost) || !/^[A-Za-z0-9._-]{1,32}$/.test(sshUser) || sshPort < 1 || sshPort > 65535 || tunnelPort < 42000 || tunnelPort > 42999 || !/^[A-Za-z0-9._-]{1,32}$/.test(windowsSshUser) || windowsSshPort < 1 || windowsSshPort > 65535) {
      return response(req, { error: 'Invalid enrollment request' }, 400)
    }
    if (fingerprint && !/^SHA256:[A-Za-z0-9+/=]{43}$/.test(fingerprint)) {
      return response(req, { error: 'Invalid enrollment request' }, 400)
    }
    const activityLogToken = randomToken()
    const activityLogTokenHash = await sha256(activityLogToken)
    const { data, error } = await serviceClient.rpc('consume_ai_support_enrollment', {
      p_token_hash: await sha256(token),
      p_client_id: clientId,
      p_hostname: hostname,
      p_platform: platform,
      p_ssh_host: sshHost,
      p_ssh_port: sshPort,
      p_ssh_user: sshUser,
      p_ssh_key_fingerprint: fingerprint || null,
      p_tunnel_port: tunnelPort,
      p_windows_ssh_user: windowsSshUser,
      p_windows_ssh_port: windowsSshPort,
      p_activity_log_token_hash: activityLogTokenHash,
    })
    if (error || !data?.[0]) {
      console.error('AI-support enrollment failed:', error)
      return response(req, { error: 'Enrollment token is invalid, expired, or already used' }, 409)
    }
    return response(req, {
      status: 'registered',
      client_id: data[0].client_id,
      client_name: data[0].client_name,
      tunnel_port: data[0].tunnel_port,
      activity_log_token: activityLogToken,
    })
  }

  if (body.action === 'log-ai-support') {
    const clientId = typeof body.client_id === 'string' ? body.client_id.trim() : ''
    const logToken = typeof body.log_token === 'string' ? body.log_token.trim() : ''
    const legacyCommand = typeof body.command === 'string' ? body.command.trim().slice(0, 4000) : ''
    const event = typeof body.event === 'string' ? body.event.trim() : ''
    const operation = typeof body.operation === 'string' ? body.operation.trim() : ''
    const mode = typeof body.mode === 'string' ? body.mode.trim() : ''
    const result = typeof body.result === 'string' ? body.result.trim() : ''
    const exitCode = typeof body.exit_code === 'number' && Number.isInteger(body.exit_code) ? body.exit_code : null
    const durationMs = typeof body.duration_ms === 'number' && Number.isInteger(body.duration_ms) ? body.duration_ms : null
    if (!/^ai-[a-z0-9]{8,32}$/.test(clientId) || !/^[A-Za-z0-9_-]{20,200}$/.test(logToken) || (!legacyCommand && event !== 'AI_SUPPORT_OPERATION')) {
      return response(req, { error: 'Invalid activity log request' }, 400)
    }
    const normalizedOperation = legacyCommand ? classifySupportOperation(legacyCommand) : operation
    const normalizedMode = legacyCommand
      ? (legacyCommand === '[interactive shell]' ? 'interactive' : 'command')
      : mode
    const normalizedResult = legacyCommand ? 'unknown' : result
    const validOperations = new Set([
      'interactive_shell', 'system_diagnostics', 'network_diagnostics', 'file_inspection',
      'file_change', 'service_change', 'process_change', 'scheduled_task', 'account_change',
      'remote_access', 'other_powershell',
    ])
    if (!validOperations.has(normalizedOperation) || !['interactive', 'command'].includes(normalizedMode) || !['success', 'failure', 'unknown'].includes(normalizedResult)) {
      return response(req, { error: 'Invalid activity log event' }, 400)
    }
    if (exitCode !== null && (exitCode < 0 || exitCode > 255)) return response(req, { error: 'Invalid activity log exit code' }, 400)
    if (durationMs !== null && (durationMs < 0 || durationMs > 2147483647)) return response(req, { error: 'Invalid activity log duration' }, 400)
    const { error } = await serviceClient.rpc('append_ai_support_log', {
      p_client_id: clientId,
      p_activity_log_token_hash: await sha256(logToken),
      p_event: 'AI_SUPPORT_OPERATION',
      p_details: {
        schema_version: 1,
        mode: normalizedMode,
        operation: normalizedOperation,
        result: normalizedResult,
        exit_code: exitCode,
        duration_ms: durationMs,
        compatibility: legacyCommand ? 'legacy' : 'structured',
      },
    })
    if (error) return response(req, { error: 'Activity log credentials are invalid' }, 403)
    return response(req, { status: 'logged' })
  }

  return response(req, { error: 'Unknown action' }, 400)
})
