package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
	"github.com/diego-all/atento-agent-lab/backend/internal/service"
)

// ---- fakes de la capa service ----

type fakeCustomers struct {
	c   *domain.Customer
	err error
}

func (f fakeCustomers) GetByID(context.Context, int64) (*domain.Customer, error) { return f.c, f.err }
func (f fakeCustomers) GetByEmail(context.Context, string) (*domain.Customer, error) {
	return f.c, f.err
}
func (f fakeCustomers) List(context.Context, int, int) ([]domain.Customer, error) { return nil, f.err }

type fakeOrders struct {
	o   *domain.Order
	err error
}

func (f fakeOrders) GetByID(context.Context, int64) (*domain.Order, error) { return f.o, f.err }
func (f fakeOrders) ListByCustomer(context.Context, int64) ([]domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.Order{*f.o}, nil
}

type fakeTickets struct {
	t     *domain.Ticket
	err   error
	gotIn service.CreateTicketInput
}

func (f *fakeTickets) Create(_ context.Context, in service.CreateTicketInput) (*domain.Ticket, error) {
	f.gotIn = in
	return f.t, f.err
}
func (f *fakeTickets) GetByID(context.Context, int64) (*domain.Ticket, error) { return f.t, f.err }
func (f *fakeTickets) ListByCustomer(context.Context, int64) ([]domain.Ticket, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []domain.Ticket{*f.t}, nil
}
func (f *fakeTickets) Escalate(context.Context, int64, string) (*domain.Ticket, error) {
	return f.t, f.err
}

