package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/diego-all/atento-agent-lab/backend/internal/domain"
)

const (
	maxMessageRunes = 2000
	maxSessionIDLen = 64
)

type chatService struct {
	agent AgentClient
}

// NewChatService crea el servicio de chat sobre el cliente del agente.
func NewChatService(agent AgentClient) ChatService {
	return &chatService{agent: agent}
}

func (s *chatService) Chat(ctx context.Context, req domain.ChatRequest) (*domain.ChatResponse, error) {
	req.Message = strings.TrimSpace(req.Message)
	req.CustomerEmail = strings.ToLower(strings.TrimSpace(req.CustomerEmail))

	if req.Message == "" {
		return nil, invalid("message is required")
	}
	if utf8.RuneCountInString(req.Message) > maxMessageRunes {
		return nil, invalid("message is too long (max 2000 characters)")
	}
	if len(req.SessionID) > maxSessionIDLen {
		return nil, invalid("session_id is too long")
	}
	if req.CustomerEmail != "" {
		if addr, err := mail.ParseAddress(req.CustomerEmail); err != nil || addr.Address != req.CustomerEmail {
			return nil, invalid("customer_email is not valid")
		}
	}
	if req.SessionID == "" {
		req.SessionID = newSessionID()
	}

	resp, err := s.agent.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUpstream, err)
	}
	if resp.SessionID == "" {
		resp.SessionID = req.SessionID
	}
	return resp, nil
}

func newSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "session"
	}
	return hex.EncodeToString(b)
}
