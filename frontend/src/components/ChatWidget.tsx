import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError, sendChat } from '../api'
import type { Message, Persona } from '../types'
import { MiniPath, TraceDetail } from './Trace'

const MAX_CHARS = 2000
let counter = 0
const nextId = () => `m${++counter}`

const CHIPS_KNOWN = ['¿Dónde está mi pedido?', '¿Cuánto tarda el envío a Bogotá?', 'Quiero hablar con un asesor']
const CHIPS_GUEST = ['¿Cuánto tarda el envío a Bogotá?', '¿Cómo devuelvo un producto?', '¿Qué medios de pago aceptan?']

interface Props {
  persona: Persona
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ChatWidget({ persona, open, onOpenChange }: Props) {
  const [messages, setMessages] = useState<Message[]>([])
  const [sessionId, setSessionId] = useState<string | undefined>()
  const [loading, setLoading] = useState(false)
  const [demo, setDemo] = useState(false)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [text, setText] = useState('')
  const abortRef = useRef<AbortController | null>(null)
  const endRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => () => abortRef.current?.abort(), [])
  useEffect(() => {
    if (open) inputRef.current?.focus()
  }, [open])
  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
  }, [messages, loading, open, selectedId])

  const send = useCallback(
    async (raw: string) => {
      const msg = raw.trim()
      if (!msg || loading) return
      const ctrl = new AbortController()
      abortRef.current = ctrl
      setMessages((m) => [...m, { id: nextId(), role: 'user', content: msg }])
      setText('')
      if (inputRef.current) inputRef.current.style.height = '' // vuelve a una línea
      setLoading(true)
      try {
        const res = await sendChat(
          { message: msg, session_id: sessionId, customer_email: persona.email ?? undefined },
          ctrl.signal,
        )
        const id = nextId()
        setSessionId(res.session_id)
        setMessages((m) => [
          ...m,
          {
            id,
            role: 'assistant',
            content: res.reply,
            meta: {
              escalated: res.escalated,
              agents_used: res.agents_used,
              sources: res.sources,
              tool_calls: res.tool_calls,
            },
          },
        ])
      } catch (e) {
        if (e instanceof DOMException && e.name === 'AbortError') return
        const err = e instanceof ApiError ? e.message : 'No se pudo enviar el mensaje.'
        setMessages((m) => [...m, { id: nextId(), role: 'assistant', content: err, error: true }])
      } finally {
        if (abortRef.current === ctrl) setLoading(false)
      }
    },
    [loading, persona.email, sessionId],
  )

  const known = persona.email !== null
  const first = persona.name.split(' ')[0]
  const chips = known ? CHIPS_KNOWN : CHIPS_GUEST

  return (
    <>
      {open && (
        <section
          role="dialog"
          aria-label="Chat con Nova"
          onKeyDown={(e) => e.key === 'Escape' && onOpenChange(false)}
          className="fixed z-50 flex flex-col overflow-hidden border border-line bg-surface shadow-2xl max-sm:inset-0 sm:bottom-24 sm:right-6 sm:h-[min(38rem,calc(100dvh-8rem))] sm:w-[24rem] sm:rounded-2xl"
        >
          <header className="flex items-center gap-3 bg-brand px-4 py-3 text-white">
            <span className="grid size-10 shrink-0 place-items-center rounded-full bg-sun text-lg font-extrabold text-ink" aria-hidden>
              N
            </span>
            <div className="min-w-0 flex-1">
              <h2 className="text-[15px] font-bold leading-tight">Nova</h2>
              <p className="flex items-center gap-1.5 text-xs text-white/85">
                <span className="size-2 rounded-full bg-[#4ade80]" aria-hidden />
                Asistente de NovaTel · opera NovaCX
              </p>
            </div>
            <button
              type="button"
              onClick={() => setDemo((d) => !d)}
              aria-pressed={demo}
              title="Muestra qué agentes y herramientas usó cada respuesta"
              className={`rounded-full border px-2.5 py-1 text-xs font-medium ${
                demo ? 'border-sun bg-sun text-ink' : 'border-white/50 text-white'
              }`}
            >
              Modo demo
            </button>
            <button
              type="button"
              onClick={() => onOpenChange(false)}
              aria-label="Cerrar el chat"
              className="grid size-8 place-items-center rounded-full hover:bg-white/15"
            >
              <svg viewBox="0 0 24 24" className="size-4" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" aria-hidden>
                <path d="m6 6 12 12M18 6 6 18" />
              </svg>
            </button>
          </header>

          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto bg-paper px-4 py-4" aria-live="polite">
            <Bubble>
              {known
                ? `Hola, ${first}. Soy Nova, la asistente virtual de NovaTel. Puedo consultar tus pedidos, resolver dudas de envíos, devoluciones y pagos, o pasarte con un asesor.`
                : 'Hola. Soy Nova, la asistente virtual de NovaTel. Puedo resolver dudas de envíos, devoluciones y pagos. Para consultar pedidos, elige un cliente demo en el menú de la cabecera.'}
            </Bubble>

            {messages.length === 0 && (
              <div className="flex flex-wrap gap-2 pt-1">
                {chips.map((c) => (
                  <button
                    key={c}
                    type="button"
                    onClick={() => send(c)}
                    disabled={loading}
                    className="rounded-full border border-brand/30 bg-surface px-3 py-1.5 text-[13px] font-medium text-brand hover:bg-brand-soft disabled:opacity-50"
                  >
                    {c}
                  </button>
                ))}
              </div>
            )}

            {messages.map((m) =>
              m.role === 'user' ? (
                <div key={m.id} className="flex justify-end">
                  <p className="max-w-[85%] whitespace-pre-wrap break-words rounded-2xl rounded-br-md bg-brand px-3.5 py-2.5 text-[15px] leading-relaxed text-white">
                    {m.content}
                  </p>
                </div>
              ) : (
                <div key={m.id} className="max-w-[92%]">
                  <Bubble error={m.error}>{m.content}</Bubble>
                  {m.meta?.escalated && (
                    <p className="mt-1.5 w-fit rounded-md bg-sun-soft px-2 py-1 text-xs font-medium text-sun-ink">
                      Un asesor continuará tu caso
                    </p>
                  )}
                  {demo && m.meta && (
                    <>
                      <MiniPath meta={m.meta} active={selectedId === m.id} onClick={() => setSelectedId(selectedId === m.id ? null : m.id)} />
                      {selectedId === m.id && (
                        <div className="mt-2 rounded-lg border border-line bg-surface p-3">
                          <TraceDetail meta={m.meta} />
                        </div>
                      )}
                    </>
                  )}
                </div>
              ),
            )}

            {loading && (
              <div className="typing flex w-fit gap-1.5 rounded-2xl rounded-bl-md border border-line bg-surface px-4 py-3.5" role="status" aria-label="Nova está escribiendo">
                <span className="size-1.5 rounded-full bg-muted" />
                <span className="size-1.5 rounded-full bg-muted" />
                <span className="size-1.5 rounded-full bg-muted" />
              </div>
            )}
            <div ref={endRef} />
          </div>

          <form
            className="border-t border-line bg-surface p-3"
            onSubmit={(e) => {
              e.preventDefault()
              send(text)
            }}
          >
            <div className="flex items-end gap-2 rounded-2xl border border-line bg-paper p-1.5 focus-within:border-brand">
              <label htmlFor="chat-msg" className="sr-only">
                Mensaje para Nova
              </label>
              <textarea
                id="chat-msg"
                ref={inputRef}
                value={text}
                rows={1}
                maxLength={MAX_CHARS}
                placeholder="Escribe tu mensaje"
                onChange={(e) => {
                  setText(e.target.value)
                  e.target.style.height = 'auto'
                  e.target.style.height = `${Math.min(e.target.scrollHeight, 120)}px`
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                    e.preventDefault()
                    send(text)
                  }
                }}
                className="h-10 max-h-[120px] min-w-0 flex-1 resize-none bg-transparent px-2.5 py-2 text-[15px] leading-6 outline-none focus-visible:outline-none placeholder:text-muted"
              />
              <button
                type="submit"
                disabled={loading || !text.trim()}
                aria-label="Enviar mensaje"
                className="grid size-10 shrink-0 place-items-center rounded-xl bg-brand text-white disabled:opacity-40"
              >
                <svg viewBox="0 0 24 24" className="size-5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                  <path d="M5 12h14M13 6l6 6-6 6" />
                </svg>
              </button>
            </div>
            <p className="mt-2 px-1 text-[11px] leading-snug text-muted">
              Nova es una IA y puede equivocarse. Nunca compartas tu tarjeta ni contraseñas por este chat.
            </p>
          </form>
        </section>
      )}

      <button
        type="button"
        onClick={() => onOpenChange(!open)}
        aria-expanded={open}
        aria-label={open ? 'Cerrar el chat con Nova' : 'Abrir el chat con Nova'}
        className={`fixed bottom-5 right-5 z-40 flex items-center gap-2 rounded-full bg-brand py-3.5 pl-4 pr-5 text-sm font-bold text-white shadow-xl hover:bg-brand-deep sm:bottom-6 sm:right-6 ${
          open ? 'max-sm:hidden' : ''
        }`}
      >
        <svg viewBox="0 0 24 24" className="size-6" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
          {open ? <path d="m6 9 6 6 6-6" /> : <path d="M21 12a8 8 0 0 1-11.6 7.1L4 20l1.1-4.5A8 8 0 1 1 21 12z" />}
        </svg>
        {open ? 'Cerrar' : 'Chatea con Nova'}
      </button>
    </>
  )
}

function Bubble({ children, error = false }: { children: React.ReactNode; error?: boolean }) {
  return (
    <p
      className={`max-w-[92%] whitespace-pre-wrap break-words rounded-2xl rounded-bl-md border px-3.5 py-2.5 text-[15px] leading-relaxed ${
        error ? 'border-danger/40 bg-surface text-danger' : 'border-line bg-surface'
      }`}
    >
      {children}
    </p>
  )
}
