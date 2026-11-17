# Análisis de riesgos: OWASP Top 10 for LLM Applications 2025

Este análisis se hizo **leyendo el código** del repositorio. Solo se cuentan como mitigaciones los controles que existen en el código; lo que falta se lista como brecha. No se ejecutó ninguna prueba adversarial contra un LLM real, así que los niveles de riesgo son **estimaciones cualitativas**, no mediciones.

> **Verificación.** Los controles del agente (Python) tienen pruebas automáticas. Los del backend Go (middleware, servicios, MCP) están escritos pero **no se compilaron ni probaron** en la sandbox de generación; revalida con `make test` y `make cover` en tu máquina.

Escala: probabilidad (P) y impacto (I) en Baja / Media / Alta, antes y después de los controles.

## Hallazgos prioritarios (léelos primero)

| # | Hallazgo | Dónde | Gravedad |
|---|---|---|---|
| H1 | **La identidad del cliente es el email que él mismo escribe.** No hay autenticación: cualquiera que conozca (o adivine) `ana.gomez@example.com` puede consultar sus pedidos y crear/escalar tickets en su nombre. | `agent/app/main.py` (`customer_email` del cuerpo), `graph.py` (`identify`) | Alta |
| H2 | **`escalate_to_human` nunca se autoriza con el servidor MCP real.** `guard_tools` verifica la propiedad del ticket llamando a `list_customer_tickets`, pero el servidor MCP Go **no expone esa tool** (solo `get_customer`, `get_order_status`, `list_customer_orders`, `create_ticket`, `escalate_to_human`). El conjunto de tickets propios queda vacío y toda escalación devuelve "No autorizado". Además, `human_handoff` no revisa ese resultado: responde "Listo, escalé tu caso" y marca `escalated: true` aunque la herramienta haya sido denegada. Las pruebas no lo detectan porque su backend falso sí implementa `list_customer_tickets`. | `agent/app/tools.py` (`owned_ticket_ids`), `graph.py` (`human_handoff`), `backend/internal/mcpserver/tools.go` | Alta (funcional y de integridad) |
| H3 | **La API REST no tiene autorización por usuario.** Con la `X-API-Key` (que nginx inyecta para cualquier visitante del frontend) se puede listar todos los clientes (`GET /customers`), leer cualquier pedido o ticket por id y escalar tickets ajenos. | `backend/internal/controller/router.go` | Alta para producción; aceptable en laboratorio |
| H4 | **El servidor MCP no vuelve a autorizar.** Acepta cualquier `customer_id` u `order_id`; confía por completo en `guard_tools` del agente y en el token interno compartido. | `backend/internal/mcpserver/tools.go` | Media |
| H5 | **`customer_email` viaja en el estado del grafo** (`run_input`), por lo que aparece en claro en las entradas de las trazas de LangSmith, aunque los metadatos usen un hash. | `agent/app/main.py` | Media |
| H6 | **Autenticación interna opcional.** Si `INTERNAL_TOKEN` está vacío, el agente y el MCP aceptan llamadas sin credenciales. | `agent/app/main.py` (`verify_token`), `backend/internal/middleware/middleware.go` (`HeaderTokenAuth`) | Media |
| H7 | **`session_id` lo elige el cliente** y es la clave de la memoria (`thread_id`). Quien conozca un `session_id` ajeno puede continuar esa conversación; además, la identidad (`customer_email`) puede cambiar entre turnos de la misma sesión. | `agent/app/main.py` | Media |

## Top 10 LLM 2025: análisis por riesgo

### LLM01: Prompt Injection

- **Cómo aplica.** El usuario puede intentar anular las instrucciones ("ignora las reglas anteriores…") o inducir llamadas a tools. También hay inyección **indirecta**: texto malicioso dentro de documentos recuperados por RAG o de resultados de tools.
- **Riesgo inherente:** P Alta · I Media-Alta.
- **Controles existentes.**
  - `guardrails.py`: `check_input` bloquea patrones de inyección en español e inglés (`_INJECTION_PATTERNS`); si bloquea, `graph.py` reemplaza el mensaje por `[mensaje bloqueado]` para que no contamine el historial.
  - `graph.py` / `prompts.py`: el contexto del RAG viaja en un mensaje de usuario delimitado por `<contexto>`, y los prompts indican tratarlo y a los resultados de tools como datos no confiables.
  - `tools.py` (`guard_tools`): aunque el modelo sea engañado, el `customer_id` se fuerza en código y se verifica la propiedad de pedidos y tickets.
  - `graph.py`: regla determinista para `HUMAN` (no depende del LLM).
