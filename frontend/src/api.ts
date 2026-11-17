import type { ChatResponse, Customer, Ticket } from './types'

export class ApiError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

const FRIENDLY: Record<number, string> = {
  401: 'La clave de la API no es válida. Revisa API_KEY en la configuración.',
  429: 'Demasiadas solicitudes seguidas. Espera un minuto e inténtalo de nuevo.',
  502: 'El servicio no respondió. Comprueba que los contenedores estén en marcha.',
  504: 'El servicio tardó demasiado. Inténtalo de nuevo.',
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, init)
  } catch (e) {
    if (e instanceof DOMException && e.name === 'AbortError') throw e
    throw new ApiError('No hay conexión con el servidor.', 0)
  }
  if (!res.ok) {
    let detail = ''
    try {
      const j = await res.json()
      detail = j?.error ?? ''
    } catch {
      /* cuerpo no JSON */
    }
    throw new ApiError(FRIENDLY[res.status] ?? (detail || `Error ${res.status}`), res.status)
  }
  return (await res.json()) as T
}

export function sendChat(
  body: { message: string; session_id?: string; customer_email?: string },
  signal?: AbortSignal,
): Promise<ChatResponse> {
  return request<ChatResponse>('/api/v1/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal,
  })
}

export async function listCustomers(signal?: AbortSignal): Promise<Customer[]> {
  return (await request<Customer[] | null>('/api/v1/customers?limit=100', { signal })) ?? []
}

export async function listTickets(customerId: number, signal?: AbortSignal): Promise<Ticket[]> {
  return (await request<Ticket[] | null>(`/api/v1/customers/${customerId}/tickets`, { signal })) ?? []
}
