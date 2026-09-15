package security

import (
	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/utils"
)

type PepperProvider struct {
}

func NewPepperProvider(e *utils.PepperEnv) (core.PepperProvider, error) {

	return &PepperProvider{}, nil
}

func (p *PepperProvider) Current() (int, []byte) {
	return 0, nil
}

func (p *PepperProvider) Get(version int) ([]byte, bool) {
	return nil, false
}
