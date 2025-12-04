// Send Welcome Email Edge Function
// Sends login credentials to newly approved users

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

// Resend API for sending emails (free tier: 100 emails/day)
const RESEND_API_KEY = Deno.env.get('RESEND_API_KEY')

interface EmailRequest {
  email: string
  tempPassword?: string
}

serve(async (req) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    const supabaseClient = createClient(
      Deno.env.get('SUPABASE_URL') ?? '',
      Deno.env.get('SUPABASE_SERVICE_ROLE_KEY') ?? ''
    )

    // Verify the request is from an authenticated admin
    const authHeader = req.headers.get('Authorization')
    if (!authHeader) {
      throw new Error('No authorization header')
    }

    const token = authHeader.replace('Bearer ', '')
    const { data: { user }, error: authError } = await supabaseClient.auth.getUser(token)
    
    if (authError || !user) {
      throw new Error('Unauthorized')
    }

    // Check if user is admin
    const { data: approval } = await supabaseClient
      .from('user_approvals')
      .select('role')
      .eq('user_id', user.id)
      .single()

    if (!approval || approval.role !== 'admin') {
      throw new Error('Admin access required')
    }

    // Get request body
    const { email, tempPassword }: EmailRequest = await req.json()

    if (!email) {
      throw new Error('Email is required')
    }

    // Check if Resend API key is configured
    if (!RESEND_API_KEY) {
      console.log('RESEND_API_KEY not configured, skipping email send')
      return new Response(
        JSON.stringify({ 
          success: false, 
          message: 'Email service not configured. Please set RESEND_API_KEY.' 
        }),
        { 
          headers: { ...corsHeaders, 'Content-Type': 'application/json' },
          status: 200 
        }
      )
    }

    // Send welcome email via Resend
    const emailResponse = await fetch('https://api.resend.com/emails', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${RESEND_API_KEY}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        from: 'Remote Desktop <noreply@hawkeye123.dk>',
        to: [email],
        subject: '✅ Your Remote Desktop Account is Approved!',
        html: `
          <!DOCTYPE html>
          <html>
          <head>
            <style>
              body { font-family: 'Segoe UI', Arial, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 20px; }
              .container { max-width: 600px; margin: 0 auto; background: #1e293b; border-radius: 16px; padding: 40px; }
              .header { text-align: center; margin-bottom: 30px; }
              .logo { font-size: 48px; margin-bottom: 10px; }
              h1 { color: #6366f1; margin: 0; font-size: 28px; }
              .content { line-height: 1.6; }
              .credentials { background: #334155; border-radius: 12px; padding: 20px; margin: 20px 0; }
              .credentials p { margin: 10px 0; }
              .label { color: #94a3b8; font-size: 14px; }
              .value { color: #f8fafc; font-weight: bold; font-size: 18px; }
              .button { display: inline-block; background: linear-gradient(135deg, #6366f1, #8b5cf6); color: white; padding: 14px 28px; border-radius: 8px; text-decoration: none; font-weight: bold; margin-top: 20px; }
              .footer { text-align: center; margin-top: 30px; color: #64748b; font-size: 14px; }
              .warning { background: #fef3c7; color: #92400e; padding: 15px; border-radius: 8px; margin-top: 20px; }
            </style>
          </head>
          <body>
            <div class="container">
              <div class="header">
                <div class="logo">🖥️</div>
                <h1>Welcome to Remote Desktop!</h1>
              </div>
              
              <div class="content">
                <p>Great news! Your account has been approved by an administrator.</p>
                <p>You can now log in and start sharing your screen securely.</p>
                
                <div class="credentials">
                  <p><span class="label">Email:</span><br><span class="value">${email}</span></p>
                  ${tempPassword ? `<p><span class="label">Temporary Password:</span><br><span class="value">${tempPassword}</span></p>` : ''}
                </div>
                
                <p style="text-align: center;">
                  <a href="https://stangtennis.github.io/Remote/agent.html" class="button">
                    🚀 Login Now
                  </a>
                </p>
                
                ${tempPassword ? `
                <div class="warning">
                  ⚠️ <strong>Security Notice:</strong> Please change your password after your first login.
                </div>
                ` : ''}
              </div>
              
              <div class="footer">
                <p>🔒 All connections are encrypted end-to-end</p>
                <p>Remote Desktop © 2025</p>
              </div>
            </div>
          </body>
          </html>
        `,
      }),
    })

    if (!emailResponse.ok) {
      const errorData = await emailResponse.json()
      console.error('Resend API error:', errorData)
      throw new Error(`Failed to send email: ${errorData.message || 'Unknown error'}`)
    }

    const result = await emailResponse.json()
    console.log('Email sent successfully:', result)

    return new Response(
      JSON.stringify({ success: true, message: 'Welcome email sent!', id: result.id }),
      { 
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
        status: 200 
      }
    )

  } catch (error) {
    console.error('Error:', error.message)
    return new Response(
      JSON.stringify({ success: false, error: error.message }),
      { 
        headers: { ...corsHeaders, 'Content-Type': 'application/json' },
        status: 400 
      }
    )
  }
})
