import type { Persona } from './types'

// Cuentas de demostración: coinciden con los clientes semilla del CRM simulado.
export const PERSONAS: Persona[] = [
  { email: null, name: 'Visitante', detail: 'Sin iniciar sesión' },
  { email: 'ana.gomez@example.com', name: 'Ana Gómez', detail: 'Cliente premium' },
  { email: 'carlos.ruiz@example.com', name: 'Carlos Ruiz', detail: 'Cliente estándar' },
  { email: 'laura.mejia@example.com', name: 'Laura Mejía', detail: 'Cliente estándar' },
]
