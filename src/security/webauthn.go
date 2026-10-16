package security

import (
	"encoding/json"
	"fmt"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

type WebAuthnVerifier struct {
}

func NewWebAuthnVerifier() core.WebAuthnVerifier {
	return &WebAuthnVerifier{}
}

func (wf *WebAuthnVerifier) BeginRegistration(user *core.User, existing []*core.Passkey) (options json.RawMessage, session []byte, err error) {
	return nil, nil, nil
}

func (wf *WebAuthnVerifier) FinishRegistration(user *core.User, existing []*core.Passkey, session []byte, response []byte) (*core.Passkey, error) {
	return nil, nil
}
func (wf *WebAuthnVerifier) BeginLogin(user *core.User, existing []*core.Passkey) (options json.RawMessage, session []byte, err error) {
	return nil, nil, nil
}
func (wf *WebAuthnVerifier) FinishLogin(user *core.User, existing []*core.Passkey, session []byte, response []byte) (*core.Passkey, error) {
	return nil, nil
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
