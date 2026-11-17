import { useEffect, useState } from 'react'
import { ART, type ArtKey } from './art'

interface Cta {
  label: string
  href?: string
  chat?: boolean
}

interface Slide {
  title: string
  text: string
  art: ArtKey
  bg: string
  fg: string
  disc: string
  inkColor: string
  accent: string
  primary: string
  ctas: [Cta, Cta]
}

const SLIDES: Slide[] = [
  {
    title: 'Agentes de IA y asesores humanos en un solo equipo',
    text: 'Nova resuelve lo frecuente, pasa lo complejo a una persona y deja cada caso con ticket y trazabilidad.',
    art: 'chat',
    bg: 'bg-brand',
    fg: 'text-white',
    disc: 'bg-sun',
    inkColor: '#0e1330',
    accent: '#ffffff',
    primary: 'bg-sun text-ink',
    ctas: [{ label: 'Probar a Nova', chat: true }, { label: 'Ver cómo funciona', href: '#como-funciona' }],
  },
  {
    title: 'Cada contacto queda como un ticket con historia',
    text: 'Prioridad, motivo de escalamiento y estado a la vista del asesor que toma el caso.',
    art: 'ticket',
    bg: 'bg-ink',
    fg: 'text-white',
    disc: 'bg-coral',
    inkColor: '#ffffff',
    accent: '#ffc62b',
    primary: 'bg-white text-ink',
    ctas: [{ label: 'Abrir la consola', href: '#/consola' }, { label: 'Probar a Nova', chat: true }],
  },
]

const reduceMotion = () => typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

export function Hero({ onAsk }: { onAsk: () => void }) {
  const [i, setI] = useState(0)
  const [playing, setPlaying] = useState(() => !reduceMotion())
  const [hover, setHover] = useState(false)

  useEffect(() => {
    if (!playing || hover) return
    const t = setInterval(() => setI((v) => (v + 1) % SLIDES.length), 7000)
    return () => clearInterval(t)
  }, [playing, hover])

  const go = (n: number) => setI((n + SLIDES.length) % SLIDES.length)
  const s = SLIDES[i]
  const Art = ART[s.art]

  return (
    <section
      id="top"
      aria-roledescription="carrusel"
      aria-label="Destacados"
      className="mx-auto max-w-7xl px-4 pt-4"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      <div key={i} className={`slide-in relative overflow-hidden rounded-3xl ${s.bg} ${s.fg}`}>
        <div
          className="grid items-center gap-6 px-6 pb-20 pt-10 sm:px-10 md:min-h-[26rem] md:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)] md:px-14"
          role="group"
          aria-roledescription="diapositiva"
          aria-label={`${i + 1} de ${SLIDES.length}`}
        >
          <div>
            <h1 className="max-w-[20ch] text-4xl font-extrabold leading-[1.03] tracking-tight sm:text-5xl lg:text-6xl">{s.title}</h1>
            <p className="mt-4 max-w-[46ch] text-base leading-relaxed opacity-90 sm:text-lg">{s.text}</p>
            <div className="mt-7 flex flex-wrap gap-3">
              {s.ctas.map((c, n) => {
                const cls =
                  n === 0 ? `rounded-full px-6 py-3 text-sm font-bold ${s.primary}` : 'rounded-full border-2 border-current px-6 py-[10px] text-sm font-bold'
                return c.chat ? (
                  <button key={c.label} type="button" onClick={onAsk} className={cls}>
                    {c.label}
                  </button>
                ) : (
                  <a key={c.label} href={c.href} className={cls}>
                    {c.label}
                  </a>
                )
              })}
            </div>
          </div>

          <div className="relative mx-auto grid aspect-square w-full max-w-[22rem] place-items-center">
            <span className={`absolute inset-4 rounded-full ${s.disc}`} aria-hidden />
            <Art className="relative size-[78%]" ink={s.inkColor} accent={s.accent} />
          </div>
        </div>

        <div className="absolute inset-x-0 bottom-0 flex items-center justify-between px-6 pb-5 sm:px-10 md:px-14">
          <div className="flex items-center gap-2">
            {SLIDES.map((_, n) => (
              <button
                key={n}
                type="button"
                onClick={() => go(n)}
                aria-label={`Ir a la diapositiva ${n + 1}`}
                aria-current={n === i}
                className="grid h-6 place-items-center"
              >
                <span className={`block h-2 rounded-full bg-current transition-all ${n === i ? 'w-8' : 'w-2 opacity-40'}`} />
              </button>
            ))}
            <button
              type="button"
              onClick={() => setPlaying((p) => !p)}
              aria-label={playing ? 'Pausar carrusel' : 'Reanudar carrusel'}
              className="ml-2 grid size-8 place-items-center rounded-full border border-current/40"
            >
              {playing ? (
                <svg viewBox="0 0 24 24" className="size-3.5" fill="currentColor" aria-hidden>
                  <rect x="6" y="5" width="4" height="14" rx="1" />
                  <rect x="14" y="5" width="4" height="14" rx="1" />
                </svg>
              ) : (
                <svg viewBox="0 0 24 24" className="size-3.5" fill="currentColor" aria-hidden>
                  <path d="M7 5v14l12-7z" />
                </svg>
              )}
            </button>
          </div>
          <div className="flex gap-2">
            {[
              { label: 'Anterior', d: 'm15 5-7 7 7 7', n: i - 1 },
              { label: 'Siguiente', d: 'm9 5 7 7-7 7', n: i + 1 },
            ].map((b) => (
              <button
                key={b.label}
                type="button"
                onClick={() => go(b.n)}
                aria-label={b.label}
                className="grid size-10 place-items-center rounded-full border border-current/40 hover:bg-white/10"
              >
                <svg viewBox="0 0 24 24" className="size-4" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                  <path d={b.d} />
                </svg>
              </button>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
