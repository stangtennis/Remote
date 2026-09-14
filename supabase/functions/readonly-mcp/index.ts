// Read-only MCP interface for the AI support client.
// Authentication is supplied by the Supabase Edge Function JWT gateway and
// re-used by the user-scoped Supabase client below, so database RLS remains in force.

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

// This stateless endpoint implements the current protocol only. Keeping older
// versions out avoids advertising JSON-RPC features (such as batching) that
// this small transport does not implement.
const MCP_VERSIONS = ['2025-06-18']
const MAX_LIMIT = 100
const MAX_OFFSET = 500

const corsHeaders = {
  'Access-Control-Allow-Origin': 'https://dashboard.hawkeye123.dk',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
  'Access-Control-Allow-Methods': 'POST, OPTIONS',
  'Content-Type': 'application/json',
}

type JsonRpcRequest = {
  jsonrpc: '2.0'
  id?: string | number | null
  method: string
  params?: Record<string, unknown>
}

function jsonRpc(id: JsonRpcRequest['id'], result: unknown, status = 200) {
  return new Response(JSON.stringify({ jsonrpc: '2.0', id, result }), {
    headers: corsHeaders,
    status,
  })
}

function jsonRpcError(id: JsonRpcRequest['id'], code: number, message: string, status = 200) {
  return new Response(JSON.stringify({
    jsonrpc: '2.0',
    id,
    error: { code, message },
  }), {
    headers: corsHeaders,
    status,
  })
}

function clampLimit(value: unknown) {
  if (value === undefined) return 20
  if (typeof value !== 'number' || !Number.isInteger(value)) throw new Error('limit must be an integer')
  const parsed = value
  return Math.max(1, Math.min(MAX_LIMIT, Math.floor(parsed)))
}

function clampOffset(value: unknown) {
  if (value === undefined) return 0
  if (typeof value !== 'number' || !Number.isInteger(value)) throw new Error('offset must be an integer')
  const parsed = value
  return Math.max(0, Math.min(MAX_OFFSET, Math.floor(parsed)))
}

