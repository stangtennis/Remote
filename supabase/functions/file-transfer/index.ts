// file-transfer Edge Function
// Purpose: Fallback file transfer when WebRTC data channel is unavailable

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from 'https://esm.sh/@supabase/supabase-js@2'

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

interface FileTransferRequest {
  session_id: string;
  file_name: string;
  chunk_index: number;
  total_chunks: number;
  data: string; // base64 encoded chunk
}

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