- **Brechas.** Las expresiones regulares se evaden con paráfrasis, otros idiomas, ofuscación o *homoglyphs*; no hay clasificador de inyecciones. Los documentos del RAG y las salidas de tools no pasan por `check_input`. El delimitador y el prompt reducen, pero no eliminan, la inyección indirecta.
- **Riesgo residual:** P Media · I Media (el impacto lo acota la autorización en código, salvo H1).

### LLM02: Sensitive Information Disclosure

- **Cómo aplica.** Datos de clientes (nombre, email, pedidos) pasan por el LLM y por servicios externos; pueden filtrarse en respuestas, logs o trazas.
- **Riesgo inherente:** P Media · I Alta.
- **Controles existentes.**
  - `check_input` redacta tarjetas (validadas con Luhn) y secretos (claves tipo `sk-…`, `AKIA…`, `ghp_…`, `Bearer …`) **antes** de que lleguen al LLM.
  - `check_output` enmascara tarjetas, secretos y **emails distintos al del cliente identificado**.
  - `mcpserver/tools.go`: `customerView` minimiza datos (no entrega el teléfono).
  - Go: `RequestLogger` no registra query strings ni cuerpos; `SecurityHeaders` añade `Cache-Control: no-store`.
  - `main.py`: los metadatos de la traza usan un hash del email, no el email.
- **Brechas.** H1 (identidad no verificada), H3, H5. No se redactan teléfonos, documentos de identidad ni direcciones. Los nombres y el detalle de pedidos pueden salir en la respuesta (es el comportamiento deseado para el cliente correcto, pero depende de H1). La memoria de conversación en proceso conserva datos personales sin expiración.
- **Riesgo residual:** P Media · I Alta, **hasta resolver H1**.

### LLM03: Supply Chain

- **Cómo aplica.** Dependencias Python y Go, imágenes base, el modelo/proveedor, el servidor MCP y assets de terceros.
- **Riesgo inherente:** P Media · I Alta.
- **Controles existentes.** El servidor MCP es propio (no se consumen MCPs de terceros). `Makefile` (`make lint`) ejecuta `gosec` y `govulncheck` **si están instalados**. `pyproject.toml` configura `ruff` con reglas `S` (bandit).
- **Brechas.** `requirements.txt` usa rangos (`>=`) y no hay archivo de bloqueo (lockfile); `go.sum` no está versionado (el `Dockerfile` ejecuta `go mod tidy` y `swag@latest` en cada build), así que las compilaciones no son reproducibles. No hay SBOM, escaneo de imágenes ni `pip-audit` en CI. Swagger UI se carga desde `unpkg.com` sin integridad de subrecurso (SRI). Los embeddings y el chat se envían a terceros (riesgo de proveedor).
- **Riesgo residual:** P Media · I Alta.

### LLM04: Data and Model Poisoning

- **Cómo aplica.** La base de conocimiento alimenta las respuestas; contenido manipulado produciría respuestas manipuladas. No hay *fine-tuning*.
- **Riesgo inherente:** P Baja · I Media.
- **Controles existentes.** La única fuente es `agent/knowledge/*.md`, versionado en el repositorio (revisión de cambios por *pull request*). La ingesta usa ids deterministas (`rag.py`), por lo que es idempotente.
- **Brechas.** No hay validación de contenido ni firma de documentos. Si se elimina un archivo `.md`, **sus fragmentos permanecen en el vector store** (la ingesta solo hace *upsert*). Si algún día se permiten cargas de documentos por usuarios, habrá que añadir validación, procedencia y aprobación.
- **Riesgo residual:** P Baja · I Media.

### LLM05: Improper Output Handling

- **Cómo aplica.** La salida del modelo se muestra en el frontend, se guarda en tickets y se usa como argumento de tools.
- **Riesgo inherente:** P Media · I Media.
- **Controles existentes.**
  - Argumentos de tools tipados y validados en Go (`argInt64`, `argString`) y en los services: asunto ≤ 200, descripción ≤ 4000, motivo ≤ 500, prioridad restringida a `low|medium|high|urgent`.
  - Todo el SQL del repositorio usa parámetros (`$1`, `$2`…) en `repository/postgres`.
  - `writeError` no filtra detalles internos; los errores de tools se devuelven como mensajes genéricos (`userMessage`).
  - `check_output` limpia secretos, tarjetas y emails ajenos.
