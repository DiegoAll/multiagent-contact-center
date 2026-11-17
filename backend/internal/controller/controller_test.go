package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

// ---- fakes de la capa service ----

type fakeCustomerSvc struct {
	customer *domain.Customer
	err      error
}

func (f fakeCustomerSvc) GetByID(context.Context, int64) (*domain.Customer, error) {
	return f.customer, f.err
}
func (f fakeCustomerSvc) GetByEmail(context.Context, string) (*domain.Customer, error) {
	return f.customer, f.err
}
func (f fakeCustomerSvc) List(context.Context, int, int) ([]domain.Customer, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.Customer{*f.customer}, nil
}

type fakeOrderSvc struct {
	order *domain.Order
	err   error
}

func (f fakeOrderSvc) GetByID(context.Context, int64) (*domain.Order, error) { return f.order, f.err }
func (f fakeOrderSvc) ListByCustomer(context.Context, int64) ([]domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.Order{*f.order}, nil
}

type fakeTicketSvc struct {
	ticket *domain.Ticket
	err    error
	gotIn  service.CreateTicketInput
}

func (f *fakeTicketSvc) Create(_ context.Context, in service.CreateTicketInput) (*domain.Ticket, error) {
	f.gotIn = in
	return f.ticket, f.err
}
func (f *fakeTicketSvc) GetByID(context.Context, int64) (*domain.Ticket, error) {
	return f.ticket, f.err
}
func (f *fakeTicketSvc) ListByCustomer(context.Context, int64) ([]domain.Ticket, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.Ticket{*f.ticket}, nil
}
func (f *fakeTicketSvc) Escalate(context.Context, int64, string) (*domain.Ticket, error) {
	return f.ticket, f.err
}

type fakeChatSvc struct {
	resp *domain.ChatResponse
	err  error
}

func (f fakeChatSvc) Chat(context.Context, domain.ChatRequest) (*domain.ChatResponse, error) {
	return f.resp, f.err
}

// ---- helpers ----

func newTestRouter(t *testing.T, mod func(*Deps)) http.Handler {
	t.Helper()
	swagger := filepath.Join(t.TempDir(), "swagger.json")
	if err := os.WriteFile(swagger, []byte(`{"swagger":"2.0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := Deps{
		Customers:   fakeCustomerSvc{customer: &domain.Customer{ID: 1, Name: "Ana", Email: "ana@example.com"}},
		Orders:      fakeOrderSvc{order: &domain.Order{ID: 7, CustomerID: 1, Status: "shipped"}},
		Tickets:     &fakeTicketSvc{ticket: &domain.Ticket{ID: 3, CustomerID: 1, Subject: "s"}},
		Chat:        fakeChatSvc{resp: &domain.ChatResponse{Reply: "hola", SessionID: "abc"}},
		SwaggerFile: swagger,
	}
	if mod != nil {
		mod(&d)
	}
	return NewRouter(d)
}

func request(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// ---- tests ----

func TestRoutes_HappyPaths(t *testing.T) {
	h := newTestRouter(t, nil)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"list customers", http.MethodGet, "/api/v1/customers?limit=5&offset=1", "", 200},
		{"get customer", http.MethodGet, "/api/v1/customers/1", "", 200},
		{"lookup customer", http.MethodGet, "/api/v1/customers/lookup?email=ana@example.com", "", 200},
		{"customer orders", http.MethodGet, "/api/v1/customers/1/orders", "", 200},
		{"customer tickets", http.MethodGet, "/api/v1/customers/1/tickets", "", 200},
		{"get order", http.MethodGet, "/api/v1/orders/7", "", 200},
		{"get ticket", http.MethodGet, "/api/v1/tickets/3", "", 200},
		{"create ticket", http.MethodPost, "/api/v1/tickets", `{"customer_id":1,"subject":"s","description":"d"}`, 201},
		{"escalate ticket", http.MethodPost, "/api/v1/tickets/3/escalate", `{"reason":"molesto"}`, 200},
		{"chat", http.MethodPost, "/api/v1/chat", `{"message":"hola"}`, 200},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(h, tc.method, tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.want, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("content-type = %q", ct)
			}
		})
	}
}

func TestRoutes_InvalidIDs(t *testing.T) {
	h := newTestRouter(t, nil)
	for _, path := range []string{
		"/api/v1/customers/abc",
		"/api/v1/customers/0",
		"/api/v1/customers/-1/orders",
		"/api/v1/customers/x/tickets",
		"/api/v1/orders/abc",
		"/api/v1/tickets/abc",
	} {
		if rec := request(h, http.MethodGet, path, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400", path, rec.Code)
		}
	}
	if rec := request(h, http.MethodPost, "/api/v1/tickets/abc/escalate", `{"reason":"x"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("escalate bad id status = %d, want 400", rec.Code)
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"invalid input", domain.ErrInvalidInput, http.StatusBadRequest},
		{"upstream", domain.ErrUpstream, http.StatusBadGateway},
		{"unknown", errors.New("secret internal detail"), http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestRouter(t, func(d *Deps) {
				d.Customers = fakeCustomerSvc{err: tc.err}
				d.Orders = fakeOrderSvc{err: tc.err}
				d.Tickets = &fakeTicketSvc{err: tc.err}
				d.Chat = fakeChatSvc{err: tc.err}
			})
			for _, path := range []string{
				"/api/v1/customers", "/api/v1/customers/1", "/api/v1/customers/lookup?email=a@b.co",
				"/api/v1/customers/1/orders", "/api/v1/customers/1/tickets",
				"/api/v1/orders/1", "/api/v1/tickets/1",
			} {
				rec := request(h, http.MethodGet, path, "")
				if rec.Code != tc.want {
					t.Fatalf("%s status = %d, want %d", path, rec.Code, tc.want)
				}
				if strings.Contains(rec.Body.String(), "secret internal detail") {
					t.Fatalf("internal error leaked in response: %s", rec.Body.String())
				}
			}
			for _, call := range []struct{ path, body string }{
				{"/api/v1/tickets", `{"customer_id":1,"subject":"s","description":"d"}`},
				{"/api/v1/tickets/1/escalate", `{"reason":"r"}`},
				{"/api/v1/chat", `{"message":"hola"}`},
			} {
				if rec := request(h, http.MethodPost, call.path, call.body); rec.Code != tc.want {
					t.Fatalf("%s status = %d, want %d", call.path, rec.Code, tc.want)
				}
			}
		})
	}
}

