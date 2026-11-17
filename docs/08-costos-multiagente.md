# ¿En qué incrementa el costo la arquitectura multiagente?

> **Sin cifras de precios.** Los precios de los proveedores cambian; verifica las tarifas vigentes de tu cuenta. Tampoco se han medido costos reales de este sistema (no se ejecutó contra un LLM real). Lo que sigue es el análisis de **cuántas llamadas y qué tokens** genera el diseño; para medir el costo real, usa LangSmith (sección final).

## Qué añade el multiagente

| Fuente de costo | Explicación |
|---|---|
| **Más llamadas al LLM por turno** | Un nodo `supervisor` clasifica la intención (1 llamada) y luego trabaja un especialista (conocimiento: 1 llamada; transacciones: un bucle con tools). Un agente único habría decidido todo en el mismo bucle. |
| **Más tokens de entrada** | Cada agente lleva su propio prompt de sistema y su ventana de historial; el contexto no se comparte "gratis" entre nodos. |
| **Latencia aditiva** | Las llamadas del supervisor y del especialista son secuenciales: el tiempo de respuesta suma ambas. |
| **Bucle de tools** | En el agente de transacciones cada vuelta (modelo → tool → modelo) vuelve a enviar el historial y los resultados acumulados de tools. |
| **Embeddings** | Una consulta al RAG vectoriza la pregunta (1 llamada de embeddings por turno de conocimiento). Además, la ingesta al arrancar vuelve a calcular los embeddings de todos los fragmentos en cada reinicio (se hace *upsert*, pero se re-vectoriza). |
| **Observabilidad** | LangSmith y el almacenamiento de trazas tienen su propio costo según el plan. |

El multiagente también tiene beneficios no económicos (separación de permisos, prompts más cortos y específicos, pruebas por especialista), pero el costo extra es real y conviene medirlo antes de decidir si compensa.

## Optimizaciones que el código ya incluye

| Optimización | Dónde | Efecto |
|---|---|---|
| Regla determinista para `HUMAN` (expresión regular sobre "asesor", "supervisor", etc.) | `graph.py` (`_HUMAN_RE`, `supervisor`) | Esos turnos no gastan la llamada del supervisor |
| `human_handoff` sin LLM | `graph.py` | Crea el ticket y escala con llamadas directas a tools |
| `quality_agent` sin LLM | `graph.py` + `guardrails.py` | Revisión de salida con reglas, costo cero en tokens (a cambio de no verificar fundamentación) |
| Guardrails de entrada sin LLM | `guardrails.py` | Los intentos bloqueados no llegan a ningún modelo |
| Ventana de historial | `HISTORY_WINDOW = 6` (4 para el supervisor) | Acota los tokens por turno; el historial de tools no se arrastra entre turnos |
| Respuesta "necesito tu email" sin LLM | `graph.py` (`NEEDS_IDENTITY_REPLY`) | Sin identidad no se corre el agente de transacciones |
| Tope de pasos de tools | `max_tool_steps` (6 por defecto → límite de recursión 13) | Evita bucles caros |
| Contexto RAG acotado | `rag_top_k = 4`, fragmentos de ~700 caracteres | Limita los tokens del prompt de conocimiento |
| Temperatura 0 y modelo configurable | `config.py` | Resultados más estables; se puede elegir un modelo pequeño |
| Largo máximo del mensaje | 2000 caracteres | Acota la entrada |

## Fórmula de costo por conversación

Por turno *t*:

```text
C_turno = Σ_j ( Tin_j · p_in + Tout_j · p_out )  +  Nemb · Temb · p_emb

Tin_j  = Tsys_j + Thist_j + Tctx_j + Ttools_j
```

