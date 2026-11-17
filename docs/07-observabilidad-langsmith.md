# Observabilidad del agente con LangSmith

LangSmith registra cada ejecución del agente como una **traza** (árbol de pasos): nodos del grafo, llamadas al LLM, recuperación RAG y llamadas a tools. Es la herramienta principal para depurar el comportamiento del agente, medir tokens/costo/latencia y construir datasets de evaluación.

> **Estado.** Probar esto requiere una cuenta de LangSmith y una API key; no se ha ejecutado contra ninguna en la generación del proyecto. Las variables y nombres de abajo salen del código; los detalles del producto (planes, alertas, nombres exactos de ajustes) verifícalos en la documentación vigente de LangSmith.

## Activación

El servicio del agente no contiene código específico de LangSmith: LangChain/LangGraph envían las trazas automáticamente cuando el proceso tiene estas variables de entorno:

| Variable | Valor |
|---|---|
| `LANGSMITH_TRACING` | `true` |
| `LANGSMITH_API_KEY` | tu clave de LangSmith |
| `LANGSMITH_PROJECT` | por ejemplo `atento-agent-lab` |
| `LANGSMITH_ENDPOINT` | solo si usas otra región o instancia propia |

Importante:

- Deben estar en el **entorno del proceso** del contenedor `agent` (sección `environment`/`env_file` de compose). `agent/app/config.py` lee `.env` con `pydantic-settings`, pero eso **no exporta** las variables al entorno del proceso; si pones `LANGSMITH_*` solo en un `.env` que lee pydantic, el SDK no las verá.
- Sin `LANGSMITH_TRACING=true`, no se envía nada (comportamiento por defecto).

## Qué se registra hoy

En `agent/app/main.py`, cada llamada a `POST /chat` ejecuta el grafo con esta configuración:

| Elemento | Valor |
|---|---|
| `run_name` | `customer_support_chat` |
| `tags` | `["atento-lab"]` |
| `metadata.session_id` | id de la conversación (también es el `thread_id` de la memoria) |
| `metadata.customer_hash` | primeros 12 caracteres del SHA-256 del email (o `null`) |

Con ellos puedes filtrar en LangSmith por sesión o por cliente (sin exponer el email en los metadatos).

## Qué se ve en cada traza

Estructura esperada (nombres de nodos tal como están en `graph.py`):

```text
customer_support_chat
├── input_guard                      (sin LLM)
├── identify                         (tool MCP: get_customer)
├── supervisor                       (1 llamada LLM, salvo regla HUMAN)
├── knowledge_agent                  (retriever PGVector + 1 llamada LLM)
│   ├── Retriever  → fragmentos y su fuente
│   └── ChatOpenAI → prompt con <contexto>, respuesta, tokens
├── transactions_agent               (sub-grafo de create_agent)
│   ├── ChatOpenAI → tool_call
│   ├── list_customer_orders (tool MCP)
│   └── ChatOpenAI → respuesta final
├── human_handoff                    (create_ticket + escalate_to_human, sin LLM)
├── quality_agent                    (sin LLM)
└── finalize
```

Solo aparecen los nodos que el ruteo activa. Para cada paso puedes revisar entradas/salidas, latencia, errores, y en las llamadas al LLM: modelo, tokens de entrada/salida y costo estimado.

Preguntas que la traza responde rápido:

- ¿Por qué se fue por `human_handoff` o por `knowledge_agent`? (salida del `supervisor`)
- ¿Qué fragmentos recuperó el RAG y con qué fuente? (¿era el documento correcto?)
- ¿Qué tool llamó, con qué argumentos, y qué respondió el MCP? ¿Hubo `No autorizado`?
- ¿Cuántos pasos dio el bucle de tools y se acercó al límite?
- ¿Dónde se fue el tiempo (supervisor, RAG, LLM, tool)?

## Métricas de producto y de costo

En el panel de monitoreo del proyecto: volumen de trazas, tasa de error, latencia (p50/p95/p99), tokens y costo por traza y agregados. LangSmith calcula el costo a partir del modelo registrado; con un modelo que no reconozca (por ejemplo, un proveedor alternativo) puede requerir configurar sus precios. Para el análisis de qué componentes cuestan más, ver [`08-costos-multiagente.md`](08-costos-multiagente.md).

