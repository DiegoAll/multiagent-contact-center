import { Logo } from './Header'

interface Service {
  title: string
  text: string
  demo?: boolean
}

const SERVICES: Service[] = [
  {
    title: 'Nova AI Agent',
    text: 'Agente conversacional con base de conocimiento y herramientas conectadas al CRM del cliente.',
    demo: true,
  },
  {
    title: 'Ticketing y escalamiento',
    text: 'Cada caso se registra con prioridad y motivo, y llega al asesor humano que lo continúa.',
    demo: true,
  },
  {
    title: 'Base de conocimiento con fuentes',
    text: 'Las respuestas salen de las políticas del cliente y citan el documento que las respalda.',
    demo: true,
  },
  { title: 'Agent Assist', text: 'Sugerencias de respuesta y resumen del caso para el asesor mientras atiende.' },
  { title: 'Atención al cliente omnicanal', text: 'Chat, voz y mensajería con una sola vista del historial del cliente.' },
  { title: 'Cobranza y soporte técnico', text: 'Operaciones especializadas con guiones, métricas y control de calidad.' },
]

export function Services() {
  return (
    <section id="servicios" aria-labelledby="serv-title" className="mx-auto mt-16 max-w-7xl scroll-mt-32 px-4">
      <h2 id="serv-title" className="max-w-[24ch] text-3xl font-extrabold tracking-tight sm:text-4xl">
        Qué hacemos por la experiencia de tus clientes
      </h2>
      <p className="mt-3 max-w-[60ch] leading-relaxed text-muted">
        Operamos la atención de marcas de distintos sectores combinando personas, procesos y agentes de IA. Las tarjetas marcadas están implementadas en esta demo.
      </p>
      <ul className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {SERVICES.map((s) => (
          <li key={s.title} className={`rounded-2xl p-6 ${s.demo ? 'bg-surface ring-2 ring-brand' : 'bg-surface'}`}>
            <h3 className="text-lg font-bold">{s.title}</h3>
            <p className="mt-2 text-[15px] leading-relaxed text-muted">{s.text}</p>
            {s.demo && <span className="mt-4 inline-block rounded-full bg-brand-soft px-3 py-1 text-xs font-bold text-brand">En esta demo</span>}
          </li>
        ))}
      </ul>
    </section>
  )
}

const STEPS = [
  { title: 'El cliente escribe', text: 'Los guardrails revisan el mensaje y bloquean intentos de inyección antes de llegar al modelo.' },
  { title: 'El supervisor elige al especialista', text: 'Conocimiento para políticas, Transacciones para pedidos y tickets, Traspaso humano para casos que lo piden.' },
  { title: 'El especialista responde', text: 'Con fuentes de la base de conocimiento o con datos del CRM obtenidos por herramientas MCP.' },
  { title: 'Un asesor toma lo escalado', text: 'El caso aparece como ticket en la consola, con prioridad y motivo.' },
]

