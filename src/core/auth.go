package core

import "time"

type AuthStep string

const (
	StepWebAuthn         AuthStep = "webauthn"
	StepWebAuthnRegister AuthStep = "webauthn_register"
)

type LoginResult struct {
	Token     string
	ExpiresIn time.Duration
	NextStep  AuthStep
}
