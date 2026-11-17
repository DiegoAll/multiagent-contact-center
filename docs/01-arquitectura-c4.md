# Arquitectura (modelo C4)

Este documento describe **Atento Agent Lab**, un agente de atención al cliente construido como práctica técnica: frontend React, API en Go con capas, servidor MCP en Go y un servicio de agente en Python con LangChain y LangGraph.

Convención: los diagramas usan `flowchart` de Mermaid con etiquetas al estilo C4 (`«Persona»`, `«Contenedor: tecnología»`, `«Componente»`) y `subgraph` para los límites. No se usa la sintaxis experimental `C4Context`, para que renderice igual en GitHub y en `mmdc`.

> **Estado de verificación.** El servicio del agente (Python) tiene pruebas automáticas. El backend Go **no se ha podido compilar ni probar en la sandbox donde se generó** (el proxy de módulos de Go estaba bloqueado), así que sus diagramas describen el código escrito, no un comportamiento comprobado en ejecución.

## Decisiones de arquitectura que explican los diagramas

| Decisión | Motivo |
|---|---|
| El agente vive en Python (FastAPI + LangGraph) y el resto en Go | Aprender LangChain/LangGraph en su ecosistema natural, y conservar Go para la API, el modelo de capas y el servidor MCP. |
| El navegador **nunca** habla con el agente | La API Go es la única puerta de entrada: valida entradas, aplica rate limit y oculta el servicio del agente. |
| Las tools se publican por **MCP** (servidor Go) | El agente descubre las tools por protocolo; cualquier otro cliente MCP puede reutilizarlas. |
| Las tools del servidor MCP reutilizan los mismos *services* que la API REST | Una sola implementación de reglas de negocio; el CRM simulado se reemplaza cambiando el repositorio. |
| La autorización sobre las tools se aplica **en código** (`guard_tools`) | El prompt no es un control de seguridad. |

## Nivel 1: Contexto

```mermaid
flowchart TB
  cliente["«Persona»<br/><b>Cliente</b><br/>Consulta políticas, pedidos y tickets"]
  asesor["«Persona»<br/><b>Asesor humano</b><br/>Atiende los casos escalados"]

  subgraph limite["Límite del sistema"]
    lab["«Sistema de software»<br/><b>Atento Agent Lab</b><br/>Agente de atención al cliente:<br/>chat, RAG, tools MCP, multiagente"]
  end

  llm["«Sistema externo»<br/><b>Proveedor LLM</b><br/>OpenAI o DeepSeek<br/>(API compatible con OpenAI)"]
  emb["«Sistema externo»<br/><b>API de embeddings</b><br/>OpenAI (o 'hash' offline para demo)"]
  ls["«Sistema externo»<br/><b>LangSmith</b><br/>Trazas, datasets y evaluación"]
  crm["«Sistema externo (futuro)»<br/><b>CRM / Ticketing</b><br/>Salesforce, HubSpot, Zendesk<br/>Hoy: simulado con PostgreSQL"]
  cc["«Sistema externo (futuro)»<br/><b>Plataforma de contact center</b><br/>Genesys, Twilio, Amazon Connect"]

  cliente -->|"Conversa por chat web"| lab
  lab -->|"Marca tickets como escalados"| asesor
  lab -->|"Completions y function calling"| llm
  lab -->|"Vectoriza consultas y documentos"| emb
  lab -->|"Envía trazas (opcional)"| ls
  lab -.->|"Reemplazo del CRM simulado"| crm
  lab -.->|"Canal de voz/chat y handoff real"| cc
```

<sub>Fuente: [`diagrams/c4-1-contexto.mmd`](diagrams/c4-1-contexto.mmd) · SVG: [`diagrams/c4-1-contexto.svg`](diagrams/c4-1-contexto.svg)</sub>

Notas:

- El CRM y el contact center aparecen punteados porque **hoy son simulados** (PostgreSQL con datos ficticios). El diseño los desacopla (ver [`05-caso-de-estudio.md`](05-caso-de-estudio.md)).
- Hoy, "escalar a un humano" solo marca el ticket como `escalated` en la base de datos. **No hay notificación ni integración real con un asesor ni con un contact center.**

## Nivel 2: Contenedores

