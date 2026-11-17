// Package agentclient implementa service.AgentClient hablando HTTP con el
// servicio del agente (Python / FastAPI).
package agentclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
)

// HTTPClient llama a POST {baseURL}/chat.
type HTTPClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// New crea el cliente. token (opcional) se envía en X-Internal-Token.
func New(baseURL, token string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: timeout},
	}
}

// Chat envía el mensaje al agente y decodifica su respuesta.
func (c *HTTPClient) Chat(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("X-Internal-Token", c.token)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("agent returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var out domain.ChatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode agent response: %w", err)
	}
	return &out, nil
}
