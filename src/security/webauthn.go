package security

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

type WebAuthnVerifier struct {
	w *webauthn.WebAuthn
}

func NewWebAuthnVerifier(w *webauthn.WebAuthn) (core.WebAuthnVerifier, error) {
	if w == nil || w.Config == nil {
		return nil, errors.New("webauthn verifier: a configured instance is required")
	}
	return &WebAuthnVerifier{
		w: w,
	}, nil
}

func (wf *WebAuthnVerifier) BeginRegistration(user *core.User, existing []*core.Passkey) (options json.RawMessage, session []byte, err error) {
	credentials, err := toCredentials(existing)
	if err != nil {
		return nil, nil, err
	}

	creation, sessionData, err := wf.w.BeginRegistration(
		webauthnUser{
			user:  user,
			creds: credentials,
		},
		webauthn.WithExclusions(
			webauthn.Credentials(credentials).CredentialDescriptors()),
		webauthn.WithAuthenticatorSelection(
			protocol.AuthenticatorSelection{
				ResidentKey:      protocol.ResidentKeyRequirementPreferred,
				UserVerification: protocol.VerificationRequired,
			}),
	)

	if err != nil {
		return nil, nil, fmt.Errorf("begin registration: %w", err)
	}

	options, err = json.Marshal(creation)
	if err != nil {
		return nil, nil, fmt.Errorf("encode registration options: %w", err)
	}

	session, err = json.Marshal(sessionData)
	if err != nil {
		return nil, nil, fmt.Errorf("encode registration session: %w", err)
	}

	return options, session, nil
}

func (wf *WebAuthnVerifier) FinishRegistration(user *core.User, existing []*core.Passkey, session []byte, response []byte) (*core.Passkey, error) {
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session, &sessionData); err != nil {
		return nil, fmt.Errorf("decode registration session: %w", err)
	}
	credentials, err := toCredentials(existing)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(response))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid registration response", core.ErrBadRequest)
	}
	cred, err := wf.w.CreateCredential(webauthnUser{
		user:  user,
		creds: credentials,
	}, sessionData, parsed)

	if err != nil {
		slog.Warn("passkey registration rejected", "user_id", user.ID, "reason", err)
		return nil, core.ErrPasskeyInvalid
	}
	return toPasskey(user.ID, cred)
}

func (wf *WebAuthnVerifier) BeginLogin(user *core.User, existing []*core.Passkey) (options json.RawMessage, session []byte, err error) {
	credentials, err := toCredentials(existing)
	if err != nil {
		return nil, nil, err
	}

	creation, sessionData, err := wf.w.BeginLogin(
		webauthnUser{
			user:  user,
			creds: credentials,
		},
		webauthn.WithUserVerification(protocol.VerificationRequired))

	if err != nil {
		return nil, nil, fmt.Errorf("begin login: %w", err)
	}

	options, err = json.Marshal(creation)
	if err != nil {
		return nil, nil, fmt.Errorf("encode login options: %w", err)
	}

	session, err = json.Marshal(sessionData)
	if err != nil {
		return nil, nil, fmt.Errorf("encode login session: %w", err)
	}

	return options, session, nil
}

func (wf *WebAuthnVerifier) FinishLogin(user *core.User, existing []*core.Passkey, session []byte, response []byte) (*core.Passkey, error) {
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session, &sessionData); err != nil {
		return nil, fmt.Errorf("decode login session: %w", err)
	}
	credentials, err := toCredentials(existing)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(response))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid login response", core.ErrBadRequest)
	}
	cred, err := wf.w.ValidateLogin(webauthnUser{
		user:  user,
		creds: credentials,
	}, sessionData, parsed)
	if err != nil {
		slog.Warn("passkey login rejected", "user_id", user.ID, "reason", err)
		return nil, core.ErrPasskeyInvalid
	}

	if cred.Authenticator.CloneWarning {
		slog.Warn("passkey clone warning", "user_id", user.ID, "credential_id", cred.ID)
		return nil, core.ErrPasskeyInvalid
	}

	return toPasskey(user.ID, cred)
}

type webauthnUser struct {
	user  *core.User
	creds []webauthn.Credential
}

func (u webauthnUser) WebAuthnID() []byte                         { return u.user.ID[:] }
func (u webauthnUser) WebAuthnName() string                       { return u.user.Email }
func (u webauthnUser) WebAuthnDisplayName() string                { return u.user.Name }
func (u webauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func toCredentials(passkeys []*core.Passkey) ([]webauthn.Credential, error) {
	creds := make([]webauthn.Credential, 0, len(passkeys))
	for _, p := range passkeys {
		var c webauthn.Credential
		if err := json.Unmarshal(p.Data, &c); err != nil {
			return nil, fmt.Errorf("decode passkey: %w", err)
		}
		creds = append(creds, c)
	}
	return creds, nil
}

func toPasskey(userID uuid.UUID, c *webauthn.Credential) (*core.Passkey, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode passkey: %w", err)
	}
	return &core.Passkey{ID: c.ID, UserID: userID, Data: data}, nil
}
