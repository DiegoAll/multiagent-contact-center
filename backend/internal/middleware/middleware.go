// Package middleware agrupa los middlewares HTTP propios (auth, rate limit,
// cabeceras de seguridad y logging).
package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}

// HeaderTokenAuth exige que la cabecera indicada coincida con el token
// esperado (comparación en tiempo constante). Si token está vacío, no aplica.
func HeaderTokenAuth(header, token string) func(http.Handler) http.Handler {
	if token == "" {
		return func(next http.Handler) http.Handler { return next }
	}
	want := []byte(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := []byte(r.Header.Get(header))
			if subtle.ConstantTimeCompare(got, want) != 1 {
				jsonError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// APIKeyAuth protege rutas con la cabecera X-API-Key.
func APIKeyAuth(key string) func(http.Handler) http.Handler {
	return HeaderTokenAuth("X-API-Key", key)
}

// SecurityHeaders añade cabeceras defensivas a todas las respuestas.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// RateLimit limita las peticiones por IP en ventanas fijas de un minuto.
// perMin <= 0 desactiva el límite.
func RateLimit(perMin int) func(http.Handler) http.Handler {
	if perMin <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	var (
		mu     sync.Mutex
		counts = map[string]int{}
		window = time.Now()
	)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientKey(r)

			mu.Lock()
			if time.Since(window) >= time.Minute {
				counts = map[string]int{}
				window = time.Now()
			}
			counts[key]++
			n := counts[key]
			mu.Unlock()

			if n > perMin {
				w.Header().Set("Retry-After", "60")
				jsonError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RequestLogger registra método, ruta, estado y duración. No registra query
// strings ni cuerpos para evitar filtrar datos personales en los logs.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			log.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", chimw.GetReqID(r.Context()),
			)
		})
	}
}