export function HowItWorks() {
  return (
    <section id="como-funciona" aria-labelledby="how-title" className="mx-auto mt-16 max-w-7xl scroll-mt-32 px-4">
      <div className="rounded-3xl bg-surface p-8 sm:p-12">
        <h2 id="how-title" className="max-w-[26ch] text-3xl font-extrabold tracking-tight sm:text-4xl">
          Cómo trabaja Nova con tu equipo
        </h2>
        <ol className="mt-8 grid gap-6 md:grid-cols-2 lg:grid-cols-4">
          {STEPS.map((s, n) => (
            <li key={s.title} className="relative border-t-4 border-brand pt-4">
              <span className="text-sm font-bold text-brand">Paso {n + 1}</span>
              <h3 className="mt-1 text-lg font-bold leading-snug">{s.title}</h3>
              <p className="mt-2 text-[15px] leading-relaxed text-muted">{s.text}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  )
}

const INDUSTRIES = [
  'Retail y e-commerce',
  'Telecomunicaciones',
  'Servicios financieros',
  'Seguros',
  'Salud',
  'Utilities y energía',
  'Logística y movilidad',
  'Viajes y hospitalidad',
]

export function Industries() {
  return (
    <section id="industrias" aria-labelledby="ind-title" className="mx-auto mt-16 max-w-7xl scroll-mt-32 px-4">
      <h2 id="ind-title" className="text-3xl font-extrabold tracking-tight sm:text-4xl">
        Industrias
      </h2>
      <ul className="mt-6 flex flex-wrap gap-3">
        {INDUSTRIES.map((x, n) => (
          <li
            key={x}
            className={`rounded-full px-5 py-2.5 text-[15px] font-semibold ${n === 0 ? 'bg-brand text-white' : 'bg-surface'}`}
          >
            {x}
          </li>
        ))}
      </ul>
      <p className="mt-4 max-w-[64ch] text-sm leading-relaxed text-muted">
        La demo simula la atención de NovaTel, un cliente ficticio de retail y e-commerce de tecnología. Los clientes, pedidos y tickets son datos de prueba.
      </p>
    </section>
  )
}

export function DemoBand({ onAsk }: { onAsk: () => void }) {
  return (
    <section aria-labelledby="demo-title" className="mx-auto mt-16 max-w-7xl px-4">
      <div className="flex flex-col items-start justify-between gap-6 rounded-3xl bg-brand-deep p-8 text-white sm:p-12 md:flex-row md:items-center">
        <div>
          <h2 id="demo-title" className="max-w-[26ch] text-3xl font-extrabold leading-tight tracking-tight sm:text-4xl">
            Escríbele a Nova como cliente de NovaTel
          </h2>
          <p className="mt-3 max-w-[58ch] leading-relaxed opacity-85">
            Elige un cliente en el menú de la cabecera, pregunta por un pedido o pide hablar con un asesor, y mira cómo el ticket llega a la consola.
          </p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-3">
          <button type="button" onClick={onAsk} className="rounded-full bg-sun px-7 py-3.5 text-sm font-bold text-ink">
            Hablar con Nova
          </button>
          <a href="#/consola" className="rounded-full border-2 border-white px-7 py-3 text-sm font-bold">
            Abrir la consola
          </a>
        </div>
      </div>
    </section>
  )
}

export function Footer() {
  return (
    <footer className="mt-16 bg-ink text-white">
      <div className="mx-auto grid max-w-7xl gap-10 px-4 py-12 md:grid-cols-[1.4fr_1fr_1fr]">
        <div>
          <Logo light />
          <p className="mt-4 max-w-[40ch] text-sm leading-relaxed text-white/70">
            NovaCX es una empresa ficticia creada para un proyecto de práctica: un agente de IA multiagente con ticketing para un contact center. Clientes, pedidos y tickets son datos de prueba.
          </p>
        </div>
        <nav aria-label="Secciones">
          <h2 className="text-sm font-bold">Secciones</h2>
          <ul className="mt-3 space-y-2 text-sm text-white/70">
            <li>
              <a href="#servicios" className="hover:text-white">Servicios</a>
            </li>
            <li>
              <a href="#como-funciona" className="hover:text-white">Cómo trabaja Nova</a>
            </li>
            <li>
              <a href="#industrias" className="hover:text-white">Industrias</a>
            </li>
            <li>
              <a href="#/consola" className="hover:text-white">Consola de asesores</a>
            </li>
          </ul>
        </nav>
        <div>
          <h2 className="text-sm font-bold">Horarios de NovaTel (cliente demo)</h2>
          <p className="mt-3 text-sm leading-relaxed text-white/70">
            Asesores: lunes a viernes de 8:00 a. m. a 6:00 p. m. y sábados de 9:00 a. m. a 1:00 p. m. Nova atiende las 24 horas.
          </p>
        </div>
      </div>
    </footer>
  )
}