```mermaid
flowchart TB
  cliente["«Persona»<br/><b>Cliente</b>"]

  subgraph sistema["Sistema: Atento Agent Lab"]
    spa["«Contenedor: React + Vite + TypeScript + Tailwind»<br/><b>Frontend SPA</b><br/>Portal, chat de Nova y consola de asesores<br/>(servido por nginx)"]
    api["«Contenedor: Go + chi»<br/><b>API REST</b><br/>Capas controller / service / repository<br/>Swagger en /swagger"]
    mcp["«Contenedor: Go + mcp-go»<br/><b>Servidor MCP</b><br/>Tools del CRM simulado<br/>Streamable HTTP en /mcp"]
    agent["«Contenedor: Python + FastAPI + LangChain + LangGraph»<br/><b>Servicio del agente</b><br/>Supervisor + especialistas, RAG y guardrails"]
    db[("«Contenedor: PostgreSQL + pgvector»<br/><b>Base de datos</b><br/>CRM simulado (goose) y vector store")]
  end

  llm["«Externo»<br/><b>Proveedor LLM</b>"]
  emb["«Externo»<br/><b>API de embeddings</b>"]
  ls["«Externo»<br/><b>LangSmith</b>"]

  cliente -->|"HTTPS"| spa
  spa -->|"/api/v1/* (nginx inyecta X-API-Key)"| api
  api -->|"POST /chat + X-Internal-Token"| agent
  agent -->|"MCP Streamable HTTP + X-Internal-Token"| mcp
  api -->|"SQL (pgx)"| db
  mcp -->|"SQL (pgx)"| db
  agent -->|"Búsqueda vectorial (PGVector)"| db
  agent -->|"HTTPS"| llm
  agent -->|"HTTPS"| emb
  agent -.->|"HTTPS (LANGSMITH_TRACING)"| ls
```

<sub>Fuente: [`diagrams/c4-2-contenedores.mmd`](diagrams/c4-2-contenedores.mmd) · SVG: [`diagrams/c4-2-contenedores.svg`](diagrams/c4-2-contenedores.svg)</sub>

| Contenedor | Tecnología | Responsabilidad | Puerto |
|---|---|---|---|
| Frontend SPA | React, Vite, TypeScript, Tailwind, servido por nginx | Portal, chat de Nova (con modo demo de trazas) y consola de asesores. nginx reenvía `/api` a la API e inyecta `X-API-Key`. | 3000 (host) |
| API REST | Go, chi, goose, pgx | Endpoints `/api/v1`, Swagger, validación, rate limit, cliente del agente | 8080 |
| Servidor MCP | Go, mcp-go | Tools del CRM simulado vía Streamable HTTP en `/mcp` (o stdio con `-stdio`) | 8081 |
| Servicio del agente | Python, FastAPI, LangChain, LangGraph | Grafo multiagente, RAG, guardrails, memoria por sesión | 8000 |
| Base de datos | PostgreSQL con pgvector | Tablas del CRM simulado (goose) y vector store de la base de conocimiento | 5432 |

Autenticación entre contenedores:

- SPA → API: cabecera `X-API-Key`, inyectada por nginx (no vive en el JavaScript del navegador).
- API → agente y agente → MCP: cabecera `X-Internal-Token` (secreto compartido, comparación en tiempo constante en Go y `hmac.compare_digest` en Python). **Si `INTERNAL_TOKEN` queda vacío, ambos servicios aceptan llamadas sin autenticación**; compose debe definirlo siempre.

## Nivel 3: Componentes del backend Go

```mermaid
flowchart LR
  spa["«Contenedor»<br/>Frontend SPA"]
  agentx["«Contenedor»<br/>Servicio del agente"]
  dbx[("«Contenedor»<br/>PostgreSQL")]

  subgraph apibin["Binario cmd/api: API REST (Go + chi)"]
    router["«Componente»<br/><b>Router chi + middlewares</b><br/>RequestID, Recoverer, SecurityHeaders,<br/>RequestLogger, Timeout, APIKeyAuth, RateLimit"]
    ctrl["«Componente»<br/><b>Controllers</b><br/>customer, order, ticket, chat<br/>(dependen solo de interfaces de service)"]
    svcs["«Componente»<br/><b>Services (interfaces + implementación)</b><br/>CustomerService, OrderService,<br/>TicketService, ChatService<br/>Validación y reglas de negocio"]
    acl["«Componente»<br/><b>agentclient</b><br/>Implementa AgentClient (HTTP)"]
  end

  subgraph mcpbin["Binario cmd/mcp: servidor MCP (Go + mcp-go)"]
    mcpsrv["«Componente»<br/><b>mcpserver</b><br/>ToolSpec declarativa → tools MCP y catálogo<br/>Token interno X-Internal-Token"]
  end

  subgraph compartido["Compartido por ambos binarios"]
    repoi["«Interfaces»<br/><b>repository</b><br/>CustomerRepository, OrderRepository,<br/>TicketRepository"]
    repoc["«Componente»<br/><b>repository/postgres</b><br/>Implementación concreta (pgx stdlib)"]
    mig["«Componente»<br/><b>migrate + migrations</b><br/>goose: esquema y seed del CRM simulado"]
    dom["«Componente»<br/><b>domain</b><br/>Entidades y errores de dominio"]
  end

  spa -->|"/api/v1/*"| router
  router --> ctrl
  ctrl --> svcs
  svcs -->|"depende de la interfaz"| repoi
  repoc -.->|"implementa"| repoi
  repoc --> dbx
  mig -->|"migra al arrancar la API"| dbx
  svcs --> acl
  acl -->|"POST /chat"| agentx
  agentx -->|"MCP"| mcpsrv
  mcpsrv -->|"reutiliza los mismos services"| svcs
  svcs -.-> dom
```

