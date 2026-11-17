import { useEffect, useRef, useState } from 'react'
import { PERSONAS } from '../personas'
import type { Persona } from '../types'

export function Logo({ light = false }: { light?: boolean }) {
  return (
    <span className="flex items-center gap-2.5">
      <span className="grid size-9 place-items-center rounded-xl bg-brand text-white" aria-hidden>
        <svg viewBox="0 0 32 32" className="size-5" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
          <path d="M9 22V10l14 12V10" />
        </svg>
      </span>
      <span className={`text-xl font-extrabold tracking-tight ${light ? 'text-white' : 'text-ink'}`}>
        Nova<span className={light ? 'text-sun' : 'text-brand'}>CX</span>
      </span>
    </span>
  )
}

interface LinkItem {
  label: string
  href: string
}
interface Column {
  title?: string
  links: LinkItem[]
}
interface Group {
  label: string
  href?: string
  cols?: Column[]
}

// Estructura habitual de un portal de BPO: propuesta de valor, servicios (IA y CX), industrias y acceso al portal.
const NAV: Group[] = [
  { label: '¿Por qué NovaCX?', href: '#como-funciona' },
  {
    label: 'Servicios',
    cols: [
      {
        title: 'Soluciones con IA',
        links: [
          { label: 'Nova AI Agent', href: '#servicios' },
          { label: 'Agent Assist', href: '#servicios' },
          { label: 'Analítica de conversaciones', href: '#servicios' },
        ],
      },
      {
        title: 'Soluciones CX',
        links: [
          { label: 'Atención al cliente', href: '#servicios' },
          { label: 'Cobranza', href: '#servicios' },
          { label: 'Soporte técnico', href: '#servicios' },
          { label: 'Ventas', href: '#servicios' },
        ],
      },
    ],
  },
  {
    label: 'Industrias',
    cols: [
      {
        links: [
          { label: 'Retail y e-commerce', href: '#industrias' },
          { label: 'Telecomunicaciones', href: '#industrias' },
          { label: 'Servicios financieros', href: '#industrias' },
          { label: 'Seguros', href: '#industrias' },
        ],
      },
      {
        links: [
          { label: 'Salud', href: '#industrias' },
          { label: 'Utilities y energía', href: '#industrias' },
          { label: 'Logística y movilidad', href: '#industrias' },
          { label: 'Viajes y hospitalidad', href: '#industrias' },
        ],
      },
    ],
  },
  { label: 'Consola de asesores', href: '#/consola' },
]

interface Props {
  persona: Persona
  onPersona: (p: Persona) => void
  onOpenChat: () => void
}