- **Brechas.** `human_handoff` guarda el texto del cliente (hasta 1000 caracteres) como descripción del ticket: cualquier consola de asesores que lo muestre debe **escapar** el contenido. El frontend debe renderizar la respuesta del agente como **texto** (no `dangerouslySetInnerHTML`); verifícalo en el código del frontend. No se valida el esquema de la respuesta del LLM más allá de la etiqueta de ruteo (que tiene respaldo determinista).
- **Riesgo residual:** P Baja · I Media.

### LLM06: Excessive Agency

- **Cómo aplica.** El agente puede crear tickets y escalar casos; un agente con demasiado poder o mal acotado puede actuar sobre datos ajenos o spamear acciones.
- **Riesgo inherente:** P Media · I Alta.
- **Controles existentes.**
  - `tools.py`: lista de permitidas (`ALLOWED_TOOLS`); cualquier otra tool del servidor se descarta (el test del agente incluye una tool `delete_everything` de prueba).
  - `guard_tools`: fuerza `customer_id` en `get_customer`, `list_customer_orders`, `list_customer_tickets` y `create_ticket`; verifica que el pedido pertenezca al cliente; exige que el ticket sea del cliente antes de escalar.
  - Solo dos tools de escritura (`create_ticket`, `escalate_to_human`). Bucle acotado por `max_tool_steps` (límite de recursión `max_tool_steps * 2 + 1`).
  - Separación de responsabilidades: el especialista de conocimiento no tiene tools; solo el de transacciones y el de traspaso las usan.
- **Brechas.** H2 (la autorización de escalación está rota con el MCP real), H4. No hay confirmación humana en código para acciones de escritura (solo una indicación en el prompt de pedir confirmación antes de crear un ticket). No hay tope de tickets por sesión o por cliente.
- **Riesgo residual:** P Media · I Media-Alta.

### LLM07: System Prompt Leakage

- **Cómo aplica.** Intentos de extraer las instrucciones del sistema.
- **Riesgo inherente:** P Alta · I Baja (los prompts no contienen secretos de negocio).
- **Controles existentes.** Los prompts no incluyen credenciales ni datos de terceros (`prompts.py`); `check_input` bloquea consultas del tipo "muestra tu prompt de sistema"; un **token canario** (`CANARY_TOKEN`) va en cada prompt y `check_output` descarta la respuesta si aparece (`system_prompt_leak` → mensaje de respaldo).
- **Brechas.** El valor por defecto del canario está en el repositorio (`config.py`); cambia el valor por despliegue. El canario detecta copia literal, no paráfrasis de las instrucciones. Lo importante es que los controles de seguridad no dependan del prompt (ya es así para la autorización).
- **Riesgo residual:** P Media · I Baja.

### LLM08: Vector and Embedding Weaknesses

- **Cómo aplica.** El vector store (`atento_kb`, PGVector) podría filtrar datos entre inquilinos, ser manipulado o ser consultado sin control de acceso.
- **Riesgo inherente:** P Baja · I Media.
- **Controles existentes.** Una sola colección con contenido público de políticas (no hay datos de clientes en el vector store); ingesta solo desde archivos del repositorio.
- **Brechas.** El agente usa **las mismas credenciales de base de datos** que el CRM simulado (mínimo privilegio no aplicado). No hay filtros por inquilino (no hacen falta hoy, sí si se atiende a varios clientes del BPO). Los textos y consultas se envían al proveedor de embeddings. Con `EMBEDDINGS_PROVIDER=hash` la búsqueda es léxica: sirve para la demo, no es representativa en calidad.
- **Riesgo residual:** P Baja · I Media.

### LLM09: Misinformation

