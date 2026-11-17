# Caso de estudio: copiloto de atención omnicanal para un BPO

> Caso **ficticio** para practicar el diseño de una arquitectura agéntica. La empresa, los volúmenes y las metas son supuestos de trabajo, no datos reales ni de ningún cliente.

## Contexto

*Tienda en línea* (cliente de un BPO / contact center) vende electrónica con envíos nacionales. El BPO atiende por chat sus consultas de posventa. Los asesores humanos consultan el CRM y el sistema de tickets para responder.

## Problema

- Gran parte de las consultas son repetitivas: políticas de envío, devoluciones, garantía, medios de pago y estado del pedido.
- Esas consultas ocupan tiempo de asesores que se necesita para casos complejos o sensibles.
- La calidad de la respuesta varía según el asesor y su conocimiento de las políticas.
- Los clientes esperan en cola incluso para preguntas que tienen respuesta documentada.

## Objetivos

1. Resolver sin intervención humana las consultas de **conocimiento** (políticas) y de **estado** (pedidos, tickets) de forma correcta y trazable.
2. **Escalar bien**: cuando el cliente lo pide, el caso es complejo o el agente no puede resolver, abrir/escalar un ticket y dejar el contexto listo para el asesor.
3. Hacerlo de forma **segura**: sin exponer datos de otros clientes, sin ejecutar acciones no autorizadas y con evidencia para auditoría.
4. Poder **sustituir** CRM, ticketing y canal sin reescribir el agente.

## Alcance

| Dentro | Fuera (por ahora) |
|---|---|
| Chat web, consulta de políticas con citas de fuente | Voz, WhatsApp y correo (canales adicionales) |
| Consulta de pedidos y tickets del cliente identificado | Modificar o cancelar pedidos, reembolsos, cobros |
| Crear tickets y escalar a un asesor | Enrutamiento real a colas del contact center |
| CRM y ticketing **simulados** (PostgreSQL) | Integraciones reales con sistemas del cliente |
| Trazabilidad y evaluación del agente | Panel de analítica para supervisores |

## Flujos

| Flujo | Cuándo se activa | Qué ocurre | Nodos del grafo |
|---|---|---|---|
| **Conocimiento** | "¿Cuánto tarda el envío a Bogotá?" | Recupera fragmentos de la base de conocimiento y responde solo con ese contexto, citando la fuente. | `supervisor` → `knowledge_agent` → `quality_agent` |
| **Transacciones** | "¿Dónde está mi pedido?" | Identifica al cliente, consulta pedidos/tickets con tools autorizadas y resume. Puede abrir un ticket si el cliente lo pide. | `identify` → `supervisor` → `transactions_agent` → `quality_agent` |
| **Ambos** | "¿Puedo devolver mi monitor? ¿Cómo va mi pedido?" | Primero política, luego datos del pedido; una sola respuesta combinada. | `knowledge_agent` → `transactions_agent` |
| **Escalamiento a humano** | "Quiero hablar con un asesor", quejas graves | Sin pasar por un LLM para la decisión de contacto: crea un ticket de prioridad alta y lo escala. Si no hay identidad, pide el email. | `supervisor` (regla `HUMAN`) → `human_handoff` |
| **Bloqueado** | Intento de inyección o mensaje vacío/excesivo | Respuesta neutra; nada llega al LLM. | `input_guard` → `finalize` |

> Hoy la "escalación" solo marca el ticket (`escalated = true`) en el CRM simulado. La entrega real al asesor (cola, notificación, transferencia de la conversación) es parte de la integración con el contact center. Además, hay una brecha conocida que impide que la escalación funcione con el servidor MCP real; está descrita como H2 en [`04-riesgos-owasp-llm.md`](04-riesgos-owasp-llm.md).

## KPIs de negocio y cómo medirlos

No se fijan metas numéricas: dependen del cliente y de una línea base previa al piloto.

