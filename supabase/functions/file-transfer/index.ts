// file-transfer Edge Function
// Purpose: Fallback file transfer when WebRTC data channel is unavailable

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': 'https://dashboard.hawkeye123.dk',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

interface FileTransferRequest {
  session_id: string;
  file_name: string;
  chunk_index: number;
  total_chunks: number;
  data: string; // base64 encoded chunk
}

// Resource limits to prevent abuse of the fallback file-transfer path.
const MAX_CHUNKS = 2048           // hard cap on total_chunks per file
const MAX_CHUNK_BYTES = 4 * 1024 * 1024 // max decoded size per chunk (4 MB)
const MAX_FILE_NAME_LEN = 255

serve(async (req) => {
  // Handle CORS preflight
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

    if (req.method === 'POST') {
      // Upload file chunk
      const {
        session_id,
        file_name,
        chunk_index,
        total_chunks,
        data,
      }: FileTransferRequest = await req.json()

      if (!session_id || !file_name || chunk_index === undefined || !data) {
        throw new Error('Missing required fields')
      }

      // Enforce resource limits before touching storage.
      if (total_chunks < 1 || total_chunks > MAX_CHUNKS) {
        throw new Error(`total_chunks out of range (1..${MAX_CHUNKS})`)
      }
      if (chunk_index < 0 || chunk_index >= total_chunks) {
        throw new Error('chunk_index out of range')
      }
      if (file_name.length > MAX_FILE_NAME_LEN || /[\\/]|\.\./.test(file_name)) {
        throw new Error('Invalid file_name')
      }
      // base64 length -> approximate decoded byte length (4/3 ratio).
      const decodedLen = Math.floor((data.length * 3) / 4)
      if (decodedLen > MAX_CHUNK_BYTES) {
        throw new Error(`Chunk too large (max ${MAX_CHUNK_BYTES} bytes)`)
      }

      // Verify session ownership
      const { data: session, error: sessionError } = await supabaseClient
        .from('remote_sessions')
        .select('id, created_by, status')
        .eq('id', session_id)
        .single()

      if (sessionError || !session || session.created_by !== user.id) {
        throw new Error('Invalid session')
      }

      if (session.status !== 'active' && session.status !== 'pending') {
        throw new Error('Session is not active')
      }

      // Upload chunk to Storage
      const chunkPath = `file-transfers/${session_id}/${file_name}.chunk${chunk_index}`
      const chunkData = Uint8Array.from(atob(data), c => c.charCodeAt(0))

      const { error: uploadError } = await supabaseClient.storage
        .from('file-transfers')
        .upload(chunkPath, chunkData, {
          contentType: 'application/octet-stream',
          upsert: true,
        })

      if (uploadError) {
        throw uploadError
      }

      // If this is the last chunk, create metadata file
      if (chunk_index === total_chunks - 1) {
        const metadata = {
          file_name,
          total_chunks,
          session_id,
          uploaded_at: new Date().toISOString(),
        }

        const metadataPath = `file-transfers/${session_id}/${file_name}.meta`
        await supabaseClient.storage
          .from('file-transfers')
          .upload(
            metadataPath,
            new TextEncoder().encode(JSON.stringify(metadata)),
            { contentType: 'application/json', upsert: true }
          )
      }

      return new Response(
        JSON.stringify({
          success: true,
          chunk_index,
          total_chunks,
        }),
        {
          headers: { ...corsHeaders, 'Content-Type': 'application/json' },
          status: 200,
        }
      )
    } else if (req.method === 'GET') {
      // Download file chunks
      const url = new URL(req.url)
      const session_id = url.searchParams.get('session_id')
      const file_name = url.searchParams.get('file_name')

      if (!session_id || !file_name) {
        throw new Error('Missing session_id or file_name')
      }

      // Verify session ownership
      const { data: session, error: sessionError } = await supabaseClient
        .from('remote_sessions')
        .select('id, created_by')
        .eq('id', session_id)
        .single()

      if (sessionError || !session || session.created_by !== user.id) {
        throw new Error('Invalid session')
      }

      // Get metadata
      const metadataPath = `file-transfers/${session_id}/${file_name}.meta`
      const { data: metadataBlob, error: metaError } = await supabaseClient.storage
        .from('file-transfers')
        .download(metadataPath)

      if (metaError) {
        throw new Error('File not found')
      }

      const metadataText = await metadataBlob.text()
      const metadata = JSON.parse(metadataText)

      // Get all chunks
      const chunks = []
      for (let i = 0; i < metadata.total_chunks; i++) {
        const chunkPath = `file-transfers/${session_id}/${file_name}.chunk${i}`
        const { data: chunkBlob, error: chunkError } = await supabaseClient.storage
          .from('file-transfers')
          .download(chunkPath)

        if (chunkError) {
          throw new Error(`Chunk ${i} not found`)
        }

        const arrayBuffer = await chunkBlob.arrayBuffer()
        const base64 = btoa(String.fromCharCode(...new Uint8Array(arrayBuffer)))
        chunks.push(base64)
      }

      return new Response(
        JSON.stringify({
          file_name: metadata.file_name,
          total_chunks: metadata.total_chunks,
          chunks,
        }),
        {
          headers: { ...corsHeaders, 'Content-Type': 'application/json' },
          status: 200,
        }
      )
    }

    throw new Error('Method not allowed')
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
