# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY . .
# El repo no versiona go.sum: se resuelve aquí. Para builds reproducibles,
# ejecuta `make tidy` localmente y versiona go.mod/go.sum.
RUN go mod tidy && go mod download
# Genera docs/swagger.json a partir de las anotaciones swaggo.
RUN swag init -g cmd/api/main.go --parseInternal -o docs --outputTypes json
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mcp ./cmd/mcp

FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/api /out/mcp /app/
COPY --from=build /src/docs/swagger.json /app/docs/swagger.json
USER app
EXPOSE 8080 8081
# Por defecto arranca la API; el servicio MCP usa command: ["/app/mcp"].
CMD ["/app/api"]
