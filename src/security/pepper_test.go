package security_test

import (
	"encoding/base64"
	"testing"

	"github.com/andretop97/UserApiV2/src/security"
	"github.com/andretop97/UserApiV2/src/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPepperProvider(t *testing.T) {
	t.Run("versão única válida", func(t *testing.T) {
		secret := base64.StdEncoding.EncodeToString([]byte("meu-pepper"))
		env := &utils.PepperEnv{Version: 1, Secret: "1:" + secret}

		provider, err := security.NewPepperProvider(env)

		require.NoError(t, err)
		version, got := provider.Current()
		assert.Equal(t, 1, version)
		assert.Equal(t, []byte("meu-pepper"), got)
	})

	t.Run("múltiplas versões, Current aponta pra atual e Get acessa qualquer uma", func(t *testing.T) {
		s1 := base64.StdEncoding.EncodeToString([]byte("pepper-v1"))
		s2 := base64.StdEncoding.EncodeToString([]byte("pepper-v2"))
		env := &utils.PepperEnv{Version: 2, Secret: "1:" + s1 + ",2:" + s2}

		provider, err := security.NewPepperProvider(env)
		require.NoError(t, err)

		version, current := provider.Current()
		assert.Equal(t, 2, version)
		assert.Equal(t, []byte("pepper-v2"), current)

		old, ok := provider.Get(1)
		assert.True(t, ok)
		assert.Equal(t, []byte("pepper-v1"), old)
	})

	t.Run("par sem separador retorna erro", func(t *testing.T) {
		env := &utils.PepperEnv{Version: 1, Secret: "semseparador"}

		_, err := security.NewPepperProvider(env)

		assert.Error(t, err)
	})

	t.Run("versão não numérica retorna erro", func(t *testing.T) {
		secret := base64.StdEncoding.EncodeToString([]byte("pepper"))
		env := &utils.PepperEnv{Version: 1, Secret: "abc:" + secret}

		_, err := security.NewPepperProvider(env)

		assert.Error(t, err)
	})

	t.Run("base64 inválido retorna erro", func(t *testing.T) {
		env := &utils.PepperEnv{Version: 1, Secret: "1:isso-nao-e-base64-!!"}

		_, err := security.NewPepperProvider(env)

		assert.Error(t, err)
	})

	t.Run("versão atual ausente no mapa retorna erro", func(t *testing.T) {
		secret := base64.StdEncoding.EncodeToString([]byte("pepper"))
		env := &utils.PepperEnv{Version: 99, Secret: "1:" + secret}

		_, err := security.NewPepperProvider(env)

		assert.Error(t, err)
	})
}

func TestPepperProvider_Get(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte("pepper"))
	env := &utils.PepperEnv{Version: 1, Secret: "1:" + secret}
	provider, err := security.NewPepperProvider(env)
	require.NoError(t, err)

	t.Run("versão existente", func(t *testing.T) {
		got, ok := provider.Get(1)
		assert.True(t, ok)
		assert.Equal(t, []byte("pepper"), got)
	})

	t.Run("versão inexistente", func(t *testing.T) {
		got, ok := provider.Get(999)
		assert.False(t, ok)
		assert.Nil(t, got)
	})
}
