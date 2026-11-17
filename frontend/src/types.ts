// Contrato de POST /api/v1/chat (ver backend/internal/domain/models.go).
export interface ToolCall {
  name: string
  args?: Record<string, unknown>
}

export interface ChatResponse {
  session_id: string
  reply: string
  escalated: boolean
  agents_used?: string[]
  sources?: string[]
  tool_calls?: ToolCall[]
}

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  error?: boolean
  meta?: Omit<ChatResponse, 'reply' | 'session_id'>
}

export interface Persona {
  email: string | null
  name: string
  detail: string
}

export interface Customer {
  id: number
  name: string
  email: string
  segment: string
}

export interface Ticket {
  id: number
  customer_id: number
  subject: string
  description: string
  priority: 'low' | 'medium' | 'high' | 'urgent'
  status: string
  escalated: boolean
  escalation_reason?: string
  created_at: string
}