func specByName(t *testing.T, svc Services, name string) ToolSpec {
	t.Helper()
	for _, s := range Tools(svc) {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("tool %q not found", name)
	return ToolSpec{}
}

func testServices() (Services, *fakeTickets) {
	tickets := &fakeTickets{t: &domain.Ticket{ID: 5, CustomerID: 1}}
	return Services{
		Customers: fakeCustomers{c: &domain.Customer{ID: 1, Name: "Ana", Email: "ana@example.com", Phone: "+57 300", Segment: "premium"}},
		Orders:    fakeOrders{o: &domain.Order{ID: 7, CustomerID: 1, Status: "shipped"}},
		Tickets:   tickets,
	}, tickets
}

func TestTools_Catalog(t *testing.T) {
	want := []string{"get_customer", "get_order_status", "list_customer_orders", "list_customer_tickets", "create_ticket", "escalate_to_human"}
	specs := Tools(Services{})
	if len(specs) != len(want) {
		t.Fatalf("got %d tools, want %d", len(specs), len(want))
	}
	for i, name := range want {
		if specs[i].Name != name {
			t.Errorf("tool %d = %q, want %q", i, specs[i].Name, name)
		}
		if specs[i].Description == "" || specs[i].Handler == nil {
			t.Errorf("tool %q incomplete", name)
		}
		if specs[i].Access != "read" && specs[i].Access != "write" {
			t.Errorf("tool %q has invalid access %q", name, specs[i].Access)
		}
	}
}

func TestGetCustomer_ByEmailAndByID_MinimizesData(t *testing.T) {
	svc, _ := testServices()
	spec := specByName(t, svc, "get_customer")

	for _, args := range []map[string]any{
		{"email": "ana@example.com"},
		{"customer_id": float64(1)},
	} {
		out, err := spec.Handler(context.Background(), args)
		if err != nil {
			t.Fatal(err)
		}
		view, ok := out.(customerView)
		if !ok || view.Name != "Ana" || view.Segment != "premium" {
			t.Fatalf("unexpected output: %#v", out)
		}
	}
}

func TestGetCustomer_RequiresIdentifier(t *testing.T) {
	svc, _ := testServices()
	_, err := specByName(t, svc, "get_customer").Handler(context.Background(), map[string]any{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestGetCustomer_ServiceError(t *testing.T) {
	svc, _ := testServices()
	svc.Customers = fakeCustomers{err: domain.ErrNotFound}
	_, err := specByName(t, svc, "get_customer").Handler(context.Background(), map[string]any{"customer_id": 9})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestOrderTools(t *testing.T) {
	svc, _ := testServices()

	out, err := specByName(t, svc, "get_order_status").Handler(context.Background(), map[string]any{"order_id": float64(7)})
	if err != nil {
		t.Fatal(err)
	}
	if o, ok := out.(*domain.Order); !ok || o.Status != "shipped" {
		t.Fatalf("unexpected: %#v", out)
	}
	if _, err := specByName(t, svc, "get_order_status").Handler(context.Background(), map[string]any{}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}

	list, err := specByName(t, svc, "list_customer_orders").Handler(context.Background(), map[string]any{"customer_id": 1})
	if err != nil {
		t.Fatal(err)
	}
	if orders, ok := list.([]domain.Order); !ok || len(orders) != 1 {
		t.Fatalf("unexpected: %#v", list)
	}
	if _, err := specByName(t, svc, "list_customer_orders").Handler(context.Background(), nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestListCustomerTicketsTool(t *testing.T) {
	svc, _ := testServices()
	spec := specByName(t, svc, "list_customer_tickets")

	if spec.Access != "read" {
		t.Fatalf("list_customer_tickets must be read-only, got %q", spec.Access)
	}
	out, err := spec.Handler(context.Background(), map[string]any{"customer_id": float64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if tickets, ok := out.([]domain.Ticket); !ok || len(tickets) != 1 || tickets[0].ID != 5 {
		t.Fatalf("unexpected: %#v", out)
	}
	if _, err := spec.Handler(context.Background(), nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}

	svc.Tickets = &fakeTickets{err: domain.ErrNotFound}
	if _, err := specByName(t, svc, "list_customer_tickets").Handler(context.Background(), map[string]any{"customer_id": 1}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCreateTicketTool(t *testing.T) {
	svc, tickets := testServices()
	spec := specByName(t, svc, "create_ticket")

	out, err := spec.Handler(context.Background(), map[string]any{
		"customer_id": float64(1), "subject": "  Asunto ", "description": "Detalle", "priority": "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tk, ok := out.(*domain.Ticket); !ok || tk.ID != 5 {
		t.Fatalf("unexpected: %#v", out)
	}
	if tickets.gotIn.Subject != "Asunto" || tickets.gotIn.Priority != "high" || tickets.gotIn.CustomerID != 1 {
		t.Fatalf("service received %+v", tickets.gotIn)
	}

	if _, err := spec.Handler(context.Background(), map[string]any{"subject": "x"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestEscalateTool(t *testing.T) {
	svc, _ := testServices()
	spec := specByName(t, svc, "escalate_to_human")

	if _, err := spec.Handler(context.Background(), map[string]any{"ticket_id": float64(5), "reason": "molesto"}); err != nil {
		t.Fatal(err)
	}
	if _, err := spec.Handler(context.Background(), map[string]any{"reason": "x"}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestArgInt64(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want int64
		ok   bool
	}{
		{"float whole", float64(3), 3, true},
		{"float fractional", 3.5, 0, false},
		{"int", 4, 4, true},
		{"int64", int64(5), 5, true},
		{"string", "6", 0, false},
		{"missing", nil, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := argInt64(map[string]any{"k": tc.in}, "k")
			if got != tc.want || ok != tc.ok {
				t.Fatalf("got (%d,%v), want (%d,%v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestUserMessage_NeverLeaksInternals(t *testing.T) {
	if got := userMessage(domain.ErrNotFound); !strings.Contains(got, "No se encontró") {
		t.Errorf("not found message = %q", got)
	}
	invalid := errors.Join(domain.ErrInvalidInput, errors.New("order_id is required"))
	if got := userMessage(invalid); !strings.Contains(got, "order_id is required") || strings.Contains(got, "\n") {
		t.Errorf("invalid message = %q", got)
	}
	if got := userMessage(errors.New("pq: password authentication failed")); strings.Contains(got, "password") {
		t.Errorf("internal error leaked: %q", got)
	}
}

func TestCatalogMarkdown(t *testing.T) {
	md := CatalogMarkdown(Tools(Services{}))
	for _, want := range []string{"# Catálogo de tools MCP", "## `get_customer`", "## `escalate_to_human`", "| `order_id` | number | sí |"} {
		if !strings.Contains(md, want) {
			t.Errorf("catalog missing %q", want)
		}
	}
}
