package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/utils"
	"golang.org/x/crypto/argon2"
)

type Argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

func NewArgon2Params(e *utils.Argon2Env) Argon2Params {
	return Argon2Params{
		memory:      e.Memory,
		iterations:  e.Iterations,
		parallelism: e.Parallelism,
		saltLength:  e.SaltLength,
		keyLength:   e.KeyLength,
	}
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

func addPepper(password string, pepper []byte) []byte {
	h := hmac.New(sha256.New, pepper)
	h.Write([]byte(password))
	return h.Sum(nil)
}

func encode(params Argon2Params, salt, hash []byte) string {
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, params.memory, params.iterations, params.parallelism,
		b64Salt, b64Hash,
	)
}

func decode(encoded string) (params Argon2Params, salt, hash []byte, err error) {
	parts := strings.Split(encoded, "$")

	if len(parts) != 6 {
		return params, nil, nil, errors.New("malformed encoded string")
	}
	var version int
	_, err = fmt.Sscanf(parts[2], "v=%d", &version)
	if err != nil {
		return params, nil, nil, err
	}
	if version != argon2.Version {
		return params, nil, nil, errors.New("incompatible Argon2 version")
	}

	_, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.memory, &params.iterations, &params.parallelism)
	if err != nil {
		return params, nil, nil, err
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return params, nil, nil, err
	}
	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return params, nil, nil, err
	}
	params.keyLength = uint32(len(hash))

	return params, salt, hash, nil
}

func (p *PasswordEncryption) Hash(password string) (encodedHash string, pepperVersion int, err error) {
	salt, err := p.generateSalt()
	if err != nil {
		return "", 0, err
	}
	version, pepper := p.pepperProvider.Current()
	peppered := addPepper(password, pepper)
	hashed := argon2.IDKey(peppered, salt, p.params.iterations, p.params.memory, p.params.parallelism, p.params.keyLength)
	return encode(p.params, salt, hashed), version, nil
}

func (p *PasswordEncryption) Verify(password, encodedHash string, pepperVersion int) (bool, error) {
	params, salt, hash, err := decode(encodedHash)
	if err != nil {
		return false, err
	}

	pepper, ok := p.pepperProvider.Get(pepperVersion)
	if !ok {
		return false, fmt.Errorf("pepper version not found")
	}

	peppered := addPepper(password, pepper)
	hashedPassword := argon2.IDKey(peppered, salt, params.iterations, params.memory, params.parallelism, params.keyLength)

	isEqual := subtle.ConstantTimeCompare(hash, hashedPassword) == 1
	return isEqual, nil
}