| Variable | Significado |
|---|---|
| `j` | Cada llamada al LLM del turno (supervisor, conocimiento, cada vuelta del bucle de tools) |
| `Tin_j`, `Tout_j` | Tokens de entrada y salida de la llamada `j` |
| `p_in`, `p_out` | Precio por token de entrada y de salida del modelo (**verifica el vigente**) |
| `Tsys_j` | Prompt de sistema del nodo |
| `Thist_j` | Historial dentro de la ventana (≤ `HISTORY_WINDOW` mensajes) |
| `Tctx_j` | Contexto RAG (≈ `rag_top_k` × tamaño del fragmento) en el nodo de conocimiento |
| `Ttools_j` | Resultados de tools acumulados en el bucle (crecen en cada vuelta) |
| `Nemb`, `Temb`, `p_emb` | Número de llamadas de embeddings, tokens por llamada y precio de embeddings |

Por conversación: `C_conv = Σ_t C_turno(t)`. Por **interacción resuelta** (el KPI de negocio): `C_conv / conversaciones resueltas`, sumando el costo de infraestructura prorrateado y el tiempo de asesor de los casos que se escalan.

Como el historial está acotado por ventana, el costo por turno no crece indefinidamente con la longitud de la conversación; sí crece dentro del bucle de tools.

## Llamadas al LLM por ruta del grafo

Cálculo a partir de `graph.py` con la configuración por defecto. Las rutas con tools cuentan solo llamadas al **modelo de chat**; las llamadas a tools MCP y a la base de datos no consumen tokens.

| Ruta | Mínimo | Típico | Máximo | Embeddings |
|---|---|---|---|---|
| Entrada bloqueada por guardrail | 0 | 0 | 0 | 0 |
| Escalamiento a humano (regla `HUMAN`) | 0 | 0 | 0 | 0 |
| Escalamiento a humano (clasificado por el LLM) | 1 | 1 | 1 | 0 |
| Conocimiento | 2 | 2 | 2 | 1 |
| Transacciones, cliente identificado | 2 | 3 | 8 | 0 |
| Transacciones, sin email | 1 | 1 | 1 | 0 |
| Ambos, cliente identificado | 3 | 4 | 9 | 1 |
| Ambos, sin email | 2 | 2 | 2 | 1 |

Cómo salen los números:

- `supervisor` = 1 (salvo regla `HUMAN`); `knowledge_agent` = 1; `human_handoff`, `quality_agent`, `input_guard` e `identify` = 0.
- `transactions_agent` (cliente identificado): mínimo 1 (responde sin tools), típico 2 (una tool y respuesta final), máximo ≈ 7 antes de que el límite de recursión (`max_tool_steps × 2 + 1 = 13` pasos) corte con un mensaje de transferencia. El máximo exacto depende de cómo cuenta los pasos la versión de LangGraph; trátalo como aproximado.
- Los totales suman estas piezas (por ejemplo, "Transacciones típico" = 1 supervisor + 2 del especialista = 3).

### Comparación con un agente único (hipotética)

Un solo agente con todas las tools (incluida una de búsqueda en la base de conocimiento) no tendría supervisor. No existe una implementación de agente único en este repositorio; la tabla es una estimación de diseño.

| Ruta | Multiagente (típico) | Agente único (típico, estimado) | Diferencia |
|---|---|---|---|
| Conocimiento | 2 | 2 (llamada a la tool de búsqueda + respuesta) | ≈ igual en llamadas; el multiagente **siempre** recupera, aunque la pregunta no lo necesite |
| Transacciones | 3 | 2–3 | +0 a +1 (el supervisor) |
| Ambos | 4 | 3–4 | +0 a +1 |
| Escalamiento por regla | 0 | 1–2 (el LLM decidiría y llamaría tools) | El multiagente ahorra llamadas |

Conclusión provisional: el sobrecosto en llamadas del multiagente está sobre todo en **una llamada corta extra del supervisor**, y se compensa parcialmente con rutas sin LLM. Los tokens extra por prompts duplicados y la latencia aditiva son el costo adicional que hay que medir en LangSmith.

