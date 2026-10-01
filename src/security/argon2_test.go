package security

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/argon2"
)

type fakePepperProvider struct {
	version int
	secrets map[int][]byte
}

func (f *fakePepperProvider) Current() (int, []byte) {
	return f.version, f.secrets[f.version]
}

func (f *fakePepperProvider) Get(version int) ([]byte, bool) {
	secret, ok := f.secrets[version]
	return secret, ok
}

func testParams() Argon2Params {
	return Argon2Params{
		memory:      8 * 1024,
		iterations:  1,
		parallelism: 1,
		saltLength:  16,
		keyLength:   32,
	}
}

func newTestEncryption() (core.PasswordEncryption, *fakePepperProvider) {
	provider := &fakePepperProvider{
		version: 1,
		secrets: map[int][]byte{
			1: []byte("pepper-v1"),
			2: []byte("pepper-v2"),
		},
	}
	pe := NewPasswordEncryption(testParams(), provider)
	return pe, provider
}

func TestAddPepper(t *testing.T) {
	h1 := addPepper("senha", []byte("pepper1"))
	h2 := addPepper("senha", []byte("pepper2"))
	h3 := addPepper("senha", []byte("pepper1"))

	assert.NotEqual(t, h1, h2)
	assert.Equal(t, h1, h3)
}

func TestEncodeDecode(t *testing.T) {
	params := testParams()
	salt := []byte("saltdetestecom16b")
	hash := []byte("hashdetesteparaoargon2idfake123")

	encoded := encode(params, salt, hash)
	decodedParams, decodedSalt, decodedHash, err := decode(encoded)

	require.NoError(t, err)
	assert.Equal(t, params.memory, decodedParams.memory)
	assert.Equal(t, params.iterations, decodedParams.iterations)
	assert.Equal(t, params.parallelism, decodedParams.parallelism)
	assert.Equal(t, salt, decodedSalt)
	assert.Equal(t, hash, decodedHash)
}

func TestDecode_Errors(t *testing.T) {
	t.Run("string sem os 6 segmentos", func(t *testing.T) {
		_, _, _, err := decode("hash-invalido")

		assert.Error(t, err)
	})

	t.Run("versão de argon2 incompatível", func(t *testing.T) {
		encoded := fmt.Sprintf("$argon2id$v=%d$m=8192,t=1,p=1$salt$hash", 999)

		_, _, _, err := decode(encoded)

		assert.Error(t, err)
	})

	t.Run("base64 do salt inválido", func(t *testing.T) {
		hash := base64.RawStdEncoding.EncodeToString([]byte("hash-teste"))
		encoded := fmt.Sprintf("$argon2id$v=%d$m=8192,t=1,p=1$%s$%s", argon2.Version, "!!!invalido!!!", hash)

		_, _, _, err := decode(encoded)

		assert.Error(t, err)
	})
}

func TestHash(t *testing.T) {
	t.Run("gera hash no formato esperado e retorna a versão atual do pepper", func(t *testing.T) {
		pe, _ := newTestEncryption()

		encodedHash, version, err := pe.Hash("minhaSenha123")

		require.NoError(t, err)
		assert.Equal(t, 1, version)
		assert.True(t, strings.HasPrefix(encodedHash, "$argon2id$v="))
	})

	t.Run("duas chamadas para a mesma senha geram hashes diferentes (salt aleatório)", func(t *testing.T) {
		pe, _ := newTestEncryption()

		hash1, _, err := pe.Hash("minhaSenha123")
		require.NoError(t, err)
		hash2, _, err := pe.Hash("minhaSenha123")
		require.NoError(t, err)

		assert.NotEqual(t, hash1, hash2)
	})
}

func TestVerify(t *testing.T) {
	t.Run("senha correta retorna true", func(t *testing.T) {
		pe, _ := newTestEncryption()
		encodedHash, version, err := pe.Hash("minhaSenha123")
		require.NoError(t, err)

		ok, err := pe.Verify("minhaSenha123", encodedHash, version)

		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("senha incorreta retorna false", func(t *testing.T) {
		pe, _ := newTestEncryption()
		encodedHash, version, err := pe.Hash("minhaSenha123")
		require.NoError(t, err)

		ok, err := pe.Verify("senhaErrada", encodedHash, version)

		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("hash gerado antes de uma rotação de pepper continua verificável", func(t *testing.T) {
		pe, provider := newTestEncryption()

		encodedHash, version, err := pe.Hash("minhaSenha123")
		require.NoError(t, err)
		assert.Equal(t, 1, version)

		provider.version = 2 // simula rotação: a versão atual passa a ser 2

		ok, err := pe.Verify("minhaSenha123", encodedHash, version) // continua pedindo a v1
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("versão de pepper desconhecida retorna erro", func(t *testing.T) {
		pe, _ := newTestEncryption()
		encodedHash, _, err := pe.Hash("minhaSenha123")
		require.NoError(t, err)

		ok, err := pe.Verify("minhaSenha123", encodedHash, 999)

		assert.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("encodedHash malformado retorna erro", func(t *testing.T) {
		pe, _ := newTestEncryption()

		ok, err := pe.Verify("minhaSenha123", "hash-invalido", 1)

		assert.Error(t, err)
		assert.False(t, ok)
	})
}