func TestBodyValidation(t *testing.T) {
	h := newTestRouter(t, nil)
	tests := []struct {
		name, path, body string
	}{
		{"malformed json", "/api/v1/chat", `{"message":`},
		{"unknown field rejected", "/api/v1/chat", `{"message":"hola","admin":true}`},
		{"ticket malformed", "/api/v1/tickets", `not json`},
		{"escalate malformed", "/api/v1/tickets/1/escalate", `[]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if rec := request(h, http.MethodPost, tc.path, tc.body); rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestBodyTooLarge(t *testing.T) {
	h := newTestRouter(t, nil)
	big := `{"message":"` + strings.Repeat("a", maxBodyBytes+10) + `"}`
	if rec := request(h, http.MethodPost, "/api/v1/chat", big); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateTicket_PassesInputToService(t *testing.T) {
	svc := &fakeTicketSvc{ticket: &domain.Ticket{ID: 9}}
	h := newTestRouter(t, func(d *Deps) { d.Tickets = svc })

	rec := request(h, http.MethodPost, "/api/v1/tickets",
		`{"customer_id":2,"subject":"Asunto","description":"Detalle","priority":"high"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if svc.gotIn.CustomerID != 2 || svc.gotIn.Priority != "high" || svc.gotIn.Subject != "Asunto" {
		t.Fatalf("service received %+v", svc.gotIn)
	}
	var got domain.Ticket
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.ID != 9 {
		t.Fatalf("bad body: %s (%v)", rec.Body.String(), err)
	}
}

func TestAPIKeyProtectsAPIButNotHealth(t *testing.T) {
	h := newTestRouter(t, func(d *Deps) { d.APIKey = "secret" })

	if rec := request(h, http.MethodGet, "/api/v1/customers", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("api without key = %d, want 401", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil)
	req.Header.Set("X-API-Key", "secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("api with key = %d, want 200", rec.Code)
	}
	if rec := request(h, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", rec.Code)
	}
}

func TestRateLimitApplied(t *testing.T) {
	h := newTestRouter(t, func(d *Deps) { d.RateLimitPerMin = 1 })
	if rec := request(h, http.MethodGet, "/api/v1/customers", ""); rec.Code != http.StatusOK {
		t.Fatalf("first = %d", rec.Code)
	}
	if rec := request(h, http.MethodGet, "/api/v1/customers", ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second = %d, want 429", rec.Code)
	}
}

func TestReadyz(t *testing.T) {
	ready := newTestRouter(t, func(d *Deps) { d.Ready = func(context.Context) error { return nil } })
	if rec := request(ready, http.MethodGet, "/readyz", ""); rec.Code != http.StatusOK {
		t.Fatalf("ready = %d", rec.Code)
	}
	notReady := newTestRouter(t, func(d *Deps) { d.Ready = func(context.Context) error { return errors.New("db down") } })
	if rec := request(notReady, http.MethodGet, "/readyz", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("not ready = %d, want 503", rec.Code)
	}
	noCheck := newTestRouter(t, nil)
	if rec := request(noCheck, http.MethodGet, "/readyz", ""); rec.Code != http.StatusOK {
		t.Fatalf("no check = %d", rec.Code)
	}
}

func TestSwaggerEndpoints(t *testing.T) {
	h := newTestRouter(t, nil)

	if rec := request(h, http.MethodGet, "/swagger", ""); rec.Code != http.StatusMovedPermanently {
		t.Fatalf("/swagger = %d, want 301", rec.Code)
	}
	rec := request(h, http.MethodGet, "/swagger/", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SwaggerUIBundle") {
		t.Fatalf("/swagger/ = %d, body: %s", rec.Code, rec.Body.String())
	}
	rec = request(h, http.MethodGet, "/swagger/doc.json", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"swagger"`) {
		t.Fatalf("/swagger/doc.json = %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestQueryInt(t *testing.T) {
	tests := []struct {
		query string
		want  int
	}{
		{"", 20},
		{"?limit=5", 5},
		{"?limit=abc", 20},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/x"+tc.query, nil)
		if got := queryInt(req, "limit", 20); got != tc.want {
			t.Errorf("queryInt(%q) = %d, want %d", tc.query, got, tc.want)
		}
	}
}
