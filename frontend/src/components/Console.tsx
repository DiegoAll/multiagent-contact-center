import { useCallback, useEffect, useMemo, useState } from 'react'
import { listCustomers, listTickets } from '../api'
import type { Ticket } from '../types'

type Filter = 'all' | 'escalated' | 'open'

const PRIORITY: Record<Ticket['priority'], { label: string; cls: string; rank: number }> = {
  urgent: { label: 'Urgente', cls: 'bg-coral text-white', rank: 0 },
  high: { label: 'Alta', cls: 'bg-sun text-ink', rank: 1 },
  medium: { label: 'Media', cls: 'bg-brand-soft text-brand', rank: 2 },
  low: { label: 'Baja', cls: 'bg-paper text-muted', rank: 3 },
}

const fmt = new Intl.DateTimeFormat('es-CO', { dateStyle: 'medium', timeStyle: 'short' })

interface Row extends Ticket {
  customer: string
}

export function Console() {
  const [rows, setRows] = useState<Row[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [filter, setFilter] = useState<Filter>('all')
  const [selected, setSelected] = useState<number | null>(null)
  const [updated, setUpdated] = useState<Date | null>(null)

  const load = useCallback(async (signal?: AbortSignal) => {
    try {
      const customers = await listCustomers(signal)
      const lists = await Promise.all(customers.map((c) => listTickets(c.id, signal)))
      const all = lists.flatMap((tickets, i) => tickets.map((t) => ({ ...t, customer: customers[i].name })))
      all.sort((a, b) => +new Date(b.created_at) - +new Date(a.created_at))
      setRows(all)
      setError(null)
      setUpdated(new Date())
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') return
      setError(e instanceof Error ? e.message : 'No se pudieron cargar los tickets.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const ctrl = new AbortController()
    void load(ctrl.signal)
    const t = setInterval(() => void load(ctrl.signal), 8000)
    return () => {
      ctrl.abort()
      clearInterval(t)
    }
  }, [load])

  const shown = useMemo(
    () => rows.filter((r) => (filter === 'all' ? true : filter === 'escalated' ? r.escalated : !r.escalated)),
    [rows, filter],
  )
  const detail = rows.find((r) => r.id === selected) ?? null
  const counts = { all: rows.length, escalated: rows.filter((r) => r.escalated).length, open: rows.filter((r) => !r.escalated).length }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold tracking-tight sm:text-4xl">Consola de asesores</h1>
          <p className="mt-2 max-w-[62ch] leading-relaxed text-muted">
            Tickets que Nova abre y escala a una persona. Pídele a Nova hablar con un asesor y el caso aparece aquí en segundos.
          </p>
        </div>
        <div className="flex items-center gap-3 text-sm text-muted">
          {updated && <span>Actualizado {updated.toLocaleTimeString('es-CO', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</span>}
          <button type="button" onClick={() => void load()} className="rounded-full border border-line bg-surface px-4 py-2 font-semibold text-ink hover:border-brand">
            Actualizar
          </button>
        </div>
      </div>

      <p className="mt-4 rounded-xl bg-sun-soft px-4 py-3 text-sm leading-relaxed text-sun-ink">
        Consola de demostración sin inicio de sesión: muestra datos ficticios. En producción exigiría autenticación de asesores y permisos por cola.
      </p>

      <div role="group" aria-label="Filtrar tickets" className="mt-6 flex flex-wrap gap-2">
        {(
          [
            ['all', 'Todos'],
            ['escalated', 'Escalados'],
            ['open', 'Abiertos'],
          ] as const
        ).map(([k, label]) => (
          <button
            key={k}
            type="button"
            aria-pressed={filter === k}
            onClick={() => setFilter(k)}
            className={`rounded-full px-4 py-2 text-sm font-semibold ${filter === k ? 'bg-ink text-white' : 'bg-surface hover:bg-brand-soft'}`}
          >
            {label} <span className="opacity-60">{counts[k]}</span>
          </button>
        ))}
      </div>

      {error && (
        <p role="alert" className="mt-6 rounded-xl border border-danger/40 bg-surface px-4 py-3 text-danger">
          {error}
        </p>
      )}

      <div className="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="overflow-hidden rounded-2xl bg-surface">
          {loading ? (
            <p className="p-6 text-muted">Cargando tickets…</p>
          ) : shown.length === 0 ? (
            <p className="p-6 leading-relaxed text-muted">
              No hay tickets en esta vista. Abre el chat, elige a un cliente demo y escribe «Quiero hablar con un asesor».
            </p>
          ) : (
            <ul>
              {shown.map((t) => {
                const p = PRIORITY[t.priority] ?? PRIORITY.medium
                const on = t.id === selected
                return (
                  <li key={t.id} className="border-b border-line last:border-0">
                    <button
                      type="button"
                      onClick={() => setSelected(on ? null : t.id)}
                      aria-pressed={on}
                      className={`grid w-full grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-4 gap-y-1 px-5 py-4 text-left hover:bg-paper ${on ? 'bg-brand-soft' : ''}`}
                    >
                      <span className="font-mono text-sm font-semibold text-muted">#{t.id}</span>
                      <span className="min-w-0">
                        <span className="block truncate font-semibold">{t.subject}</span>
                        <span className="block truncate text-sm text-muted">
                          {t.customer} · {fmt.format(new Date(t.created_at))}
                        </span>
                      </span>
                      <span className="flex flex-wrap justify-end gap-2">
                        <span className={`rounded-full px-2.5 py-1 text-xs font-bold ${p.cls}`}>{p.label}</span>
                        <span className={`rounded-full px-2.5 py-1 text-xs font-bold ${t.escalated ? 'bg-ink text-white' : 'bg-paper text-muted'}`}>
                          {t.escalated ? 'Escalado' : 'Abierto'}
                        </span>
                      </span>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
        </div>

        <aside aria-label="Detalle del ticket" className="h-fit rounded-2xl bg-surface p-6 lg:sticky lg:top-40">
          {detail ? (
            <div className="space-y-4">
              <div>
                <p className="font-mono text-sm text-muted">Ticket #{detail.id}</p>
                <h2 className="mt-1 text-xl font-bold leading-snug">{detail.subject}</h2>
              </div>
              <dl className="space-y-3 text-sm">
                <div>
                  <dt className="font-semibold text-muted">Cliente</dt>
                  <dd>{detail.customer}</dd>
                </div>
                <div>
                  <dt className="font-semibold text-muted">Descripción</dt>
                  <dd className="whitespace-pre-wrap break-words leading-relaxed">{detail.description || 'Sin descripción'}</dd>
                </div>
                {detail.escalation_reason && (
                  <div>
                    <dt className="font-semibold text-muted">Motivo del escalamiento</dt>
                    <dd className="break-words leading-relaxed">{detail.escalation_reason}</dd>
                  </div>
                )}
                <div>
                  <dt className="font-semibold text-muted">Creado</dt>
                  <dd>{fmt.format(new Date(detail.created_at))}</dd>
                </div>
              </dl>
            </div>
          ) : (
            <p className="text-sm leading-relaxed text-muted">Selecciona un ticket para ver su descripción y el motivo del escalamiento.</p>
          )}
        </aside>
      </div>
    </div>
  )
}
