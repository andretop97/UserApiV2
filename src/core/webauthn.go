package core

import "encoding/json"

type WebAuthnVerifier interface {
	BeginRegistration(user *User, existing []*Passkey) (options json.RawMessage, session []byte, err error)
	FinishRegistration(user *User, existing []*Passkey, session []byte, response []byte) (*Passkey, error)

	BeginLogin(user *User, existing []*Passkey) (options json.RawMessage, session []byte, err error)
	FinishLogin(user *User, existing []*Passkey, session []byte, response []byte) (*Passkey, error)
}
