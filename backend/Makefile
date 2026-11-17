.PHONY: tidy test cover swagger run-api run-mcp mcp-catalog lint

# Paquetes con lógica unit-testeable. El paquete postgres se cubre con tests
# de integración (testcontainers) en una siguiente iteración.
COVER_PKGS = ./internal/service/... ./internal/controller/... ./internal/middleware/... \
             ./internal/agentclient/... ./internal/mcpserver/... ./internal/config/...
COVER_MIN  = 80

tidy:
	go mod tidy

test:
	go test ./... -race -count=1

cover:
	go test -race -count=1 -coverprofile=cover.out $(COVER_PKGS)
	go tool cover -func=cover.out | tail -n 1
	@go tool cover -func=cover.out | awk -v min=$(COVER_MIN) '/^total:/ { gsub("%","",$$3); if ($$3+0 < min) { print "Cobertura " $$3 "% < " min "%"; exit 1 } else { print "Cobertura OK: " $$3 "%" } }'
	@echo "Informe HTML: go tool cover -html=cover.out"

# Requiere: go install github.com/swaggo/swag/cmd/swag@latest
swagger:
	swag init -g cmd/api/main.go --parseInternal -o docs --outputTypes json

run-api:
	go run ./cmd/api

run-mcp:
	go run ./cmd/mcp

# Genera el catálogo de tools MCP (equivalente al Swagger de la API REST).
mcp-catalog:
	go run ./cmd/mcp -catalog > ../docs/mcp-tools.md

lint:
	gofmt -l -e .
	go vet ./...
	@command -v gosec >/dev/null && gosec ./... || echo "gosec no instalado (go install github.com/securego/gosec/v2/cmd/gosec@latest)"
	@command -v govulncheck >/dev/null && govulncheck ./... || echo "govulncheck no instalado"
