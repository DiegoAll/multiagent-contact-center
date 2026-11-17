// Ilustraciones en SVG (la CSP del front no permite imágenes externas).
interface ArtProps {
  className?: string
  ink?: string
  accent?: string
}

const base = { viewBox: '0 0 200 200', fill: 'none', 'aria-hidden': true } as const

export function ChatArt({ className, ink = '#0e1330', accent = '#ffc62b' }: ArtProps) {
  return (
    <svg {...base} className={className}>
      <rect x="20" y="20" width="132" height="92" rx="20" fill={ink} />
      <path d="M44 112v26l28-26z" fill={ink} />
      <rect x="42" y="42" width="84" height="9" rx="4.5" fill="#fff" opacity=".9" />
      <rect x="42" y="60" width="64" height="9" rx="4.5" fill="#fff" opacity=".55" />
      <rect x="42" y="78" width="74" height="9" rx="4.5" fill="#fff" opacity=".35" />
      <rect x="96" y="124" width="86" height="54" rx="18" fill={accent} />
      <path d="M160 178v18l-20-18z" fill={accent} />
      <circle cx="122" cy="151" r="5.5" fill={ink} />
      <circle cx="139" cy="151" r="5.5" fill={ink} />
      <circle cx="156" cy="151" r="5.5" fill={ink} />
    </svg>
  )
}

export function TicketArt({ className, ink = '#0e1330', accent = '#ffc62b' }: ArtProps) {
  return (
    <svg {...base} className={className}>
      <rect x="26" y="36" width="148" height="128" rx="18" fill={ink} />
      <rect x="44" y="54" width="46" height="14" rx="7" fill={accent} />
      <rect x="44" y="80" width="112" height="9" rx="4.5" fill="#fff" opacity=".9" />
      <rect x="44" y="98" width="88" height="9" rx="4.5" fill="#fff" opacity=".5" />
      <path d="M44 124h112" stroke="#fff" strokeOpacity=".3" strokeWidth="3" strokeDasharray="6 7" />
      <rect x="44" y="136" width="58" height="18" rx="9" fill="#ff4b5c" />
      <circle cx="140" cy="145" r="9" fill="#fff" opacity=".85" />
    </svg>
  )
}

export type ArtKey = 'chat' | 'ticket'

export const ART: Record<ArtKey, (p: ArtProps) => React.JSX.Element> = {
  chat: ChatArt,
  ticket: TicketArt,
}