- **Cómo aplica.** El agente puede inventar plazos, políticas o estados de pedido.
- **Riesgo inherente:** P Media · I Media.
- **Controles existentes.** El prompt de conocimiento limita la respuesta al `<contexto>`, prohíbe inventar plazos/precios y ofrece transferir a un asesor si no hay información; la respuesta incluye `sources`; temperatura 0 por defecto; los datos de pedidos vienen de tools, no del modelo.
- **Brechas.** `quality_agent` **no comprueba la fundamentación** (no hay LLM-juez ni verificación contra el contexto). El retriever siempre devuelve `k` fragmentos aunque sean poco relevantes; no hay umbral de similitud. Un LLM puede resumir mal una fecha o un monto de una tool. Falta medir esto: ver el dataset dorado en [`06-evaluacion-y-pruebas.md`](06-evaluacion-y-pruebas.md) (sin ejecutar aún).
- **Riesgo residual:** P Media · I Media.

### LLM10: Unbounded Consumption

- **Cómo aplica.** Abuso de costo (tokens), de CPU/memoria o denegación de servicio.
- **Riesgo inherente:** P Media · I Media-Alta.
- **Controles existentes.**
  - Largo del mensaje ≤ 2000 en Go (`chat_service.go`) y en el agente (`max_message_chars`, `ChatRequest`).
  - Go: cuerpo limitado con `MaxBytesReader`, `chimw.Timeout(90s)`, tiempos de espera del servidor HTTP, rate limit por IP (`RATE_LIMIT_PER_MIN`, 60 por defecto).
  - Agente: `request_timeout_seconds` (90 s), `llm_timeout_seconds` (30 s) con 2 reintentos, `max_tool_steps`, ventana de historial (`HISTORY_WINDOW`), reintentos acotados al conectar con MCP, `ChatRequest.max_length` en campos.
- **Brechas.** No hay presupuesto de tokens ni de costo por sesión/usuario. El rate limit se calcula por `RemoteAddr`: detrás de nginx todos comparten un contador. `InMemorySaver` crece sin límite (riesgo de agotar memoria) y se pierde al reiniciar. No hay límite de concurrencia en el agente. El cliente HTTP de la API espera 60 s (`AGENT_TIMEOUT_SECONDS`) mientras que el agente permite 90 s: la API puede cortar antes de que el agente termine.
- **Riesgo residual:** P Media · I Media.

## Resumen de riesgo

| Riesgo | Inherente (P/I) | Residual (P/I) | Principal brecha |
|---|---|---|---|
| LLM01 Prompt Injection | Alta / Media-Alta | Media / Media | Regex evadibles; inyección indirecta |
| LLM02 Sensitive Information Disclosure | Media / Alta | Media / Alta | H1, H3, H5 |
| LLM03 Supply Chain | Media / Alta | Media / Alta | Sin lockfiles, SBOM ni escaneo |
| LLM04 Data and Model Poisoning | Baja / Media | Baja / Media | Fragmentos huérfanos; sin validación |
| LLM05 Improper Output Handling | Media / Media | Baja / Media | Escapar en consola de asesores y frontend |
| LLM06 Excessive Agency | Media / Alta | Media / Media-Alta | H2, H4, sin confirmación en código |
| LLM07 System Prompt Leakage | Alta / Baja | Media / Baja | Canario público por defecto |
| LLM08 Vector and Embedding Weaknesses | Baja / Media | Baja / Media | Mismas credenciales de base de datos |
| LLM09 Misinformation | Media / Media | Media / Media | Sin verificación de fundamentación |
| LLM10 Unbounded Consumption | Media / Media-Alta | Media / Media | Sin presupuesto de tokens; rate limit por IP |

## OWASP API Security Top 10 (2023) aplicado a la API Go

| ID | Riesgo | Estado en el código |
|---|---|---|
| API1 | Broken Object Level Authorization | **Brecha** (H3): `GET /orders/{id}`, `/tickets/{id}`, `/customers/{id}` no verifican propietario. |
| API2 | Broken Authentication | Clave estática `X-API-Key` (comparación en tiempo constante). Sin usuarios ni JWT. Si `API_KEY` está vacía, no hay autenticación. |
| API3 | Broken Object Property Level Authorization | Parcial: el MCP minimiza campos; la API REST devuelve el objeto de dominio completo (incluye teléfono). |
| API4 | Unrestricted Resource Consumption | Parcial: rate limit por IP, límite de cuerpo, timeouts, `limit` máximo 100 en listados. |
| API5 | Broken Function Level Authorization | **Brecha**: cualquier portador de la clave puede crear y escalar tickets. |
| API6 | Unrestricted Access to Sensitive Business Flows | **Brecha**: sin tope de tickets o chats por cliente. |
| API7 | Server Side Request Forgery | No aplica: no hay URLs controladas por el usuario. |
| API8 | Security Misconfiguration | Parcial: cabeceras de seguridad, sin CORS abierto; credenciales por defecto de desarrollo y tokens vacíos permitidos. |
| API9 | Improper Inventory Management | Swagger UI público en `/swagger/` (sin autenticación) y versionado `/api/v1`. |
| API10 | Unsafe Consumption of APIs | Parcial: respuesta del agente limitada a 1 MiB, JSON decodificado con tipos; el proveedor LLM se trata como no confiable. |

