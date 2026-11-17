import type { Message } from '../types'
import { AGENT_LABELS, labelOf } from '../labels'

type Meta = NonNullable<Message['meta']>

/** Recorrido compacto, en una línea, que va bajo cada respuesta. */
export function MiniPath({ meta, active, onClick }: { meta: Meta; active: boolean; onClick: () => void }) {
  const agents = meta.agents_used ?? []
  if (agents.length === 0 && !meta.escalated) return null
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      aria-label="Ver recorrido del agente"
      className={`mt-2 flex w-fit max-w-full flex-wrap items-center gap-x-2 gap-y-1 rounded-md px-2 py-1 text-xs transition-colors ${
        active ? 'bg-brand-soft text-ink' : 'text-muted hover:bg-brand-soft/60'
      }`}
    >
      {agents.map((a, i) => (
        <span key={`${a}-${i}`} className="flex items-center gap-2">
          {i > 0 && <span className="trace-line h-px w-3 bg-brand" aria-hidden />}
          <span>{labelOf(a)}</span>
        </span>
      ))}
      {meta.escalated && (
        <span className="rounded bg-sun-soft px-1.5 py-0.5 font-medium text-sun-ink">Escalado a un asesor</span>
      )}
    </button>
  )
}

/** Detalle completo: línea de tiempo de agentes, fuentes y herramientas. */
export function TraceDetail({ meta }: { meta: Meta | null }) {
  if (!meta) {
    return (
      <p className="text-sm leading-relaxed text-muted">
        Envía un mensaje y verás aquí qué agentes intervinieron, qué documentos consultó y qué herramientas del CRM llamó.
      </p>
    )
  }
  const agents = meta.agents_used ?? []
  const sources = meta.sources ?? []
  const tools = meta.tool_calls ?? []
  return (
    <div className="space-y-6">
      <section aria-labelledby="t-path">
        <h3 id="t-path" className="mb-3 text-sm font-semibold">
          Recorrido
        </h3>
        <ol className="relative space-y-4 pl-6">
          <span className="absolute left-[7px] top-2 bottom-2 w-px bg-line" aria-hidden />
          {agents.map((a, i) => (
            <li key={`${a}-${i}`} className="relative">
              <span className="absolute -left-6 top-1 grid size-[15px] place-items-center rounded-full border border-brand bg-surface">
                <span className="size-[7px] rounded-full bg-brand" />
              </span>
              <p className="text-sm font-medium">{labelOf(a)}</p>
              <p className="text-xs leading-relaxed text-muted">{AGENT_LABELS[a]?.role}</p>
            </li>
          ))}
        </ol>
      </section>

      {sources.length > 0 && (
        <section aria-labelledby="t-src">
          <h3 id="t-src" className="mb-2 text-sm font-semibold">
            Fuentes consultadas
          </h3>
          <ul className="flex flex-wrap gap-1.5">
            {sources.map((s) => (
              <li key={s} className="rounded-md border border-line bg-surface px-2 py-1 font-mono text-xs">
                {s}
              </li>
            ))}
          </ul>
        </section>
      )}

      {tools.length > 0 && (
        <section aria-labelledby="t-tools">
          <h3 id="t-tools" className="mb-2 text-sm font-semibold">
            Herramientas del CRM
          </h3>
          <ul className="space-y-2">
            {tools.map((t, i) => (
              <li key={i} className="rounded-md border border-line bg-surface p-2.5">
                <p className="font-mono text-xs font-medium text-brand">{t.name}</p>
                {t.args && Object.keys(t.args).length > 0 && (
                  <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words font-mono text-xs text-muted">
                    {JSON.stringify(t.args, null, 1)}
                  </pre>
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      {meta.escalated && (
        <p className="rounded-md bg-sun-soft px-3 py-2 text-sm text-sun-ink">
          El caso quedó escalado: un asesor humano continuará la atención.
        </p>
      )}
    </div>
  )
}
