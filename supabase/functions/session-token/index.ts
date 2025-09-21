// Supabase Edge Function: session-token
// Issues a short-lived remote control session with optional PIN and ICE config
// Runtime: Deno on Supabase Edge Functions

import { createClient } from "jsr:@supabase/supabase-js@2";

// CORS helper
const corsHeaders = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Headers": "authorization, x-client-info, apikey, content-type",
  "Access-Control-Allow-Methods": "*",
};

function jsonResponse(status: number, body: unknown, origin?: string) {
  const headers = new Headers({ "Content-Type": "application/json", ...corsHeaders });
  if (origin) headers.set("Access-Control-Allow-Origin", origin);
  return new Response(JSON.stringify(body, null, 2), { status, headers });
}

function randomPin(): string {
  const n = Math.floor(100000 + Math.random() * 900000);
  return String(n);
}

function randomToken(bytes = 24): string {
  const arr = new Uint8Array(bytes);
  crypto.getRandomValues(arr);
  return btoa(String.fromCharCode(...arr)).replace(/[^a-zA-Z0-9]/g, "").slice(0, bytes * 2);
}

function getIceServers() {
  const stunUrls = [
    "stun:stun.l.google.com:19302",
    "stun:global.stun.twilio.com:3478?transport=udp",
  ];
  const servers: any[] = [{ urls: stunUrls }];

  const turnUrls = Deno.env.get("TURN_URLS"); // e.g. "turn:turn.example.com:3478,turns:turn.example.com:5349"
  const turnUsername = Deno.env.get("TURN_USERNAME");
  const turnCredential = Deno.env.get("TURN_CREDENTIAL");

  if (turnUrls && turnUsername && turnCredential) {
    const urls = turnUrls.split(",").map((s) => s.trim()).filter(Boolean);
    servers.push({ urls, username: turnUsername, credential: turnCredential });
  }

  return servers;
}

Deno.serve(async (req: Request) => {
  if (req.method === "OPTIONS") {
    return new Response("ok", { headers: corsHeaders });
  }

  const origin = req.headers.get("origin") || undefined;

  try {
    const SUPABASE_URL = Deno.env.get("SUPABASE_URL");
    const SERVICE_ROLE_KEY = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY");
    if (!SUPABASE_URL || !SERVICE_ROLE_KEY) {
      return jsonResponse(500, { error: "Missing SUPABASE_URL or SUPABASE_SERVICE_ROLE_KEY env." }, origin);
    }

    const supabase = createClient(SUPABASE_URL, SERVICE_ROLE_KEY, {
      auth: { persistSession: false },
      global: { headers: { "X-Client-Info": "session-token-fn" } },
    });

    const body = await req.json().catch(() => ({}));
    const device_id: string | undefined = body.device_id;
    const created_by: string | undefined = body.created_by;
    const ttl_seconds: number = Math.max(60, Math.min(60 * 60, Number(body.ttl_seconds) || 15 * 60));
    const with_pin: boolean = Boolean(body.with_pin ?? false);

    if (!device_id) {
      return jsonResponse(400, { error: "device_id is required" }, origin);
    }

    const now = new Date();
    const expires_at = new Date(now.getTime() + ttl_seconds * 1000).toISOString();
    const pin = with_pin ? randomPin() : null;
    const token = randomToken(24);

    const insertPayload: any = { created_by, pin, token, expires_at };
    const { data, error } = await supabase
      .from("remote_sessions")
      .insert(insertPayload)
      .select("id, status, pin, token, created_at, expires_at")
      .single();

    if (error) {
      return jsonResponse(500, { error: error.message, code: error.code }, origin);
    }

    const iceServers = getIceServers();

    return jsonResponse(200, {
      session: data,
      iceServers,
      ttl_seconds,
    }, origin);
  } catch (e) {
    return jsonResponse(500, { error: String(e ?? "unknown error") }, origin);
  }
});