## Modelo de amenazas STRIDE por frontera de confianza

| Frontera | S (suplantación) | T (manipulación) | R (repudio) | I (divulgación) | D (denegación) | E (elevación) |
|---|---|---|---|---|---|---|
| Internet → SPA/nginx | Email como identidad (H1) | Contenido del chat | Sin registro por usuario | Fugas por respuestas | Rate limit compartido tras nginx | Clave inyectada da acceso API completo (H3) |
| nginx → API Go | Clave estática | Validación de entrada | `request_id` en logs | Cabeceras de seguridad | Timeouts y límites | Sin autorización por objeto |
| API Go → Agente | Token interno (opcional, H6) | Mensaje ya validado | Logs de la API | Datos del cliente en tránsito sin TLS en la red interna | Timeout 60 s vs 90 s | El agente confía en `customer_email` |
| Agente → MCP | Token interno | Argumentos forzados por `guard_tools` | Log `tool_denied` en el agente | Minimización en `customerView` | Reintentos acotados | El MCP no revalida (H4) |
| Agente → LLM/embeddings | Claves de API | Respuesta del modelo no confiable | Trazas en LangSmith | Datos del cliente salen al proveedor | Límites del proveedor | Inyección de prompt |
| Servicios → PostgreSQL | Credenciales compartidas | SQL parametrizado | Sin auditoría de cambios en el CRM simulado | Mismo usuario para CRM y vectores (LLM08) | Sin pool dimensionado documentado | Un solo rol con todos los permisos |

## Mejoras pendientes, priorizadas

1. **Corregir H2**: añadir la tool `list_customer_tickets` al servidor MCP (el servicio `TicketService.ListByCustomer` ya existe) y hacer que `human_handoff` falle de forma explícita si `escalate_to_human` devuelve "No autorizado" o error. Añadir una prueba de integración agente ↔ MCP real.
2. **Resolver H1**: autenticar al cliente (JWT/OTP/SSO); derivar `customer_id` del token en la API y pasarlo al agente; dejar de aceptar el email del cuerpo como identidad.
3. **Autorización por objeto en la API (H3)** y, a la vez, en las tools MCP (H4): comprobar que el recurso pertenece al `customer_id` autenticado.
4. **Hacer obligatorios los secretos**: arrancar con error si `INTERNAL_TOKEN`, `API_KEY` o `CANARY_TOKEN` están vacíos o con el valor por defecto fuera de modo desarrollo (H6).
5. **Trazas**: no incluir `customer_email` en el estado/entradas trazadas, o activar `hide_inputs`/`hide_outputs` con redacción (H5).
6. **Sesiones**: generar `session_id` en servidor, vincularlo al cliente autenticado y usar un checkpointer persistente con expiración (H7, LLM10).
7. **Presupuesto de consumo**: tope de tokens/costo por sesión y por cliente, concurrencia máxima, rate limit por usuario con la IP real (`X-Forwarded-For` de confianza).
8. **Cadena de suministro**: versionar `go.sum`, fijar versiones y lockfile de Python, `pip-audit`, `govulncheck` y escaneo de imágenes en CI, SBOM, SRI o *self-hosting* de Swagger UI.
9. **Detección de inyección más robusta**: clasificador además de regex; filtrar también contenido recuperado y resultados de tools; pruebas adversariales en CI.
10. **Fundamentación**: umbral de similitud en el retriever y verificación de que la respuesta está respaldada por el contexto (LLM-juez o reglas).
11. **Confirmación humana en código** para acciones de escritura, y tope de tickets por cliente.
12. **Mínimo privilegio en base de datos**: usuarios distintos para CRM y vector store; limpieza de fragmentos de documentos eliminados.
