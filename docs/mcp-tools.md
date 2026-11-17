> **Snapshot escrito a mano; se regenera con `make mcp-catalog`** (en `backend/`, que ejecuta `go run ./cmd/mcp -catalog`). Si cambias `backend/internal/mcpserver/tools.go`, regenera este archivo. La sección final "Errores y notas" no la genera el comando.

# Catálogo de tools MCP

Servidor: `atento-crm-tools` · Transporte: Streamable HTTP en `/mcp` (o stdio con `-stdio`).

## `get_customer`

Obtiene datos básicos de un cliente del CRM por email o por id. Indica al menos uno.

- **Acceso:** read

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `email` | string | no | Email del cliente |
| `customer_id` | number | no | ID numérico del cliente |

## `get_order_status`

Consulta el estado, el total y la fecha estimada de entrega de un pedido.

- **Acceso:** read

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `order_id` | number | sí | ID numérico del pedido |

## `list_customer_orders`

Lista los pedidos de un cliente, del más reciente al más antiguo.

- **Acceso:** read

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `customer_id` | number | sí | ID numérico del cliente |

## `create_ticket`

Abre un ticket de soporte para un cliente. Úsala solo cuando el cliente lo pida o el problema no se pueda resolver con la base de conocimiento.

- **Acceso:** write

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `customer_id` | number | sí | ID del cliente |
| `subject` | string | sí | Asunto breve del ticket |
| `description` | string | sí | Descripción del problema |
| `priority` | string | no | low, medium, high o urgent (por defecto medium) |

## `escalate_to_human`

Escala un ticket existente a un agente humano indicando el motivo. Úsala ante quejas graves, solicitudes de supervisor o cuando el agente no pueda ayudar.

- **Acceso:** write

| Parámetro | Tipo | Requerido | Descripción |
|---|---|---|---|
| `ticket_id` | number | sí | ID del ticket a escalar |
| `reason` | string | sí | Motivo del escalamiento |

## Errores y notas (no generado)

### Cómo se devuelven los errores

El handler de cada tool no lanza excepciones al cliente MCP: los errores se devuelven como **resultado de la tool con `isError`** y un mensaje seguro (`userMessage` en `server.go`), para que el LLM pueda reaccionar sin ver detalles internos.

| Causa | Mensaje devuelto |
|---|---|
| Recurso inexistente (`domain.ErrNotFound`) | `No se encontró el recurso solicitado.` |
| Entrada inválida (`domain.ErrInvalidInput`) | `Entrada inválida: <detalle>` |
| Cualquier otro error | `Error interno al ejecutar la herramienta.` |
| Resultado no serializable | `No se pudo serializar el resultado.` |

Validaciones relevantes:

| Tool | Condición de error de entrada |
|---|---|
| `get_customer` | No se indica `email` ni `customer_id` |
| `get_order_status`, `list_customer_orders`, `create_ticket`, `escalate_to_human` | Falta el id requerido o no es un entero |
| `create_ticket` | `subject` vacío o > 200 caracteres; `description` vacía o > 4000; `priority` fuera de `low`, `medium`, `high`, `urgent`; cliente inexistente (no encontrado) |
| `escalate_to_human` | `reason` vacío o > 500 caracteres; ticket inexistente (no encontrado) |

Los detalles de validación se emiten en inglés (p. ej. `subject is required (max 200 characters)`), porque vienen de la capa de servicio compartida con la API REST.

### Forma de las respuestas

Las respuestas exitosas son un único bloque de texto con JSON:

- `get_customer` devuelve `{ id, name, email, segment }` (minimización: **no** incluye teléfono).
- `get_order_status` devuelve el pedido completo (incluye `customer_id`, `status`, `total_cents`, `currency`, `items`, `estimated_delivery`, `updated_at`).
- `list_customer_orders` devuelve una lista de pedidos.
- `create_ticket` y `escalate_to_human` devuelven el ticket (`id`, `customer_id`, `subject`, `description`, `priority`, `status`, `escalated`, `escalation_reason`, `created_at`).

### Seguridad

- El endpoint `/mcp` exige la cabecera `X-Internal-Token` (si `INTERNAL_TOKEN` está vacío, **no hay autenticación**).
- **El servidor MCP no comprueba a quién pertenece cada recurso**: acepta cualquier `customer_id`, `order_id` o `ticket_id`. La autorización por cliente la impone el agente (`guard_tools` en `agent/app/tools.py`). Ver H4 en [`04-riesgos-owasp-llm.md`](04-riesgos-owasp-llm.md).
- El agente solo expone al LLM las tools de su lista `ALLOWED_TOOLS`.

### Discrepancia conocida con el agente

El agente espera una tool **`list_customer_tickets`** (figura en `ALLOWED_TOOLS` y la usa `guard_tools` para comprobar que un ticket pertenece al cliente antes de escalar), pero **este servidor MCP no la expone**. Mientras no exista, `escalate_to_human` es denegada por el agente con "No autorizado". Ver H2 en [`04-riesgos-owasp-llm.md`](04-riesgos-owasp-llm.md). La solución es añadir un `ToolSpec` `list_customer_tickets` (parámetro requerido `customer_id`) que use `TicketService.ListByCustomer`.