<sub>Fuente: [`diagrams/c4-3-componentes-backend.mmd`](diagrams/c4-3-componentes-backend.mmd) · SVG: [`diagrams/c4-3-componentes-backend.svg`](diagrams/c4-3-componentes-backend.svg)</sub>

Reglas de dependencia (el sentido de las flechas importa):

1. `controller` depende solo de las **interfaces** de `service`.
2. `service` depende solo de las **interfaces** de `repository` (`CustomerRepository`, `OrderRepository`, `TicketRepository`) y de `AgentClient`.
3. `repository/postgres` es la implementación concreta; es el único paquete que conoce SQL.
4. `cmd/api` y `cmd/mcp` son las raíces de composición: construyen repositorios concretos → services → controllers / tools.
5. `domain` no depende de nada.

Esto permite probar `service` con repositorios falsos y `controller` con services falsos, y cambiar PostgreSQL por otra fuente (por ejemplo un CRM real) sin tocar la lógica de negocio.

## Nivel 3: Componentes del servicio del agente

```mermaid
flowchart TB
  apix["«Contenedor»<br/>API REST (Go)"]
  mcpx["«Contenedor»<br/>Servidor MCP"]
  dbx[("«Contenedor»<br/>PostgreSQL + pgvector")]
  llmx["«Externo»<br/>Proveedor LLM / embeddings"]
  lsx["«Externo»<br/>LangSmith"]

  subgraph agentc["Contenedor: Servicio del agente (Python + FastAPI)"]
    http["«Componente»<br/><b>main.py (FastAPI)</b><br/>POST /chat, /healthz<br/>Verifica X-Internal-Token, timeout global"]
    lg["«Componente»<br/><b>graph.py (LangGraph)</b><br/>Grafo multiagente con estado<br/>y checkpointer por sesión"]
    guard["«Componente»<br/><b>guardrails.py</b><br/>check_input / check_output<br/>(reglas, sin LLM)"]
    tools["«Componente»<br/><b>tools.py</b><br/>ToolProvider (MCP) + guard_tools<br/>(autorización en código)"]
    rag["«Componente»<br/><b>rag.py</b><br/>Ingesta de /knowledge, retriever PGVector"]
    llmf["«Componente»<br/><b>llm.py</b><br/>Fábrica ChatOpenAI (openai | deepseek)<br/>y embeddings"]
    prompts["«Componente»<br/><b>prompts.py</b><br/>Prompts de sistema (sin datos no confiables)"]
  end

  apix -->|"POST /chat"| http
  http --> lg
  lg --> guard
  lg --> prompts
  lg --> tools
  lg --> rag
  lg --> llmf
  tools -->|"MCP Streamable HTTP"| mcpx
  rag -->|"similarity search"| dbx
  rag --> llmf
  llmf --> llmx
  lg -.->|"trazas si LANGSMITH_TRACING=true"| lsx
```

<sub>Fuente: [`diagrams/c4-3-componentes-agente.mmd`](diagrams/c4-3-componentes-agente.mmd) · SVG: [`diagrams/c4-3-componentes-agente.svg`](diagrams/c4-3-componentes-agente.svg)</sub>

### Grafo del agente (LangGraph)