Mejora sugerida (no implementada): añadir a los `metadata` la ruta elegida (`knowledge`, `transactions`, `both`, `human`) y el proveedor/modelo, para poder agrupar costo y latencia por ruta.

## Feedback, datasets y evaluación

- **Feedback**: el frontend puede enviar una valoración (👍/👎 o CSAT) asociada a la traza; así se encuentran las conversaciones malas.
- **Datasets**: guarda ejemplos buenos y malos (entrada, salida esperada). El archivo `agent/evals/dataset.jsonl` se puede cargar como dataset.
- **Experimentos**: ejecutar el dataset contra una versión del agente y comparar métricas entre versiones (cambios de prompt, modelo o grafo). Ver [`06-evaluacion-y-pruebas.md`](06-evaluacion-y-pruebas.md).
- **Alertas y automatizaciones**: según el plan, se pueden definir alertas (picos de error, latencia o costo) y reglas que envíen trazas a una cola de revisión. Verifica disponibilidad en tu plan.

## Privacidad

Las trazas contienen texto de los clientes y respuestas del agente, y salen a un servicio externo.

Lo que ocurre hoy:

- Los **metadatos** usan un hash del email, no el email.
- **Pero** `customer_email` forma parte del estado de entrada del grafo (`run_input` en `main.py`), por lo que **aparece en claro en las entradas de la traza**. Es el hallazgo H5 de [`04-riesgos-owasp-llm.md`](04-riesgos-owasp-llm.md).
- La redacción de tarjetas y secretos ocurre **dentro** del nodo `input_guard`, es decir, después de que la solicitud original ya se registró como entrada raíz de la traza. Un número de tarjeta o un secreto que escriba el cliente puede quedar en claro en esa entrada raíz aunque el LLM nunca lo reciba. Verifica en una traza real qué texto aparece.
- Los emails ajenos y secretos se enmascaran en la salida final, pero los pasos intermedios (resultados de tools, borradores) pueden contener datos personales.

Opciones, de menor a mayor control:

1. Quitar `customer_email` del estado trazado (pasarlo por la configuración o resolver la identidad antes del grafo).
2. Ocultar entradas y salidas con los ajustes del SDK (variables `LANGSMITH_HIDE_INPUTS` y `LANGSMITH_HIDE_OUTPUTS`, o un cliente con funciones de ocultación) a costa de perder utilidad para depurar.
3. Un *anonymizer* con reglas de redacción (emails, teléfonos, documentos) aplicado a las trazas, reutilizando las expresiones de `guardrails.py`.
4. Alojar LangSmith en tu infraestructura (opciones empresariales) si el tratamiento de datos lo exige.

Define también retención y quién puede ver las trazas. En Colombia, el tratamiento de datos personales de clientes se rige por la Ley 1581 de 2012; consulta con legal antes de enviar datos reales a un SaaS.

## Complementos: logs, métricas y trazas de infraestructura

LangSmith explica el **comportamiento del agente**; no sustituye la observabilidad de infraestructura.

| Qué | Estado | Propuesta |
|---|---|---|
| Logs de la API Go | JSON con `slog`: método, ruta, estado, duración y `request_id` | Enviar a CloudWatch / Cloud Logging |
| Logs del agente | Formato de texto simple (`logging.basicConfig`) | Pasar a JSON y añadir `session_id` |
| Correlación API ↔ agente | **No se propaga** el `request_id` al agente (`agentclient` solo envía el token) | Enviar `X-Request-ID` y registrarlo en ambos lados; hoy se correlaciona por `session_id` |
| Métricas (Prometheus) | No hay endpoint `/metrics` | Añadir en Go y en FastAPI (tasa, errores, latencia, tamaño de colas) |
| Trazas distribuidas (OpenTelemetry) | No instrumentado | OTel en API, MCP y agente para ver el recorrido SPA → API → agente → MCP → base de datos; en AWS, vía ADOT hacia X-Ray |

## Bucle de mejora

```text
traza mala / feedback negativo
        ↓
revisar en LangSmith (¿ruteo?, ¿RAG?, ¿tool?, ¿prompt?)
        ↓
agregar el caso a agent/evals/dataset.jsonl (con la expectativa correcta)
        ↓
corregir (prompt, documento, guardrail, tool, grafo)
        ↓
ejecutar el dataset (--runs 3) y comparar con el experimento anterior
        ↓
desplegar y vigilar la métrica en el panel
```
