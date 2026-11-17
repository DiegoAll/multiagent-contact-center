// Nombres legibles de los nodos del grafo del agente (ver agent/app/graph.py).
export const AGENT_LABELS: Record<string, { name: string; role: string }> = {
  guardrails: { name: 'Guardrails', role: 'Valida el mensaje y bloquea intentos de inyección.' },
  supervisor: { name: 'Supervisor', role: 'Decide qué especialista debe atender la consulta.' },
  knowledge: { name: 'Conocimiento', role: 'Busca en la base de conocimiento y responde con fuentes.' },
  transactions: { name: 'Transacciones', role: 'Consulta pedidos y tickets del cliente en el CRM.' },
  human_handoff: { name: 'Traspaso humano', role: 'Abre un ticket y lo escala a un asesor.' },
  quality: { name: 'Calidad', role: 'Revisa la respuesta antes de enviarla.' },
}

export const labelOf = (id: string) => AGENT_LABELS[id]?.name ?? id
