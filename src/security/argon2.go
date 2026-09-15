package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"

	"github.com/andretop97/UserApiV2/src/core"
	"golang.org/x/crypto/argon2"
)

type Argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

type PasswordEncryption struct {
	params         Argon2Params
	pepperProvider core.PepperProvider
}

func NewPasswordEncryption(params Argon2Params, pepperProvider core.PepperProvider) core.PasswordEncryption {
	return &PasswordEncryption{
		params:         params,
		pepperProvider: pepperProvider,
	}
}

func (p *PasswordEncryption) generateSalt() ([]byte, error) {
	salt := make([]byte, p.params.saltLength)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, err
	}
	return salt, nil
}

func (p *PasswordEncryption) Hash(password string) (encodedHash string, pepperVersion int, err error) {
	salt, err := p.generateSalt()
	if err != nil {
		return "", 0, nil
	}
	version, pepper := p.pepperProvider.Current()
	key := []byte(password)
	h := hmac.New(sha256.New, pepper)
	h.Write([]byte(password))
	h.Write(key)
	hashed := argon2.IDKey(key, salt, p.params.iterations, p.params.memory, p.params.parallelism, p.params.keyLength)
	return string(hashed), version, nil
}

func (p *PasswordEncryption) Verify(password, encodedHash string, pepperVersion int) (bool, error) {

	return false, nil
}