| KPI | Definición | Cómo se mediría con este sistema |
|---|---|---|
| **Tasa de contención** | Conversaciones resueltas sin asesor / total | `escalated = false` y sin solicitud posterior de humano en la misma sesión; contrastar con muestreo manual. |
| **AHT (tiempo promedio de manejo)** | Duración de la conversación hasta la resolución | Marcas de tiempo por sesión (`session_id`) y latencia por turno en LangSmith. Para los casos escalados, medir también el AHT del asesor **con** contexto previo frente a la línea base. |
| **CSAT** | Satisfacción del cliente | Pregunta de valoración al cerrar (pendiente en el frontend); se envía como *feedback* a la traza en LangSmith. |
| **Costo por interacción** | Costo del LLM y la infraestructura por conversación | Tokens y costo por traza en LangSmith + costo de infraestructura prorrateado (ver [`08-costos-multiagente.md`](08-costos-multiagente.md)). |
| **Escalamiento correcto** | Casos que debían escalarse y se escalaron, y viceversa | Precisión y exhaustividad contra el dataset dorado y revisión de muestras ([`06-evaluacion-y-pruebas.md`](06-evaluacion-y-pruebas.md)). |
| **Exactitud/fundamentación** | Respuestas correctas y respaldadas por la base de conocimiento | Evaluación con dataset dorado y LLM-juez con revisión humana de una muestra. |
| **Incidentes de seguridad** | Fugas o acciones no autorizadas | Conteo de bloqueos de `input_guard`, denegaciones `tool_denied` y resultados del equipo rojo. |

## Integraciones y cómo el diseño las desacopla

El diseño aísla cada integración detrás de una interfaz, de modo que sustituir el CRM simulado no toque el agente:

| Capacidad | Hoy (simulado) | Reemplazo posible | Qué se cambia |
|---|---|---|---|
| CRM de clientes | Tabla `customers` en PostgreSQL | Salesforce, HubSpot | Nueva implementación de `CustomerRepository` (la interfaz ya existe) |
| Pedidos | Tabla `orders` | API del e-commerce / OMS | Nueva implementación de `OrderRepository` |
| Tickets | Tabla `tickets` | Zendesk, Jira Service Management, Salesforce Service Cloud | Nueva implementación de `TicketRepository` |
| Contact center / canal | No existe | Genesys Cloud, Twilio Flex, Amazon Connect; WhatsApp/voz | Nuevo adaptador de canal que llame a `POST /api/v1/chat`; el traspaso a humano requiere un conector para transferir la conversación con su resumen |
| Base de conocimiento | Archivos `.md` | Confluence, SharePoint, Zendesk Guide | Nuevo cargador en `rag.py` (misma ingesta y retriever) |

Por qué funciona:

1. **Repositorios con interfaz**: `service` solo conoce `repository.CustomerRepository` y compañía; el repositorio concreto (`repository/postgres`) es intercambiable.
2. **MCP como contrato de herramientas**: el agente solo ve tools (`get_customer`, `get_order_status`, `create_ticket`, …). Cambiar el sistema de origen no cambia el contrato.
3. **Autorización en el agente** (`guard_tools`) independiente del origen de datos.
4. **Documentación de contrato**: Swagger para la API REST y [`mcp-tools.md`](mcp-tools.md) para las tools.

Lo que **no** queda resuelto solo por el diseño: autenticación de clientes reales, mapeo de identidades entre sistemas, límites de tasa y errores de los proveedores, y consistencia de datos entre CRMs. Eso exige trabajo propio de cada integración.

## Roadmap por fases

| Fase | Contenido | Criterio de salida |
|---|---|---|
| **0. Laboratorio (actual)** | Chat, RAG, tools MCP, multiagente, guardrails, CRM simulado, docker-compose | Demo local funcionando; pruebas unitarias del agente pasando; dataset dorado ejecutado contra un LLM real |
| **1. Endurecimiento** | Corregir H1–H7 de [`04-riesgos-owasp-llm.md`](04-riesgos-owasp-llm.md): autenticación de clientes, autorización por objeto, secretos obligatorios, sesiones persistentes | Revisión de seguridad y pruebas adversariales en CI |
| **2. Piloto** | Despliegue en GKE/AWS, observabilidad con LangSmith, CSAT, tablero de KPIs, línea base | Métricas frente a la línea base en un segmento pequeño |
| **3. Integración real** | Un CRM y un ticketing reales; handoff real al contact center | Escalamiento con transferencia de contexto verificado por asesores |
| **4. Escala y canales** | Más canales (WhatsApp, voz), multiinquilino, presupuestos por cliente | Operación con SLOs y costos monitoreados |
