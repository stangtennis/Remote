// Create Support Session Edge Function
// POST, requires auth (admin). Generates PIN + token for Quick Support.

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

serve(async (req) => {
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

    // Check if user is admin
    const { data: userApproval } = await supabaseClient
      .from('user_approvals')
      .select('role')
      .eq('user_id', user.id)
      .single()

    const isAdmin = userApproval?.role === 'admin' || userApproval?.role === 'super_admin'
    if (!isAdmin) {
      throw new Error('Admin access required')
    }

    // Generate 6-digit PIN
    const pin = Math.floor(100000 + Math.random() * 900000).toString()

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
      })
      .select()
      .single()

    if (sessionError) {
      console.error('Failed to create support session:', sessionError)
      throw sessionError
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
