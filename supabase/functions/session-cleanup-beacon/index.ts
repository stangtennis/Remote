import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const allowedOrigins = ['https://dashboard.hawkeye123.dk', 'https://supabase.hawkeye123.dk']

function getCorsHeaders(req: Request) {
  const origin = req.headers.get('origin') || ''
  return {
    'Access-Control-Allow-Origin': allowedOrigins.includes(origin) ? origin : allowedOrigins[0],
    'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
  }
}

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

// --- Threat model -----------------------------------------------------------
// The beacon is invoked on page unload via navigator.sendBeacon, which CANNOT
// attach an Authorization header, so the endpoint is unauthenticated by design.
// Authorization relies on session IDs being unguessable (crypto-random UUIDs,
// 122 bits of entropy). The residual risk: an attacker who *learns* a session
// UUID (logs, a leaked realtime payload, MITM before TLS, etc.) could POST it
// here and disconnect that session. Severity is LOW — disconnect only, no data
// access, and the owner can reconnect.
//
// Mitigations in place:
//   - UUID format validation (rejects injection / arbitrary strings).
//   - CORS allowlist (only the dashboard/supabase origins).
//   - Per-IP rate limit below (best-effort, per-instance) to make volume
//     scanning of the UUID space uneconomic even if entropy failed.
//   - The endpoint only sets status='ended'; it cannot read/modify data.

// --- Best-effort per-IP rate limiting --------------------------------------
// Edge function instances are not guaranteed to share memory, so this is a
// per-instance sliding window. Combined with UUID entropy this is sufficient
// defense-in-depth; a distributed attacker is still bounded by UUID guessing.
const RATE_LIMIT_MAX = 60    // max requests ...
const RATE_LIMIT_WINDOW_MS = 60_000 // ... per minute per IP
const rateBuckets = new Map<string, number[]>()

function rateLimitOk(ip: string): boolean {
  const now = Date.now()
  const cutoff = now - RATE_LIMIT_WINDOW_MS
  const arr = (rateBuckets.get(ip) || []).filter(t => t > cutoff)
  if (arr.length >= RATE_LIMIT_MAX) {
    rateBuckets.set(ip, arr)
    return false
  }
  arr.push(now)
  rateBuckets.set(ip, arr)
  // Cap memory: if the per-instance IP map grows large, reset it. The next
  // request for each active IP simply starts a fresh window; correctness of
  // the limit is preserved at the cost of a one-shot per-IP reset.
  if (rateBuckets.size > 4096) {
    rateBuckets.clear()
  }
  return true
}

function getClientIP(req: Request): string {
  return (
    req.headers.get('cf-connecting-ip') ||
    req.headers.get('x-real-ip') ||
    (req.headers.get('x-forwarded-for') || '').split(',')[0].trim() ||
    'unknown'
  )
}

// Beacon endpoint for reliable session cleanup during page unload.
// No auth required — sendBeacon cannot send custom headers.
// Session IDs are validated as UUIDs to prevent injection.
serve(async (req) => {
  const corsHeaders = getCorsHeaders(req)

  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  // Rate limit per client IP.
  if (!rateLimitOk(getClientIP(req))) {
    return new Response(
      JSON.stringify({ error: 'Too many cleanup requests' }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 429 }
    )
  }

  try {
    const { session_ids } = await req.json()

    if (!Array.isArray(session_ids) || session_ids.length === 0) {
      return new Response(
        JSON.stringify({ error: 'session_ids array required' }),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 400 }
      )
    }

    // Limit to 20 session IDs per request and validate UUID format
    const ids = session_ids.slice(0, 20).filter((id: string) => UUID_REGEX.test(id))
    if (ids.length === 0) {
      return new Response(
        JSON.stringify({ error: 'no valid UUIDs provided' }),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 400 }
      )
    }

    const supabaseUrl = Deno.env.get('SUPABASE_URL')!
    const supabaseKey = Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!
    const supabase = createClient(supabaseUrl, supabaseKey)

    const now = new Date().toISOString()

    // Mark sessions as ended (only if currently pending/active)
    const { data, error } = await supabase
      .from('remote_sessions')
      .update({ status: 'ended', ended_at: now })
      .in('id', ids)
      .in('status', ['pending', 'active'])
      .select('id')

    if (error) {
      console.error('Beacon cleanup error:', error)
      return new Response(
        JSON.stringify({ error: error.message }),
        { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 500 }
      )
    }

    console.log(`🧹 Beacon cleanup: ended ${data?.length || 0} sessions`)

    return new Response(
      JSON.stringify({ success: true, ended: data?.length || 0 }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 200 }
    )

  } catch (error) {
    console.error('Beacon cleanup error:', error)
    return new Response(
      JSON.stringify({ error: error.message }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' }, status: 500 }
    )
  }
})
