import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

serve(async (req) => {
  // Handle CORS preflight requests
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    const url = new URL(req.url)
    let path = url.pathname.replace('/functions/v1/static-proxy/', '')
    
    // Remove leading/trailing slashes
    path = path.replace(/^\/+|\/+$/g, '')
    
    console.log('Static proxy request path:', path)
    
    // Map paths to static files
    let fileName = ''
    switch (path) {
      case '':
      case 'dashboard':
        fileName = 'dashboard.html'
        break
      case 'agent-generator':
        fileName = 'agent-generator.html'
        break
      case 'device-manager':
        fileName = 'device-manager.html'
        break
      default:
        console.log('Unknown path:', path)
        return new Response(`Path not found: ${path}`, { 
          status: 404, 
          headers: { ...corsHeaders, 'Content-Type': 'text/plain' }
        })
    }

    // Fetch the static file from Storage
    const storageUrl = `https://ptrtibzwokjcjjxvjpin.supabase.co/storage/v1/object/public/web-assets/${fileName}`
    
    const response = await fetch(storageUrl)
    if (!response.ok) {
      return new Response('File not found', { 
        status: 404, 
        headers: { ...corsHeaders, 'Content-Type': 'text/plain' }
      })
    }

    const htmlContent = await response.text()

    // Return with correct Content-Type header
    return new Response(htmlContent, {
      headers: { 
        ...corsHeaders, 
        'Content-Type': 'text/html; charset=utf-8',
        'Cache-Control': 'public, max-age=300'
      }
    })

  } catch (error) {
    console.error('Static proxy error:', error)
    return new Response('Internal Server Error', { 
      status: 500, 
      headers: { ...corsHeaders, 'Content-Type': 'text/plain' }
    })
  }
})
