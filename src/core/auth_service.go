package core

import (
	"context"
	"encoding/json"
	"time"
)

type AuthService interface {
	Login(ctx context.Context, email, password string) (*LoginResult, error)
	RegisterWebAuthnBegin(ctx context.Context, mfaToken string) (json.RawMessage, error)
	RegisterWebAuthnFinish(ctx context.Context, mfaToken string, response []byte) (*SessionResult, error)
	WebAuthnBegin(ctx context.Context, mfaToken string) (json.RawMessage, error)
	WebAuthnFinish(ctx context.Context, mfaToken string, response []byte) (*SessionResult, error)
}

type SessionResult struct {
	Token     string
	ExpiresIn time.Duration
}
