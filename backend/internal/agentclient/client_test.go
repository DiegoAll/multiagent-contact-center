package agentclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
)

func TestChat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Internal-Token"); got != "tok" {
			t.Errorf("token header = %q", got)
		}
		var req domain.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message != "hola" {
			t.Errorf("bad body: %+v, %v", req, err)
		}
		_ = json.NewEncoder(w).Encode(domain.ChatResponse{Reply: "¡Hola!", Escalated: true})
	}))
	defer srv.Close()

	c := New(srv.URL+"/", "tok", 2*time.Second) // la barra final se normaliza
	resp, err := c.Chat(context.Background(), domain.ChatRequest{Message: "hola"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Reply != "¡Hola!" || !resp.Escalated {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestChat_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL, "", time.Second).Chat(context.Background(), domain.ChatRequest{Message: "x"})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("want error mentioning status 500, got %v", err)
	}
}

func TestChat_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "", time.Second).Chat(context.Background(), domain.ChatRequest{Message: "x"}); err == nil {
		t.Fatal("want decode error")
	}
}

func TestChat_ConnectionError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nadie escucha

	if _, err := New(url, "", time.Second).Chat(context.Background(), domain.ChatRequest{Message: "x"}); err == nil {
		t.Fatal("want connection error")
	}
}

func TestChat_NoTokenHeaderWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Header["X-Internal-Token"]; ok {
			t.Error("token header should be absent")
		}
		_ = json.NewEncoder(w).Encode(domain.ChatResponse{})
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "", time.Second).Chat(context.Background(), domain.ChatRequest{Message: "x"}); err != nil {
		t.Fatal(err)
	}
}
