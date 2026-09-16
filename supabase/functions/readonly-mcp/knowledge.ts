export const KNOWLEDGE_CATEGORIES = [
  'general',
  'installation',
  'network',
  'windows',
  'webrtc',
  'security',
  'runbook',
] as const

const MAX_TAGS = 20
const MAX_TAG_LENGTH = 100
const MAX_SOURCE_LENGTH = 300

const forbiddenPatterns = [
  /-----BEGIN [^-]+ PRIVATE KEY-----/i,
  /https?:\/\/[^\s/:@]+:[^\s/]+@/i,
  /\b(?:password|passwd|token|secret|credential|api[_-]?key|access[_-]?token|authorization)\s*[:=]\s*(?:"[^"]*"|'[^']*'|[^\s,;}]+)/i,
  /\bBearer\s+[A-Za-z0-9._~+\/=:-]+/i,
  /\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/,
  /\b(?:sk|rk|pk)_[A-Za-z0-9_-]{16,}\b/,
  /\b[A-Fa-f0-9]{64}\b/,
]

function assertAllowedFields(args: Record<string, unknown>, fields: string[]) {
  const allowed = new Set(fields)
  const unknown = Object.keys(args).filter((key) => !allowed.has(key))
  if (unknown.length > 0) throw new Error('Unknown knowledge field')
}

function validateText(value: unknown, field: string, maxLength: number, required = false) {
  if (value === undefined && !required) return ''
  if (typeof value !== 'string') throw new Error(`${field} must be text`)
  const text = value.trim()
  if (required && !text) throw new Error(`${field} is required`)
  if (text.length > maxLength) throw new Error(`${field} is too long`)
  if (forbiddenPatterns.some((pattern) => pattern.test(text))) {
    throw new Error(`${field} contains credential-like data`)
  }
  return text
}

export type KnowledgeDraftInput = {
  title: string
  summary: string
  content: string
  category: string
  tags: string[]
  source: string | null
}

export function validateKnowledgeDraftArgs(args: Record<string, unknown>): KnowledgeDraftInput {
  assertAllowedFields(args, ['title', 'summary', 'content', 'category', 'tags', 'source'])
  const title = validateText(args.title, 'title', 200, true)
  const summary = validateText(args.summary, 'summary', 500)
  const content = validateText(args.content, 'content', 20000, true)
  const category = args.category === undefined ? 'general' : validateText(args.category, 'category', 40, true)
  if (!KNOWLEDGE_CATEGORIES.includes(category as typeof KNOWLEDGE_CATEGORIES[number])) {
    throw new Error('Invalid knowledge category')
  }

  let tags: string[] = []
  if (args.tags !== undefined) {
    if (!Array.isArray(args.tags) || args.tags.length > MAX_TAGS) throw new Error('tags must contain at most 20 items')
    tags = args.tags.map((tag) => validateText(tag, 'tag', MAX_TAG_LENGTH, true))
    if (new Set(tags).size !== tags.length) throw new Error('tags must be unique')
  }

  const source = args.source === undefined ? null : validateText(args.source, 'source', MAX_SOURCE_LENGTH, true)
  return { title, summary, content, category, tags, source }
}

export function validateKnowledgePublishArgs(args: Record<string, unknown>) {
  assertAllowedFields(args, ['id'])
  if (typeof args.id !== 'string' || !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(args.id)) {
    throw new Error('id must be a valid knowledge entry ID')
  }
  return { id: args.id }
}
