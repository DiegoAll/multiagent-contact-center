package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
)

// ---- fakes en memoria que implementan las interfaces de repository ----

type fakeCustomers struct {
	items      map[int64]*domain.Customer
	err        error
	lastLimit  int
	lastOffset int
}

func (f *fakeCustomers) GetByID(_ context.Context, id int64) (*domain.Customer, error) {
	if f.err != nil {
		return nil, f.err
	}
	c, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeCustomers) GetByEmail(_ context.Context, email string) (*domain.Customer, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, c := range f.items {
		if c.Email == email {
			return c, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeCustomers) List(_ context.Context, limit, offset int) ([]domain.Customer, error) {
	f.lastLimit, f.lastOffset = limit, offset
	if f.err != nil {
		return nil, f.err
	}
	return []domain.Customer{{ID: 1}}, nil
}

type fakeOrders struct {
	items map[int64]*domain.Order
	err   error
}

func (f *fakeOrders) GetByID(_ context.Context, id int64) (*domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	o, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return o, nil
}

func (f *fakeOrders) ListByCustomer(_ context.Context, customerID int64) ([]domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.Order
	for _, o := range f.items {
		if o.CustomerID == customerID {
			out = append(out, *o)
		}
	}
	return out, nil
}

type fakeTickets struct {
	items map[int64]*domain.Ticket
	next  int64
	err   error
}

func newFakeTickets() *fakeTickets {
	return &fakeTickets{items: map[int64]*domain.Ticket{}, next: 1}
}

func (f *fakeTickets) Create(_ context.Context, t *domain.Ticket) error {
	if f.err != nil {
		return f.err
	}
	t.ID = f.next
	f.next++
	f.items[t.ID] = t
	return nil
}

func (f *fakeTickets) GetByID(_ context.Context, id int64) (*domain.Ticket, error) {
	if f.err != nil {
		return nil, f.err
	}
	t, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (f *fakeTickets) ListByCustomer(_ context.Context, customerID int64) ([]domain.Ticket, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.Ticket
	for _, t := range f.items {
		if t.CustomerID == customerID {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (f *fakeTickets) Escalate(_ context.Context, id int64, reason string) (*domain.Ticket, error) {
	t, err := f.GetByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	t.Escalated, t.EscalationReason, t.Status = true, reason, domain.TicketEscalated
	return t, nil
}

type fakeAgent struct {
	resp *domain.ChatResponse
	err  error
	got  domain.ChatRequest
}

func (f *fakeAgent) Chat(_ context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
	f.got = req
	return f.resp, f.err
}

func seededCustomers() *fakeCustomers {
	return &fakeCustomers{items: map[int64]*domain.Customer{
		1: {ID: 1, Name: "Ana", Email: "ana@example.com"},
	}}
}

// ---- CustomerService ----

func TestCustomerService_GetByID(t *testing.T) {
	svc := NewCustomerService(seededCustomers())
	ctx := context.Background()

	if _, err := svc.GetByID(ctx, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.GetByID(ctx, 99); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	c, err := svc.GetByID(ctx, 1)
	if err != nil || c.Name != "Ana" {
		t.Fatalf("unexpected result: %+v, %v", c, err)
	}
}

func TestCustomerService_GetByEmail(t *testing.T) {
	svc := NewCustomerService(seededCustomers())
	ctx := context.Background()

	tests := []struct {
		name    string
		email   string
		wantErr error
	}{
		{"valid, normalizes case and spaces", "  ANA@example.com ", nil},
		{"invalid format", "not-an-email", domain.ErrInvalidInput},
		{"display name rejected", "Ana <ana@example.com>", domain.ErrInvalidInput},
		{"not found", "nobody@example.com", domain.ErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GetByEmail(ctx, tc.email)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestCustomerService_ListClampsPagination(t *testing.T) {
	repo := seededCustomers()
	svc := NewCustomerService(repo)

	tests := []struct{ in, wantLimit, off, wantOff int }{
		{0, 20, -5, 0},
		{500, 100, 10, 10},
		{50, 50, 0, 0},
	}
	for _, tc := range tests {
		if _, err := svc.List(context.Background(), tc.in, tc.off); err != nil {
			t.Fatal(err)
		}
		if repo.lastLimit != tc.wantLimit || repo.lastOffset != tc.wantOff {
			t.Fatalf("limit/offset = %d/%d, want %d/%d", repo.lastLimit, repo.lastOffset, tc.wantLimit, tc.wantOff)
		}
	}
}

// ---- OrderService ----

func TestOrderService(t *testing.T) {
	repo := &fakeOrders{items: map[int64]*domain.Order{
		7: {ID: 7, CustomerID: 1, Status: "shipped"},
	}}
	svc := NewOrderService(repo)
	ctx := context.Background()

	if _, err := svc.GetByID(ctx, -1); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.GetByID(ctx, 8); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	o, err := svc.GetByID(ctx, 7)
	if err != nil || o.Status != "shipped" {
		t.Fatalf("unexpected: %+v, %v", o, err)
	}

	if _, err := svc.ListByCustomer(ctx, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	list, err := svc.ListByCustomer(ctx, 1)
	if err != nil || len(list) != 1 {
		t.Fatalf("unexpected: %+v, %v", list, err)
	}
}

// ---- TicketService ----

func TestTicketService_Create(t *testing.T) {
	long := strings.Repeat("a", maxSubjectRunes+1)
	tests := []struct {
		name    string
		in      CreateTicketInput
		wantErr error
	}{
		{"ok default priority", CreateTicketInput{CustomerID: 1, Subject: "Hola", Description: "Detalle"}, nil},
		{"ok explicit priority", CreateTicketInput{CustomerID: 1, Subject: "Hola", Description: "Detalle", Priority: " URGENT "}, nil},
		{"bad customer id", CreateTicketInput{CustomerID: 0, Subject: "Hola", Description: "x"}, domain.ErrInvalidInput},
		{"empty subject", CreateTicketInput{CustomerID: 1, Subject: "  ", Description: "x"}, domain.ErrInvalidInput},
		{"subject too long", CreateTicketInput{CustomerID: 1, Subject: long, Description: "x"}, domain.ErrInvalidInput},
		{"empty description", CreateTicketInput{CustomerID: 1, Subject: "Hola"}, domain.ErrInvalidInput},
		{"bad priority", CreateTicketInput{CustomerID: 1, Subject: "Hola", Description: "x", Priority: "critical"}, domain.ErrInvalidInput},
		{"unknown customer", CreateTicketInput{CustomerID: 42, Subject: "Hola", Description: "x"}, domain.ErrNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewTicketService(newFakeTickets(), seededCustomers())
			got, err := svc.Create(context.Background(), tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
			if tc.wantErr == nil {
				if got.ID == 0 || got.Status != domain.TicketOpen {
					t.Fatalf("unexpected ticket: %+v", got)
				}
			}
		})
	}
}

func TestTicketService_DefaultAndNormalizedPriority(t *testing.T) {
	svc := NewTicketService(newFakeTickets(), seededCustomers())
	a, _ := svc.Create(context.Background(), CreateTicketInput{CustomerID: 1, Subject: "s", Description: "d"})
	b, _ := svc.Create(context.Background(), CreateTicketInput{CustomerID: 1, Subject: "s", Description: "d", Priority: " HIGH "})
	if a.Priority != domain.PriorityMedium || b.Priority != domain.PriorityHigh {
		t.Fatalf("priorities = %q, %q", a.Priority, b.Priority)
	}
}

func TestTicketService_RepoErrorPropagates(t *testing.T) {
	boom := errors.New("db down")
	repo := newFakeTickets()
	repo.err = boom
	svc := NewTicketService(repo, seededCustomers())
	if _, err := svc.Create(context.Background(), CreateTicketInput{CustomerID: 1, Subject: "s", Description: "d"}); !errors.Is(err, boom) {
		t.Fatalf("want repo error, got %v", err)
	}
}

func TestTicketService_GetListEscalate(t *testing.T) {
	repo := newFakeTickets()
	svc := NewTicketService(repo, seededCustomers())
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateTicketInput{CustomerID: 1, Subject: "s", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.GetByID(ctx, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if got, err := svc.GetByID(ctx, created.ID); err != nil || got.ID != created.ID {
		t.Fatalf("unexpected: %+v, %v", got, err)
	}

	if _, err := svc.ListByCustomer(ctx, -1); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if list, err := svc.ListByCustomer(ctx, 1); err != nil || len(list) != 1 {
		t.Fatalf("unexpected: %+v, %v", list, err)
	}

	if _, err := svc.Escalate(ctx, 0, "motivo"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.Escalate(ctx, created.ID, "   "); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if _, err := svc.Escalate(ctx, 999, "motivo"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	esc, err := svc.Escalate(ctx, created.ID, "cliente molesto")
	if err != nil || !esc.Escalated || esc.Status != domain.TicketEscalated {
		t.Fatalf("unexpected: %+v, %v", esc, err)
	}
}

// ---- ChatService ----

func TestChatService_Validation(t *testing.T) {
	tests := []struct {
		name string
		req  domain.ChatRequest
	}{
		{"empty message", domain.ChatRequest{Message: "   "}},
		{"message too long", domain.ChatRequest{Message: strings.Repeat("a", maxMessageRunes+1)}},
		{"session id too long", domain.ChatRequest{Message: "hola", SessionID: strings.Repeat("s", maxSessionIDLen+1)}},
		{"invalid email", domain.ChatRequest{Message: "hola", CustomerEmail: "nope"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewChatService(&fakeAgent{resp: &domain.ChatResponse{}})
			if _, err := svc.Chat(context.Background(), tc.req); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestChatService_GeneratesSessionAndNormalizes(t *testing.T) {
	agent := &fakeAgent{resp: &domain.ChatResponse{Reply: "hola"}}
	svc := NewChatService(agent)

	resp, err := svc.Chat(context.Background(), domain.ChatRequest{Message: "  ¿Dónde está mi pedido?  ", CustomerEmail: " ANA@Example.com "})
	if err != nil {
		t.Fatal(err)
	}
	if agent.got.Message != "¿Dónde está mi pedido?" || agent.got.CustomerEmail != "ana@example.com" {
		t.Fatalf("request not normalized: %+v", agent.got)
	}
	if agent.got.SessionID == "" || resp.SessionID != agent.got.SessionID {
		t.Fatalf("session id not generated/propagated: %q / %q", agent.got.SessionID, resp.SessionID)
	}
}

func TestChatService_KeepsAgentSessionID(t *testing.T) {
	agent := &fakeAgent{resp: &domain.ChatResponse{Reply: "ok", SessionID: "from-agent"}}
	resp, err := NewChatService(agent).Chat(context.Background(), domain.ChatRequest{Message: "hola", SessionID: "mine"})
	if err != nil || resp.SessionID != "from-agent" {
		t.Fatalf("unexpected: %+v, %v", resp, err)
	}
}

func TestChatService_AgentErrorIsUpstream(t *testing.T) {
	agent := &fakeAgent{err: errors.New("timeout")}
	_, err := NewChatService(agent).Chat(context.Background(), domain.ChatRequest{Message: "hola"})
	if !errors.Is(err, domain.ErrUpstream) {
		t.Fatalf("want ErrUpstream, got %v", err)
	}
}