export function Header({ persona, onPersona, onOpenChat }: Props) {
  const [open, setOpen] = useState<string | null>(null)
  const [mobile, setMobile] = useState(false)
  const [account, setAccount] = useState(false)
  const ref = useRef<HTMLElement>(null)

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(null)
        setAccount(false)
      }
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(null)
        setAccount(false)
        setMobile(false)
      }
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [])

  const close = () => {
    setOpen(null)
    setMobile(false)
  }
  const known = persona.email !== null

  return (
    <header ref={ref} className="sticky top-0 z-30 bg-surface shadow-[0_1px_0_var(--line)]">
      <div className="bg-ink text-[13px] text-white/85">
        <div className="mx-auto flex max-w-7xl items-center justify-end gap-5 px-4 py-1.5">
          <a href="#/consola" className="hover:text-white">
            Portal de asesores
          </a>
          <button type="button" onClick={onOpenChat} className="hover:text-white">
            Contáctanos
          </button>
          <span className="hidden text-white/50 sm:inline" aria-label="Idioma: español">
            ES
          </span>
        </div>
      </div>

      <div className="mx-auto flex max-w-7xl items-center gap-4 px-4 py-3">
        <a href="#top" aria-label="NovaCX, inicio" onClick={close}>
          <Logo />
        </a>

        <nav aria-label="Principal" className="ml-4 hidden lg:block">
          <ul className="flex items-center gap-1">
            {NAV.map((g) => (
              <li key={g.label} className="relative">
                {g.cols ? (
                  <>
                    <button
                      type="button"
                      aria-expanded={open === g.label}
                      aria-haspopup="true"
                      onClick={() => setOpen(open === g.label ? null : g.label)}
                      className={`flex items-center gap-1 rounded-lg px-3 py-2 text-[15px] font-semibold hover:bg-paper ${open === g.label ? 'bg-paper text-brand' : ''}`}
                    >
                      {g.label}
                      <svg viewBox="0 0 24 24" className={`size-4 transition-transform ${open === g.label ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                        <path d="m6 9 6 6 6-6" />
                      </svg>
                    </button>
                    {open === g.label && (
                      <div className="absolute left-0 top-full z-40 mt-2 flex gap-8 rounded-2xl border border-line bg-surface p-5 shadow-xl">
                        {g.cols.map((c, i) => (
                          <div key={i} className="min-w-52">
                            {c.title && <p className="mb-2 px-3 text-sm font-bold text-muted">{c.title}</p>}
                            <ul className="space-y-0.5">
                              {c.links.map((l) => (
                                <li key={l.label}>
                                  <a href={l.href} onClick={close} className="block whitespace-nowrap rounded-lg px-3 py-2 font-medium hover:bg-brand-soft hover:text-brand">
                                    {l.label}
                                  </a>
                                </li>
                              ))}
                            </ul>
                          </div>
                        ))}
                      </div>
                    )}
                  </>
                ) : (
                  <a href={g.href} onClick={close} className="block rounded-lg px-3 py-2 text-[15px] font-semibold hover:bg-paper">
                    {g.label}
                  </a>
                )}
              </li>
            ))}
          </ul>
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <div className="relative">
            <button
              type="button"
              onClick={() => setAccount((v) => !v)}
              aria-haspopup="menu"
              aria-expanded={account}
              className="flex items-center gap-2 rounded-full border border-line px-2.5 py-1.5 text-sm font-medium hover:bg-paper"
            >
              <span
                className={`grid size-6 place-items-center rounded-full text-xs font-bold ${known ? 'bg-brand text-white' : 'bg-brand-soft text-brand'}`}
                aria-hidden
              >
                {known ? persona.name[0] : '?'}
              </span>
              <span className="hidden sm:inline">Cliente demo: {known ? persona.name.split(' ')[0] : 'visitante'}</span>
            </button>
            {account && (
              <div role="menu" className="absolute right-0 top-full mt-2 w-72 rounded-xl border border-line bg-surface p-2 shadow-xl">
                <p className="px-3 pb-1 pt-1.5 text-xs leading-relaxed text-muted">
                  Elige qué cliente de NovaTel escribe a Nova. Al cambiar, la conversación empieza de cero.
                </p>
                {PERSONAS.map((p) => (
                  <button
                    key={p.name}
                    role="menuitemradio"
                    aria-checked={p.name === persona.name}
                    onClick={() => {
                      onPersona(p)
                      setAccount(false)
                    }}
                    className={`flex w-full items-center justify-between rounded-lg px-3 py-2 text-left hover:bg-paper ${p.name === persona.name ? 'bg-brand-soft' : ''}`}
                  >
                    <span>
                      <span className="block text-sm font-medium">{p.name}</span>
                      <span className="block text-xs text-muted">{p.detail}</span>
                    </span>
                    {p.name === persona.name && <span className="text-xs font-medium text-brand">Activo</span>}
                  </button>
                ))}
              </div>
            )}
          </div>

          <button type="button" onClick={onOpenChat} className="hidden rounded-full bg-brand px-5 py-2.5 text-sm font-bold text-white hover:bg-brand-deep sm:block">
            Probar a Nova
          </button>
          <button
            type="button"
            className="grid size-10 place-items-center rounded-lg hover:bg-paper lg:hidden"
            aria-label="Menú"
            aria-expanded={mobile}
            onClick={() => setMobile((v) => !v)}
          >
            <svg viewBox="0 0 24 24" className="size-6" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden>
              {mobile ? <path d="m6 6 12 12M18 6 6 18" /> : <path d="M4 7h16M4 12h16M4 17h16" />}
            </svg>
          </button>
        </div>
      </div>

      {/* Menú móvil */}
      {mobile && (
        <nav aria-label="Principal (móvil)" className="max-h-[70dvh] overflow-y-auto border-t border-line bg-surface px-4 py-3 lg:hidden">
          <ul className="space-y-3">
            {NAV.map((g) => (
              <li key={g.label}>
                {g.cols ? (
                  <>
                    <p className="text-sm font-bold text-muted">{g.label}</p>
                    <ul className="mt-1 grid grid-cols-2 gap-x-4">
                      {g.cols.flatMap((c) => c.links).map((l) => (
                        <li key={l.label}>
                          <a href={l.href} onClick={close} className="block py-1.5 text-[15px]">
                            {l.label}
                          </a>
                        </li>
                      ))}
                    </ul>
                  </>
                ) : (
                  <a href={g.href} onClick={close} className="block py-1 text-[15px] font-semibold">
                    {g.label}
                  </a>
                )}
              </li>
            ))}
          </ul>
        </nav>
      )}
    </header>
  )
}
