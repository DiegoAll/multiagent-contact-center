import { useEffect, useState } from 'react'
import { ChatWidget } from './components/ChatWidget'
import { Console } from './components/Console'
import { Header } from './components/Header'
import { Hero } from './components/Hero'
import { DemoBand, Footer, HowItWorks, Industries, Services } from './components/Sections'
import { PERSONAS } from './personas'
import type { Persona } from './types'

const isConsole = (hash: string) => hash.startsWith('#/consola')

export default function App() {
  const [persona, setPersona] = useState<Persona>(PERSONAS[1])
  const [chatOpen, setChatOpen] = useState(false)
  const [hash, setHash] = useState(() => window.location.hash)

  useEffect(() => {
    const onHash = () => setHash(window.location.hash)
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  // Tras cambiar de vista, lleva al ancla pedida (o al inicio).
  useEffect(() => {
    const id = isConsole(hash) ? '' : hash.slice(1)
    const t = setTimeout(() => {
      const el = id ? document.getElementById(id) : null
      if (el) el.scrollIntoView()
      else window.scrollTo(0, 0)
    }, 0)
    return () => clearTimeout(t)
  }, [hash])

  return (
    <div className="min-h-dvh">
      <a
        href="#servicios"
        className="sr-only focus:not-sr-only focus:fixed focus:left-3 focus:top-3 focus:z-50 focus:rounded-lg focus:bg-ink focus:px-4 focus:py-2 focus:text-white"
      >
        Saltar al contenido
      </a>
      <Header persona={persona} onPersona={setPersona} onOpenChat={() => setChatOpen(true)} />
      <main>
        {isConsole(hash) ? (
          <Console />
        ) : (
          <>
            <Hero onAsk={() => setChatOpen(true)} />
            <Services />
            <HowItWorks />
            <Industries />
            <DemoBand onAsk={() => setChatOpen(true)} />
          </>
        )}
      </main>
      <Footer />
      {/* La sesión del agente pertenece a una identidad: al cambiar de cliente, el chat empieza de cero. */}
      <ChatWidget key={persona.name} persona={persona} open={chatOpen} onOpenChange={setChatOpen} />
    </div>
  )
}
