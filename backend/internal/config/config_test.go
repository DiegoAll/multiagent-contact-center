package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	for _, k := range []string{"PORT", "MCP_PORT", "DATABASE_URL", "AGENT_URL", "INTERNAL_TOKEN",
		"API_KEY", "SWAGGER_FILE", "AGENT_TIMEOUT_SECONDS", "RATE_LIMIT_PER_MIN"} {
		t.Setenv(k, "")
	}
	cfg := Load()
	if cfg.Port != "8080" || cfg.MCPPort != "8081" {
		t.Fatalf("unexpected ports: %+v", cfg)
	}
	if cfg.AgentTimeout != 60*time.Second || cfg.RateLimitPerMin != 60 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.APIKey != "" || cfg.SwaggerFile != "docs/swagger.json" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("API_KEY", "abc")
	t.Setenv("AGENT_TIMEOUT_SECONDS", "5")
	t.Setenv("RATE_LIMIT_PER_MIN", "10")

	cfg := Load()
	if cfg.Port != "9000" || cfg.APIKey != "abc" {
		t.Fatalf("unexpected: %+v", cfg)
	}
	if cfg.AgentTimeout != 5*time.Second || cfg.RateLimitPerMin != 10 {
		t.Fatalf("unexpected: %+v", cfg)
	}
}

func TestLoad_InvalidIntFallsBack(t *testing.T) {
	t.Setenv("RATE_LIMIT_PER_MIN", "not-a-number")
	if got := Load().RateLimitPerMin; got != 60 {
		t.Fatalf("RateLimitPerMin = %d, want 60", got)
	}
}
