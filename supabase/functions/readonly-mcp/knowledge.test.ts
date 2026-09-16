import {
  validateKnowledgeDraftArgs,
  validateKnowledgePublishArgs,
} from './knowledge.ts'

Deno.test('knowledge drafts default to general and unpublished input', () => {
  const result = validateKnowledgeDraftArgs({ title: 'Network recovery', content: 'Restart the tunnel.' })
  if (result.category !== 'general' || result.tags.length !== 0 || result.source !== null) {
    throw new Error(`unexpected defaults: ${JSON.stringify(result)}`)
  }
})

Deno.test('knowledge draft validation enforces bounds and categories', () => {
  for (const args of [
    { content: 'missing title' },
    { title: 'bad category', content: 'text', category: 'secrets' },
    { title: 'bad fields', content: 'text', extra: true },
    { title: 'secret', content: 'token=abc123' },
  ]) {
    try {
      validateKnowledgeDraftArgs(args)
      throw new Error(`expected validation failure for ${JSON.stringify(args)}`)
    } catch (error) {
      if (!(error instanceof Error) || error.message.startsWith('expected validation failure')) throw error
    }
  }
})

Deno.test('knowledge publish requires a UUID and rejects extra fields', () => {
  const id = '123e4567-e89b-12d3-a456-426614174000'
  if (validateKnowledgePublishArgs({ id }).id !== id) throw new Error('valid UUID was rejected')
  for (const args of [{ id: 'not-a-uuid' }, { id, extra: true }]) {
    try {
      validateKnowledgePublishArgs(args)
      throw new Error(`expected validation failure for ${JSON.stringify(args)}`)
    } catch (error) {
      if (!(error instanceof Error) || error.message.startsWith('expected validation failure')) throw error
    }
  }
})
