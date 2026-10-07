package core

import "context"

type AuthService interface {
	Login(ctx context.Context, email, password string) (*LoginResult, error)
	RegisterWebAuthnBegin(ctx context.Context, email string) (string, error)
	RegisterWebAuthnFinish(ctx context.Context, email, credential string) error
	WebAuthnBegin(ctx context.Context, email string) (string, error)
	WebAuthnFinish(ctx context.Context, email, credential string) error
}