```mermaid
flowchart TD
  START([START]) --> ig["input_guard<br/>check_input (sin LLM)"]
  ig -->|"bloqueado"| fin["finalize"]
  ig -->|"permitido"| idn["identify<br/>get_customer por email (tool MCP)"]
  idn --> sup["supervisor<br/>regla HUMAN o 1 llamada LLM"]
  sup -->|"knowledge"| ka["knowledge_agent<br/>RAG + 1 llamada LLM"]
  sup -->|"both"| ka
  sup -->|"transactions"| ta["transactions_agent<br/>create_agent + tools MCP guardadas"]
  sup -->|"human"| hh["human_handoff<br/>create_ticket + escalate_to_human"]
  ka -->|"route = both"| ta
  ka -->|"route = knowledge"| qa["quality_agent<br/>check_output (sin LLM)"]
  ta --> qa
  hh --> qa
  qa --> fin
  fin --> FIN([END])
```

<sub>Fuente: [`diagrams/c4-3-grafo-agente.mmd`](diagrams/c4-3-grafo-agente.mmd) · SVG: [`diagrams/c4-3-grafo-agente.svg`](diagrams/c4-3-grafo-agente.svg)</sub>

| Nodo | Usa LLM | Qué hace |
|---|---|---|
| `input_guard` | No | Valida largo, detecta patrones de inyección, redacta tarjetas (Luhn) y secretos. Si bloquea, el mensaje del historial se reemplaza por `[mensaje bloqueado]`. |
| `identify` | No (tool MCP) | Resuelve `customer_id` a partir del email con `get_customer`. |
| `supervisor` | Sí (1 llamada), salvo regla `HUMAN` | Clasifica en `KNOWLEDGE`, `TRANSACTIONS`, `BOTH` o `HUMAN`. Una expresión regular fuerza `HUMAN` ante "asesor", "supervisor", etc. Si la etiqueta del LLM no es válida, hay un respaldo determinista. |
| `knowledge_agent` | Sí (1 llamada) | Recupera `k` fragmentos (PGVector) y responde solo con ese contexto, que viaja delimitado en `<contexto>`. |
| `transactions_agent` | Sí (bucle de tools) | `create_agent` con las tools MCP **envueltas** por `guard_tools`. Límite de pasos por `max_tool_steps`. |
| `human_handoff` | No | Llama de forma determinista a `create_ticket` y `escalate_to_human`. |
| `quality_agent` | No | Aplica `check_output`: canary, secretos, tarjetas y emails ajenos. **No verifica si la respuesta está fundamentada**; el nombre sugiere más de lo que hace. |
| `finalize` | No | Agrega la respuesta al historial. |

## Secuencia de una consulta transaccional

```mermaid
sequenceDiagram
  autonumber
  actor C as Cliente
  participant S as SPA (nginx)
  participant A as API Go
  participant G as Agente (FastAPI + LangGraph)
  participant M as Servidor MCP
  participant L as Proveedor LLM
  participant D as PostgreSQL

  C->>S: "¿Dónde está mi pedido?" (con su email)
  S->>A: POST /api/v1/chat (X-API-Key)
  A->>A: Valida mensaje, email y session_id
  A->>G: POST /chat (X-Internal-Token)
  G->>G: input_guard (reglas)
  G->>M: get_customer(email)
  M->>D: SELECT cliente
  D-->>M: cliente
  M-->>G: customer_id
  G->>L: supervisor: clasificar intención
  L-->>G: TRANSACTIONS
  loop Hasta max_tool_steps
    G->>L: transactions_agent (tools permitidas)
    L-->>G: tool_call list_customer_orders
    G->>M: list_customer_orders(customer_id forzado)
    M->>D: SELECT pedidos
    D-->>M: pedidos
    M-->>G: resultado JSON
  end
  G->>L: respuesta final
  L-->>G: texto
  G->>G: quality_agent (check_output)
  G-->>A: reply, agents_used, tool_calls, sources
  A-->>S: ChatResponse
  S-->>C: Respuesta y traza del agente
```

<sub>Fuente: [`diagrams/secuencia-chat.mmd`](diagrams/secuencia-chat.mmd) · SVG: [`diagrams/secuencia-chat.svg`](diagrams/secuencia-chat.svg)</sub>

## Contrato del chat

`POST /api/v1/chat` (API Go) reenvía a `POST /chat` (agente):

```json
// Solicitud
{ "session_id": "opcional", "message": "¿Dónde está mi pedido?", "customer_email": "ana.gomez@example.com" }

// Respuesta
{
  "session_id": "9f1c...",
  "reply": "Tu pedido va en camino...",
  "escalated": false,
  "agents_used": ["guardrails", "supervisor", "transactions", "quality"],
  "sources": [],
  "tool_calls": [{ "name": "list_customer_orders", "args": { "customer_id": 1 } }]
}
```

`agents_used`, `sources` y `tool_calls` alimentan el modo demo del chat (trazas) en el frontend.