Posibles mejoras de costo (no implementadas): usar un modelo más pequeño para el supervisor y uno mayor para los especialistas (hoy un único `llm` sirve a todos los nodos), cachear respuestas de conocimiento frecuentes, y reducir la ingesta repetida al arrancar (omitir fragmentos ya indexados).

## OpenAI frente a DeepSeek

Ambos se usan a través de `ChatOpenAI`; el proyecto ya soporta los dos (`agent/app/config.py`, `agent/app/llm.py`):

| Variable | OpenAI | DeepSeek |
|---|---|---|
| `LLM_PROVIDER` | `openai` | `deepseek` |
| Clave | `OPENAI_API_KEY` | `DEEPSEEK_API_KEY` |
| Modelo | `OPENAI_MODEL` (por defecto `gpt-4o-mini`; ajústalo al modelo vigente en tu cuenta) | `DEEPSEEK_MODEL` (por defecto `deepseek-chat`) |
| URL base | por defecto del SDK | `DEEPSEEK_BASE_URL` (`https://api.deepseek.com`) |

Solo cambian la URL base, la clave y el modelo, porque DeepSeek expone una API compatible con OpenAI.

Consideraciones:

1. **Embeddings.** Según la configuración del proyecto, DeepSeek no ofrece embeddings: aunque uses DeepSeek para el chat, los embeddings siguen siendo de OpenAI (`EMBEDDINGS_PROVIDER=openai`) o `hash` (offline, solo demo, no semántico). Por tanto, usar DeepSeek **no elimina** la dependencia de una clave de OpenAI para un RAG de calidad. Verifica en la documentación vigente de DeepSeek si esto cambió.
2. **Function calling.** El agente de transacciones depende de la llamada a funciones. La calidad en esta tarea varía entre modelos: **valídala con el dataset dorado** (exactitud de selección de tool y de parámetros) antes de adoptar un modelo. Algunos modelos de razonamiento no admiten bien las tools o requieren no enviar `temperature` (por eso existe `llm_temperature: None`); verifica el modelo concreto.
3. **Privacidad y residencia de datos.** Los mensajes de los clientes salen al proveedor. Revisa dónde se procesan y almacenan, las condiciones de uso de los datos y los acuerdos de tratamiento. Para datos reales de clientes de un BPO esto puede ser decisivo y suele requerir análisis legal (en Colombia, Ley 1581 de 2012). Para esta práctica con datos ficticios no es un problema.
4. **Disponibilidad y límites.** Compara límites de tasa, *timeouts* y estabilidad; la configuración usa `llm_timeout_seconds = 30` y 2 reintentos.
5. **Costo.** Compara precios de entrada y salida vigentes y el costo total por interacción **medido**, no solo el precio por token: un modelo barato que necesita más vueltas de tools o falla más cuesta más por interacción resuelta.

Recomendación de práctica: desarrollar con el modelo más económico que pase el dataset dorado, y repetir la comparación (exactitud, latencia, costo por interacción resuelta) antes de decidir.

## Medir el costo real en LangSmith

1. Activar trazas (ver [`07-observabilidad-langsmith.md`](07-observabilidad-langsmith.md)).
2. En el proyecto, revisar **tokens y costo por traza** y los agregados; abrir una traza y ver el costo por llamada al LLM para identificar qué nodo cuesta más (supervisor, conocimiento, bucle de tools).
3. Filtrar por `metadata.session_id` para obtener el costo de una conversación completa; sumar por tag/ruta si añades la ruta como metadato.
4. Ejecutar el dataset dorado como experimento con cada modelo y comparar costo medio por caso, latencia y exactitud en el mismo informe.
5. Si el costo no aparece para un modelo no reconocido, configurar sus precios en LangSmith o calcular con la fórmula usando los tokens registrados.
6. Calcular el costo por interacción **resuelta** (incluyendo los casos escalados y la infraestructura) para comparar con el costo de la atención humana en su línea base.
