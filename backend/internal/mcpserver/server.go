package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer registra todas las tools en un servidor MCP.
func NewServer(svc Services) *server.MCPServer {
	s := server.NewMCPServer("atento-crm-tools", "1.0.0", server.WithToolCapabilities(false))
	for _, spec := range Tools(svc) {
		s.AddTool(buildTool(spec), makeHandler(spec))
	}
	return s
}

func buildTool(spec ToolSpec) mcp.Tool {
	opts := []mcp.ToolOption{mcp.WithDescription(spec.Description)}
	for _, p := range spec.Params {
		po := []mcp.PropertyOption{mcp.Description(p.Description)}
		if p.Required {
			po = append(po, mcp.Required())
		}
		if p.Type == "number" {
			opts = append(opts, mcp.WithNumber(p.Name, po...))
		} else {
			opts = append(opts, mcp.WithString(p.Name, po...))
		}
	}
	return mcp.NewTool(spec.Name, opts...)
}

func makeHandler(spec ToolSpec) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := spec.Handler(ctx, req.GetArguments())
		if err != nil {
			// Los errores de tool se devuelven como resultado (isError) para
			// que el LLM pueda reaccionar; nunca se filtran detalles internos.
			return mcp.NewToolResultError(userMessage(err)), nil
		}
		b, err := json.Marshal(out)
		if err != nil {
			return mcp.NewToolResultError("No se pudo serializar el resultado."), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

// CatalogMarkdown genera la documentación de las tools MCP (equivalente al
// Swagger de la API REST) a partir de las mismas definiciones.
func CatalogMarkdown(specs []ToolSpec) string {
	var b strings.Builder
	b.WriteString("# Catálogo de tools MCP\n\n")
	b.WriteString("Servidor: `atento-crm-tools` · Transporte: Streamable HTTP en `/mcp` (o stdio con `-stdio`).\n\n")
	for _, s := range specs {
		fmt.Fprintf(&b, "## `%s`\n\n%s\n\n", s.Name, s.Description)
		fmt.Fprintf(&b, "- **Acceso:** %s\n\n", s.Access)
		b.WriteString("| Parámetro | Tipo | Requerido | Descripción |\n|---|---|---|---|\n")
		for _, p := range s.Params {
			req := "no"
			if p.Required {
				req = "sí"
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", p.Name, p.Type, req, p.Description)
		}
		b.WriteString("\n")
	}
	return b.String()
}
