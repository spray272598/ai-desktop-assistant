const base = ''

export async function api<T = any>(path: string, opts: RequestInit = {}): Promise<T> {
  const res = await fetch(base + path, {
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  })
  const data = await res.json()
  if (data.code && data.code !== '0000') {
    throw new Error(data.message || 'api error')
  }
  return data.data as T
}

export type SessionInfo = {
  sessionId: string
  agentId: string
  userId: string
  title: string
  status: string
  messageCount: number
  createdAt: string
}

export type ToolInfo = { name: string; description: string }

export type ChatResult = {
  sessionId: string
  response: string
  intent: string
  toolCalls: number
  steps: number
  tokenUsed: number
  taskPlan?: any
  needPermission?: boolean
  pendingPermission?: any
}
