// Supabase Edge Function: File Transfer
// Handles secure file transfers between devices and sessions

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

interface FileTransferRequest {
  sourceDeviceId: string
  targetDeviceId: string
  fileName: string
  fileSize: number
  fileType: string
  transferType: 'upload' | 'download' | 'share'
  sessionId?: string
}

interface FileTransferSession {
  id: string
  sourceDeviceId: string
  targetDeviceId: string
  fileName: string
  fileSize: number
  fileType: string
  status: 'pending' | 'active' | 'completed' | 'failed' | 'cancelled'
  progress: number
  createdAt: string
  updatedAt: string
  expiresAt: string
}

serve(async (req) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    // Create authenticated Supabase client
    const supabaseClient = createClient(
      Deno.env.get('SUPABASE_URL') || 'https://ptrtibzwokjcjjxvjpin.supabase.co',
      Deno.env.get('SERVICE_ROLE_KEY') || Deno.env.get('SUPABASE_ANON_KEY') || '',
      {
        global: {
          headers: {
            Authorization: req.headers.get('Authorization') || ''
          }
        }
      }
    )

    const url = new URL(req.url)
    const path = url.pathname
    const method = req.method

    // Route handling
    if (path.includes('/api/')) {
      const endpoint = path.split('/api/')[1]
      
      switch (endpoint) {
        case 'initiate-transfer':
          if (method === 'POST') {
            return await initiateFileTransfer(req, supabaseClient)
          }
          break
          
        case 'upload-chunk':
          if (method === 'POST') {
            return await uploadFileChunk(req, supabaseClient)
          }
          break
          
        case 'download-chunk':
          if (method === 'GET') {
            return await downloadFileChunk(req, supabaseClient)
          }
          break
          
        case 'transfer-status':
          if (method === 'GET') {
            return await getTransferStatus(req, supabaseClient)
          }
          break
          
        case 'cancel-transfer':
          if (method === 'POST') {
            return await cancelTransfer(req, supabaseClient)
          }
          break
          
        case 'list-transfers':
          if (method === 'GET') {
            return await listTransfers(req, supabaseClient)
          }
          break
          
        case 'share-file':
          if (method === 'POST') {
            return await shareFile(req, supabaseClient)
          }
          break
          
        default:
          return new Response(
            JSON.stringify({ error: 'Endpoint not found' }),
            { status: 404, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
          )
      }
    }

    // Serve file transfer interface
    return new Response(getFileTransferHTML(), {
      headers: { 
        ...corsHeaders, 
        'Content-Type': 'text/html; charset=utf-8',
        'Cache-Control': 'public, max-age=300'
      }
    })

  } catch (error) {
    console.error('File transfer error:', error)
    return new Response(
      JSON.stringify({ error: 'File transfer service error', details: error.message }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
})

async function initiateFileTransfer(req: Request, supabase: any) {
  try {
    const transferRequest: FileTransferRequest = await req.json()
    
    // Validate request
    if (!transferRequest.sourceDeviceId || !transferRequest.targetDeviceId || !transferRequest.fileName) {
      return new Response(
        JSON.stringify({ error: 'Missing required fields' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Verify devices exist and are online
    const { data: devices, error: deviceError } = await supabase
      .from('remote_devices')
      .select('id, is_online, device_name')
      .in('id', [transferRequest.sourceDeviceId, transferRequest.targetDeviceId])

    if (deviceError || !devices || devices.length !== 2) {
      return new Response(
        JSON.stringify({ error: 'Invalid device IDs' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    const offlineDevices = devices.filter(d => !d.is_online)
    if (offlineDevices.length > 0) {
      return new Response(
        JSON.stringify({ 
          error: 'One or more devices are offline',
          offlineDevices: offlineDevices.map(d => d.device_name)
        }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Create transfer session
    const transferSession: FileTransferSession = {
      id: generateTransferSessionId(),
      sourceDeviceId: transferRequest.sourceDeviceId,
      targetDeviceId: transferRequest.targetDeviceId,
      fileName: transferRequest.fileName,
      fileSize: transferRequest.fileSize,
      fileType: transferRequest.fileType,
      status: 'pending',
      progress: 0,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      expiresAt: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString() // 24 hours
    }

    // Store transfer session in database
    const { data, error } = await supabase
      .from('file_transfers')
      .insert(transferSession)
      .select()

    if (error) {
      console.error('Database error:', error)
      return new Response(
        JSON.stringify({ error: 'Failed to create transfer session' }),
        { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Notify target device via Realtime
    await supabase
      .channel(`device_${transferRequest.targetDeviceId}`)
      .send({
        type: 'broadcast',
        event: 'file_transfer_request',
        payload: {
          sessionId: transferSession.id,
          sourceDevice: devices.find(d => d.id === transferRequest.sourceDeviceId)?.device_name,
          fileName: transferRequest.fileName,
          fileSize: transferRequest.fileSize,
          fileType: transferRequest.fileType
        }
      })

    return new Response(
      JSON.stringify({ 
        success: true, 
        sessionId: transferSession.id,
        message: 'File transfer initiated successfully'
      }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )

  } catch (error) {
    console.error('Initiate transfer error:', error)
    return new Response(
      JSON.stringify({ error: 'Failed to initiate transfer' }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
}

async function uploadFileChunk(req: Request, supabase: any) {
  try {
    const formData = await req.formData()
    const sessionId = formData.get('sessionId') as string
    const chunkIndex = parseInt(formData.get('chunkIndex') as string)
    const totalChunks = parseInt(formData.get('totalChunks') as string)
    const fileChunk = formData.get('chunk') as File

    if (!sessionId || !fileChunk) {
      return new Response(
        JSON.stringify({ error: 'Missing session ID or file chunk' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Verify transfer session
    const { data: session, error: sessionError } = await supabase
      .from('file_transfers')
      .select('*')
      .eq('id', sessionId)
      .single()

    if (sessionError || !session) {
      return new Response(
        JSON.stringify({ error: 'Invalid transfer session' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Upload chunk to storage
    const chunkPath = `transfers/${sessionId}/chunk_${chunkIndex.toString().padStart(4, '0')}`
    const { error: uploadError } = await supabase.storage
      .from('file-transfers')
      .upload(chunkPath, fileChunk)

    if (uploadError) {
      console.error('Chunk upload error:', uploadError)
      return new Response(
        JSON.stringify({ error: 'Failed to upload chunk' }),
        { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Update progress
    const progress = Math.round(((chunkIndex + 1) / totalChunks) * 100)
    const status = progress === 100 ? 'completed' : 'active'

    await supabase
      .from('file_transfers')
      .update({ 
        progress, 
        status,
        updatedAt: new Date().toISOString()
      })
      .eq('id', sessionId)

    // Notify progress via Realtime
    await supabase
      .channel(`transfer_${sessionId}`)
      .send({
        type: 'broadcast',
        event: 'transfer_progress',
        payload: {
          sessionId,
          progress,
          status,
          chunkIndex,
          totalChunks
        }
      })

    return new Response(
      JSON.stringify({ 
        success: true, 
        progress,
        status,
        message: `Chunk ${chunkIndex + 1}/${totalChunks} uploaded successfully`
      }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )

  } catch (error) {
    console.error('Upload chunk error:', error)
    return new Response(
      JSON.stringify({ error: 'Failed to upload chunk' }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
}

async function downloadFileChunk(req: Request, supabase: any) {
  try {
    const url = new URL(req.url)
    const sessionId = url.searchParams.get('sessionId')
    const chunkIndex = url.searchParams.get('chunkIndex')

    if (!sessionId || chunkIndex === null) {
      return new Response(
        JSON.stringify({ error: 'Missing session ID or chunk index' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Verify transfer session
    const { data: session, error: sessionError } = await supabase
      .from('file_transfers')
      .select('*')
      .eq('id', sessionId)
      .single()

    if (sessionError || !session) {
      return new Response(
        JSON.stringify({ error: 'Invalid transfer session' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Download chunk from storage
    const chunkPath = `transfers/${sessionId}/chunk_${chunkIndex.padStart(4, '0')}`
    const { data: chunkData, error: downloadError } = await supabase.storage
      .from('file-transfers')
      .download(chunkPath)

    if (downloadError) {
      console.error('Chunk download error:', downloadError)
      return new Response(
        JSON.stringify({ error: 'Failed to download chunk' }),
        { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    return new Response(chunkData, {
      headers: {
        ...corsHeaders,
        'Content-Type': 'application/octet-stream',
        'Content-Disposition': `attachment; filename="chunk_${chunkIndex}"`
      }
    })

  } catch (error) {
    console.error('Download chunk error:', error)
    return new Response(
      JSON.stringify({ error: 'Failed to download chunk' }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
}

async function getTransferStatus(req: Request, supabase: any) {
  try {
    const url = new URL(req.url)
    const sessionId = url.searchParams.get('sessionId')

    if (!sessionId) {
      return new Response(
        JSON.stringify({ error: 'Missing session ID' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    const { data: session, error } = await supabase
      .from('file_transfers')
      .select('*')
      .eq('id', sessionId)
      .single()

    if (error || !session) {
      return new Response(
        JSON.stringify({ error: 'Transfer session not found' }),
        { status: 404, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    return new Response(
      JSON.stringify(session),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )

  } catch (error) {
    console.error('Get transfer status error:', error)
    return new Response(
      JSON.stringify({ error: 'Failed to get transfer status' }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
}

async function cancelTransfer(req: Request, supabase: any) {
  try {
    const { sessionId } = await req.json()

    if (!sessionId) {
      return new Response(
        JSON.stringify({ error: 'Missing session ID' }),
        { status: 400, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Update transfer status
    const { error } = await supabase
      .from('file_transfers')
      .update({ 
        status: 'cancelled',
        updatedAt: new Date().toISOString()
      })
      .eq('id', sessionId)

    if (error) {
      return new Response(
        JSON.stringify({ error: 'Failed to cancel transfer' }),
        { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    // Clean up storage chunks
    try {
      const { data: files } = await supabase.storage
        .from('file-transfers')
        .list(`transfers/${sessionId}`)

      if (files && files.length > 0) {
        const filePaths = files.map(file => `transfers/${sessionId}/${file.name}`)
        await supabase.storage
          .from('file-transfers')
          .remove(filePaths)
      }
    } catch (cleanupError) {
      console.error('Cleanup error:', cleanupError)
    }

    // Notify via Realtime
    await supabase
      .channel(`transfer_${sessionId}`)
      .send({
        type: 'broadcast',
        event: 'transfer_cancelled',
        payload: { sessionId }
      })

    return new Response(
      JSON.stringify({ success: true, message: 'Transfer cancelled successfully' }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )

  } catch (error) {
    console.error('Cancel transfer error:', error)
    return new Response(
      JSON.stringify({ error: 'Failed to cancel transfer' }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
}

async function listTransfers(req: Request, supabase: any) {
  try {
    const url = new URL(req.url)
    const deviceId = url.searchParams.get('deviceId')
    const status = url.searchParams.get('status')

    let query = supabase
      .from('file_transfers')
      .select('*')
      .order('createdAt', { ascending: false })

    if (deviceId) {
      query = query.or(`sourceDeviceId.eq.${deviceId},targetDeviceId.eq.${deviceId}`)
    }

    if (status) {
      query = query.eq('status', status)
    }

    const { data: transfers, error } = await query

    if (error) {
      return new Response(
        JSON.stringify({ error: 'Failed to list transfers' }),
        { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    return new Response(
      JSON.stringify(transfers),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )

  } catch (error) {
    console.error('List transfers error:', error)
    return new Response(
      JSON.stringify({ error: 'Failed to list transfers' }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
}

async function shareFile(req: Request, supabase: any) {
  try {
    const { deviceId, fileName, fileSize, fileType, shareType, expiresIn } = await req.json()

    // Create share session
    const shareId = generateTransferSessionId()
    const expiresAt = new Date(Date.now() + (expiresIn || 24 * 60 * 60 * 1000)).toISOString()

    const shareSession = {
      id: shareId,
      sourceDeviceId: deviceId,
      targetDeviceId: null, // Public share
      fileName,
      fileSize,
      fileType,
      status: 'pending',
      progress: 0,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      expiresAt
    }

    const { error } = await supabase
      .from('file_transfers')
      .insert(shareSession)

    if (error) {
      return new Response(
        JSON.stringify({ error: 'Failed to create share session' }),
        { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
      )
    }

    const shareUrl = `${req.url.split('/api/')[0]}/share/${shareId}`

    return new Response(
      JSON.stringify({ 
        success: true, 
        shareId,
        shareUrl,
        expiresAt,
        message: 'File share created successfully'
      }),
      { headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )

  } catch (error) {
    console.error('Share file error:', error)
    return new Response(
      JSON.stringify({ error: 'Failed to share file' }),
      { status: 500, headers: { ...corsHeaders, 'Content-Type': 'application/json' } }
    )
  }
}

function generateTransferSessionId(): string {
  return 'transfer_' + crypto.randomUUID().replace(/-/g, '')
}

function getFileTransferHTML(): string {
  return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>📁 File Transfer Service</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh; color: #333; padding: 20px;
        }
        .container {
            max-width: 800px; margin: 0 auto;
            background: rgba(255,255,255,0.95); backdrop-filter: blur(10px);
            border-radius: 20px; padding: 40px; box-shadow: 0 8px 32px rgba(0,0,0,0.1);
        }
        h1 { color: #667eea; text-align: center; margin-bottom: 30px; font-size: 2.5rem; }
        .feature-grid {
            display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 20px; margin-top: 30px;
        }
        .feature-card {
            background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
            color: white; padding: 25px; border-radius: 15px;
            text-align: center; transition: transform 0.3s ease;
        }
        .feature-card:hover { transform: translateY(-5px); }
        .feature-icon { font-size: 2rem; margin-bottom: 15px; }
        .feature-title { font-size: 1.2rem; font-weight: 600; margin-bottom: 10px; }
        .feature-desc { font-size: 0.9rem; opacity: 0.9; }
        .api-section {
            margin-top: 40px; padding: 30px;
            background: rgba(102, 126, 234, 0.1); border-radius: 15px;
        }
        .api-title { color: #667eea; font-size: 1.5rem; margin-bottom: 20px; }
        .endpoint {
            background: white; padding: 15px; border-radius: 10px;
            margin-bottom: 15px; border-left: 4px solid #667eea;
        }
        .method { 
            display: inline-block; padding: 4px 8px; border-radius: 4px;
            font-size: 0.8rem; font-weight: 600; margin-right: 10px;
        }
        .post { background: #10b981; color: white; }
        .get { background: #3b82f6; color: white; }
        .endpoint-path { font-family: monospace; color: #667eea; font-weight: 600; }
        .endpoint-desc { margin-top: 8px; color: #666; font-size: 0.9rem; }
    </style>
</head>
<body>
    <div class="container">
        <h1>📁 File Transfer Service</h1>
        <p style="text-align: center; font-size: 1.1rem; color: #666; margin-bottom: 30px;">
            Secure, real-time file transfers between remote desktop devices
        </p>

        <div class="feature-grid">
            <div class="feature-card">
                <div class="feature-icon">🚀</div>
                <div class="feature-title">Fast Transfers</div>
                <div class="feature-desc">Chunked uploads with progress tracking and resume capability</div>
            </div>
            <div class="feature-card">
                <div class="feature-icon">🔒</div>
                <div class="feature-title">Secure</div>
                <div class="feature-desc">End-to-end encryption with session-based access control</div>
            </div>
            <div class="feature-card">
                <div class="feature-icon">📱</div>
                <div class="feature-title">Real-time</div>
                <div class="feature-desc">Live progress updates via Supabase Realtime channels</div>
            </div>
            <div class="feature-card">
                <div class="feature-icon">🌐</div>
                <div class="feature-title">Global</div>
                <div class="feature-desc">Worldwide file sharing with expiration and access controls</div>
            </div>
        </div>

        <div class="api-section">
            <h2 class="api-title">📚 API Endpoints</h2>
            
            <div class="endpoint">
                <span class="method post">POST</span>
                <span class="endpoint-path">/api/initiate-transfer</span>
                <div class="endpoint-desc">Start a new file transfer session between devices</div>
            </div>
            
            <div class="endpoint">
                <span class="method post">POST</span>
                <span class="endpoint-path">/api/upload-chunk</span>
                <div class="endpoint-desc">Upload file chunks with progress tracking</div>
            </div>
            
            <div class="endpoint">
                <span class="method get">GET</span>
                <span class="endpoint-path">/api/download-chunk</span>
                <div class="endpoint-desc">Download file chunks for reconstruction</div>
            </div>
            
            <div class="endpoint">
                <span class="method get">GET</span>
                <span class="endpoint-path">/api/transfer-status</span>
                <div class="endpoint-desc">Get real-time transfer progress and status</div>
            </div>
            
            <div class="endpoint">
                <span class="method post">POST</span>
                <span class="endpoint-path">/api/cancel-transfer</span>
                <div class="endpoint-desc">Cancel active transfer and cleanup resources</div>
            </div>
            
            <div class="endpoint">
                <span class="method get">GET</span>
                <span class="endpoint-path">/api/list-transfers</span>
                <div class="endpoint-desc">List transfer history and active sessions</div>
            </div>
            
            <div class="endpoint">
                <span class="method post">POST</span>
                <span class="endpoint-path">/api/share-file</span>
                <div class="endpoint-desc">Create public file shares with expiration</div>
            </div>
        </div>

        <div style="text-align: center; margin-top: 30px; color: #666;">
            <p>🔧 Part of the Remote Desktop System Edge Functions</p>
            <p style="margin-top: 10px;">
                <a href="/functions/v1/dashboard" style="color: #667eea; text-decoration: none;">← Back to Dashboard</a>
            </p>
        </div>
    </div>
</body>
</html>`
}