function safeText(value: unknown, maxLength = 500) {
  if (typeof value !== 'string') return null
  return value
    .replace(/https?:\/\/[^\s/:@]+:[^\s/]+@/gi, 'https://[REDACTED]@')
    .replace(/(password|passwd|token|secret|credential|apikey)\s*[:=]\s*[^\s,;]+/gi, '$1=[REDACTED]')
    .replace(/(api[_-]?key|access[_-]?token|authorization)\s*[:=]\s*[^\s,;]+/gi, '$1=[REDACTED]')
    .replace(/["']?(password|passwd|token|secret|credential|api[_-]?key|access[_-]?token|authorization)["']?\s*[:=]\s*["']?[^"',;\s}]+["']?/gi, '$1=[REDACTED]')
    .replace(/Bearer\s+[A-Za-z0-9._~+/=-]+/gi, 'Bearer [REDACTED]')
    .replace(/\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/g, '[REDACTED]')
    .replace(/\b[A-Z][A-Z0-9_]*(?:KEY|TOKEN|SECRET|PASSWORD)\s*=\s*[^\s,;]+/g, '[REDACTED_ENV]')
    .replace(/\b(?:sk|rk|pk)_[A-Za-z0-9_-]{16,}\b/g, '[REDACTED_KEY]')
    .replace(/\b[A-Fa-f0-9]{64}\b/g, '[REDACTED_HEX_SECRET]')
    .replace(/\b[A-Za-z0-9_-]{43}\b/g, '[REDACTED_BEARER]')
    .replace(/-----BEGIN [^-]+-----[\s\S]*?-----END [^-]+-----/g, '[REDACTED_KEY]')
    .slice(0, maxLength)
}

function safeDetails(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const allowed = ['device_name', 'old_name', 'new_name', 'exit_code', 'duration_ms', 'command_length', 'operation', 'command']
  return Object.fromEntries(
    Object.entries(value)
      .filter(([key]) => allowed.includes(key))
      .map(([key, item]) => {
        if (typeof item === 'string') return [key, safeText(item, key === 'command' ? 800 : 200)]
        if (typeof item === 'number' && Number.isFinite(item)) return [key, item]
        if (typeof item === 'boolean') return [key, item]
        return null
      })
      .filter((entry): entry is [string, string | number | boolean | null] => entry !== null)
  )
}

function eventSummary(event: string, details: unknown) {
  const labels: Record<string, string> = {
    DEVICE_REGISTERED: 'Klient registreret',
    DEVICE_RENAMED: 'Klient omdøbt',
    DEVICE_ONLINE: 'Klient online',
    DEVICE_OFFLINE: 'Klient offline',
    DEVICE_DELETED: 'Klient slettet',
    SESSION_CREATED: 'Fjernsession startet',
    SESSION_ENDED: 'Fjernsession afsluttet',
    SHELL_EXEC: 'Shell-handling registreret',
    AI_SUPPORT_COMMAND: 'AI-kommando på supportklient registreret',
  }
  if (event === 'DEVICE_RENAMED' && details && typeof details === 'object') {
    const oldName = safeText((details as Record<string, unknown>).old_name, 100)
    const newName = safeText((details as Record<string, unknown>).new_name, 100)
    if (oldName && newName) return `Klient omdøbt: ${oldName} -> ${newName}`
  }
  return labels[event] || event
}

function safeDevice(device: any) {
  return {
    device_id: safeText(device.device_id, 200),
    device_name: safeText(device.device_name, 200),
    platform: safeText(device.platform, 100),
    arch: safeText(device.arch, 50),
    is_online: device.is_online === true,
    last_seen: device.last_seen,
    status: safeText(device.status, 50),
    lifecycle_status: safeText(device.lifecycle_status, 50),
    approved: device.approved === true,
    agent_version: safeText(device.agent_version, 100),
    cpu_percent: device.cpu_percent,
    memory_used_mb: device.memory_used_mb,
    memory_total_mb: device.memory_total_mb,
    disk_used_gb: device.disk_used_gb,
    disk_total_gb: device.disk_total_gb,
  }
}

function safeAISupportClient(client: any) {
  return {
    client_id: safeText(client.client_id, 100),
    client_name: safeText(client.client_name, 200),
    hostname: safeText(client.hostname, 120),
    platform: safeText(client.platform, 80),
    status: safeText(client.status, 50),
    tunnel_port: client.tunnel_port,
    windows_ssh_user: safeText(client.windows_ssh_user, 80),
    windows_ssh_port: client.windows_ssh_port,
    last_seen: client.last_seen,
    created_at: client.created_at,
  }
}

function toolDefinitions() {
  return [
    {
      name: 'list_clients',
      description: 'List the authenticated user\'s remote clients with safe status information.',
      inputSchema: {
        type: 'object',
        properties: {
          online_only: { type: 'boolean', description: 'Only return clients currently marked online.' },
          limit: { type: 'integer', minimum: 1, maximum: MAX_LIMIT },
        },
      },
    },
    {
      name: 'support_context',
      description: 'Build the bounded AI-support context for a client: current status, recent actions, and optional published troubleshooting knowledge.',
      inputSchema: {
        type: 'object',
        required: ['client_id'],
        properties: {
          client_id: { type: 'string', minLength: 1, maxLength: 200 },
          knowledge_query: { type: 'string', minLength: 1, maxLength: 200 },
        },
      },
    },
    {
      name: 'client_status',
      description: 'Get safe current status for one remote client.',
      inputSchema: {
        type: 'object',
        required: ['device_id'],
        properties: { device_id: { type: 'string', minLength: 1, maxLength: 200 } },
      },
    },
    {
      name: 'client_history',
      description: 'Read a bounded, redacted activity history for one remote client.',
      inputSchema: {
        type: 'object',
        required: ['device_id'],
        properties: {
          device_id: { type: 'string', minLength: 1, maxLength: 200 },
          since: { type: 'string', description: 'Optional ISO-8601 lower time bound.' },
          offset: { type: 'integer', minimum: 0, maximum: MAX_OFFSET },
          limit: { type: 'integer', minimum: 1, maximum: MAX_LIMIT },
        },
      },
    },
    {
      name: 'knowledge_search',
      description: 'Search published support runbooks and troubleshooting knowledge.',
      inputSchema: {
        type: 'object',
        required: ['query'],
        properties: {
          query: { type: 'string', minLength: 1, maxLength: 200 },
          category: { type: 'string', enum: ['general', 'installation', 'network', 'windows', 'webrtc', 'security', 'runbook'] },
          limit: { type: 'integer', minimum: 1, maximum: MAX_LIMIT },
        },
      },
    },
  ]
}

async function listClients(supabase: any, args: Record<string, unknown>) {
  let query = supabase
    .from('remote_devices')
    .select('device_id, device_name, platform, arch, is_online, last_seen, status, lifecycle_status, approved, agent_version, cpu_percent, memory_used_mb, memory_total_mb, disk_used_gb, disk_total_gb')
    .order('device_name', { ascending: true })
    .limit(clampLimit(args.limit))
  if (args.online_only === true) query = query.eq('is_online', true)
  const [devicesResult, aiClientsResult] = await Promise.all([
    query,
    supabase
      .from('ai_support_clients')
      .select('client_id, client_name, hostname, platform, status, tunnel_port, windows_ssh_user, windows_ssh_port, last_seen, created_at')
      .order('client_name', { ascending: true })
      .limit(clampLimit(args.limit)),
  ])
  if (devicesResult.error) throw devicesResult.error
  if (aiClientsResult.error) console.warn('AI-support client list unavailable:', aiClientsResult.error.message)
  return [
    ...(devicesResult.data || []).map((device: any) => ({ ...safeDevice(device), client_type: 'remote_device' })),
    ...(aiClientsResult.data || []).map((client: any) => ({ ...safeAISupportClient(client), client_type: 'ai_support' })),
  ]
}

async function clientStatus(supabase: any, args: Record<string, unknown>) {
  const deviceId = typeof args.device_id === 'string' ? args.device_id.trim() : ''
  if (!deviceId || deviceId.length > 200) throw new Error('device_id is required and must be at most 200 characters')
  if (deviceId.startsWith('ai-')) {
    const { data, error } = await supabase
      .from('ai_support_clients')
      .select('client_id, client_name, hostname, platform, status, tunnel_port, windows_ssh_user, windows_ssh_port, last_seen, created_at')
      .eq('client_id', deviceId)
      .maybeSingle()
    if (error) throw error
    if (!data) throw new Error('AI-support client not found or not accessible')
    return { ...safeAISupportClient(data), client_type: 'ai_support' }
  }
  const { data, error } = await supabase
    .from('remote_devices')
    .select('device_id, device_name, platform, arch, is_online, last_seen, status, lifecycle_status, approved, agent_version, cpu_percent, memory_used_mb, memory_total_mb, disk_used_gb, disk_total_gb')
    .eq('device_id', deviceId)
    .maybeSingle()
  if (error) throw error
  if (!data) throw new Error('Client not found or not accessible')
  return safeDevice(data)
}

async function supportContext(supabase: any, args: Record<string, unknown>) {
  const clientId = typeof args.client_id === 'string' ? args.client_id.trim() : ''
  if (!clientId || clientId.length > 200) throw new Error('client_id is required and must be at most 200 characters')
  const [status, history] = await Promise.all([
    clientStatus(supabase, { device_id: clientId }),
    clientHistory(supabase, { device_id: clientId, limit: 20, offset: 0 }),
  ])
  let knowledge = []
  if (typeof args.knowledge_query === 'string' && args.knowledge_query.trim()) {
    knowledge = await knowledgeSearch(supabase, { query: args.knowledge_query.trim(), limit: 10 })
  }
  return { client: status, recent_history: history, knowledge }
}

async function clientHistory(supabase: any, args: Record<string, unknown>) {
  const deviceId = typeof args.device_id === 'string' ? args.device_id.trim() : ''
  if (!deviceId || deviceId.length > 200) throw new Error('device_id is required and must be at most 200 characters')
  const limit = clampLimit(args.limit)
  const offset = clampOffset(args.offset)
  let since: string | null = null
  if (args.since !== undefined) {
    const parsed = new Date(String(args.since))
    if (Number.isNaN(parsed.getTime())) throw new Error('since must be a valid ISO-8601 timestamp')
    since = parsed.toISOString()
  }

  let auditQuery = supabase
    .from('audit_logs')
    .select('device_id, event, severity, details, created_at')
    .eq('device_id', deviceId)
    .order('created_at', { ascending: false })
    .limit(limit + offset)
  if (since) auditQuery = auditQuery.gte('created_at', since)

  let supportQuery = supabase
    .from('support_action_audit')
    .select('device_id, action_type, actor_type, status, summary, details, verified, created_at')
    .eq('device_id', deviceId)
    .order('created_at', { ascending: false })
    .limit(limit + offset)
  if (since) supportQuery = supportQuery.gte('created_at', since)

  let commandQuery = supabase
    .from('device_commands')
    .select('device_id, command_type, status, created_at, completed_at')
    .eq('device_id', deviceId)
    .order('created_at', { ascending: false })
    .limit(limit + offset)
  if (since) commandQuery = commandQuery.gte('created_at', since)

  const [auditResult, supportResult, commandResult] = await Promise.all([auditQuery, supportQuery, commandQuery])
  if (auditResult.error) throw auditResult.error
  const unavailableHistory = supportResult.error || commandResult.error
  const partial = Boolean(unavailableHistory)
  if (partial) console.warn('Support action history unavailable:', unavailableHistory.message)

  const auditItems = (auditResult.data || []).map((item: any) => ({
    source: 'audit',
    event: item.event,
    summary: eventSummary(item.event, item.details),
    severity: item.severity,
    details: safeDetails(item.details),
    created_at: item.created_at,
  }))
  const supportItems = (supportResult.data || []).map((item: any) => ({
    source: 'support_action',
    event: item.action_type,
    actor: item.actor_type,
    status: item.status,
    summary: safeText(item.summary, 500),
    verified: item.verified === true,
    details: safeDetails(item.details),
    created_at: item.created_at,
  }))
  const commandItems = (commandResult.data || []).map((item: any) => ({
    source: 'command',
    event: `REMOTE_COMMAND_${String(item.status || 'unknown').toUpperCase()}`,
    summary: `Remote kommando: ${safeText(item.command_type, 80) || 'ukendt'}`,
    status: item.status,
    created_at: item.created_at,
    completed_at: item.completed_at,
  }))

  return {
    partial,
    history_scope: 'Results are limited by the authenticated user\'s RLS permissions. Support-session history can be absent for assigned users or sessions created by another admin.',
    warning: partial ? 'Support action history is not available for this user.' : null,
    entries: [...auditItems, ...supportItems, ...commandItems]
    .sort((left, right) => new Date(right.created_at).getTime() - new Date(left.created_at).getTime())
      .slice(offset, offset + limit),
  }
}

async function knowledgeSearch(supabase: any, args: Record<string, unknown>) {
  const queryText = typeof args.query === 'string' ? args.query.trim().slice(0, 200) : ''
  if (!queryText) throw new Error('query is required')
  const categories = ['general', 'installation', 'network', 'windows', 'webrtc', 'security', 'runbook']
  const category = typeof args.category === 'string' && args.category ? args.category : null
  if (category && !categories.includes(category)) throw new Error('Invalid knowledge category')
  const pattern = `%${queryText.replace(/[%_]/g, ' ')}%`
  const columns = ['title', 'summary', 'content']
  const results = await Promise.all(columns.map((column) => {
    let query = supabase
      .from('support_knowledge')
      .select('id, title, summary, content, category, tags, source, updated_at')
      .eq('is_published', true)
      .ilike(column, pattern)
      .limit(MAX_LIMIT)
    if (category) query = query.eq('category', category)
    return query
  }))
  const failed = results.find((result: any) => result.error)
  if (failed?.error) throw failed.error
  const matches = new Map<string, any>()
  results.flatMap((result: any) => result.data || []).forEach((item: any) => matches.set(item.id, item))
  return [...matches.values()]
    .sort((left, right) => new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime())
    .slice(0, clampLimit(args.limit))
    .map((item: any) => ({
      id: item.id,
      title: safeText(item.title, 200),
      summary: safeText(item.summary, 500),
      snippet: safeText(item.content, 800),
      category: item.category,
      tags: Array.isArray(item.tags)
        ? item.tags.filter((tag: unknown): tag is string => typeof tag === 'string').map((tag: string) => safeText(tag, 100)).slice(0, 20)
        : [],
      source: safeText(item.source, 300),
      updated_at: item.updated_at,
    }))
}

async function callTool(supabase: any, name: string, args: Record<string, unknown>) {
  switch (name) {
    case 'list_clients': return listClients(supabase, args)
    case 'client_status': return clientStatus(supabase, args)
    case 'client_history': return clientHistory(supabase, args)
    case 'support_context': return supportContext(supabase, args)
    case 'knowledge_search': return knowledgeSearch(supabase, args)
    default: throw new Error(`Unknown tool: ${name}`)
  }
}

serve(async (req) => {
  if (req.method === 'OPTIONS') return new Response('ok', { headers: corsHeaders })
  if (req.method !== 'POST') return new Response('Method Not Allowed', { status: 405, headers: corsHeaders })

  const authHeader = req.headers.get('Authorization')
  if (!authHeader?.startsWith('Bearer ')) return jsonRpcError(null, -32001, 'Authentication required', 401)

  let body: JsonRpcRequest
  let parsed: unknown
  try {
    parsed = await req.json()
  } catch {
    return jsonRpcError(null, -32700, 'Parse error', 400)
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
    return jsonRpcError(null, -32600, 'Invalid JSON-RPC request', 400)
  }
  const request = parsed as Record<string, unknown>
  const validId = request.id === undefined || request.id === null ||
    typeof request.id === 'string' ||
    (typeof request.id === 'number' && Number.isFinite(request.id))
  const validParams = request.params === undefined ||
    (typeof request.params === 'object' && request.params !== null && !Array.isArray(request.params))
  if (request.jsonrpc !== '2.0' || typeof request.method !== 'string' || !request.method || !validId || !validParams) {
    return jsonRpcError(null, -32600, 'Invalid JSON-RPC request', 400)
  }
  body = request as unknown as JsonRpcRequest

  const id = body.id ?? null
  if (!body.method) return jsonRpcError(id, -32600, 'Invalid JSON-RPC request')
  if (body.method === 'notifications/initialized') return new Response(null, { status: 202, headers: corsHeaders })
  if (body.id === undefined) return new Response(null, { status: 202, headers: corsHeaders })

  const supabase = createClient(
    Deno.env.get('SUPABASE_URL') ?? '',
    Deno.env.get('SUPABASE_ANON_KEY') ?? '',
    { global: { headers: { Authorization: authHeader } } },
  )
  const { data: { user }, error: userError } = await supabase.auth.getUser()
  if (userError || !user) return jsonRpcError(id, -32001, 'Unauthorized', 401)

  const { data: approval, error: approvalError } = await supabase
    .from('user_approvals')
    .select('approved')
    .eq('user_id', user.id)
    .maybeSingle()
  if (approvalError || approval?.approved !== true) return jsonRpcError(id, -32003, 'Approved user access required', 403)

  try {
    if (body.method === 'initialize') {
      const requestedVersion = typeof body.params?.protocolVersion === 'string'
        ? body.params.protocolVersion
        : null
      const protocolVersion = requestedVersion && MCP_VERSIONS.includes(requestedVersion)
        ? requestedVersion
        : requestedVersion
          ? null
          : MCP_VERSIONS[0]
      if (!protocolVersion) return jsonRpcError(id, -32602, 'Unsupported MCP protocol version')
      return jsonRpc(id, {
        protocolVersion,
        capabilities: { tools: { listChanged: false } },
        serverInfo: { name: 'remote-desktop-support', version: '1.0.0' },
      })
    }
    if (body.method === 'tools/list') return jsonRpc(id, { tools: toolDefinitions() })
    if (body.method === 'tools/call') {
      const name = typeof body.params?.name === 'string' ? body.params.name : ''
      if (!name || !toolDefinitions().some((tool) => tool.name === name)) {
        return jsonRpcError(id, -32602, 'Unknown or missing tool name')
      }
      const args = body.params?.arguments && typeof body.params.arguments === 'object'
        ? body.params.arguments as Record<string, unknown>
        : {}
      try {
        const result = await callTool(supabase, name, args)
        return jsonRpc(id, {
          content: [{ type: 'text', text: JSON.stringify(result, null, 2) }],
          isError: false,
        })
      } catch (error) {
        throw error
      }
    }
    return jsonRpcError(id, -32601, `Method not found: ${body.method}`)
  } catch (error) {
    console.error('Read-only MCP request failed:', error)
    return jsonRpc(id, {
      content: [{ type: 'text', text: 'Tool request failed' }],
      isError: true,
    })
  }
})
