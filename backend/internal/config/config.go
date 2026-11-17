// Package config carga la configuración desde variables de entorno.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config agrupa los parámetros de ejecución de los binarios api y mcp.
type Config struct {
	Port            string
	MCPPort         string
	DatabaseURL     string
	AgentURL        string
	AgentToken      string // secreto compartido backend <-> agente / MCP
	APIKey          string // clave para proteger /api/v1 (vacía = sin auth)
	SwaggerFile     string
	AgentTimeout    time.Duration
	RateLimitPerMin int
}

// Load lee las variables de entorno aplicando valores por defecto para
// desarrollo local.
func Load() Config {
	return Config{
		Port:            getenv("PORT", "8080"),
		MCPPort:         getenv("MCP_PORT", "8081"),
		DatabaseURL:     getenv("DATABASE_URL", "postgres://atento:atento@localhost:5432/atento?sslmode=disable"),
		AgentURL:        getenv("AGENT_URL", "http://localhost:8000"),
		AgentToken:      getenv("INTERNAL_TOKEN", ""),
		APIKey:          getenv("API_KEY", ""),
		SwaggerFile:     getenv("SWAGGER_FILE", "docs/swagger.json"),
		AgentTimeout:    time.Duration(getenvInt("AGENT_TIMEOUT_SECONDS", 60)) * time.Second,
		RateLimitPerMin: getenvInt("RATE_LIMIT_PER_MIN", 60),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
