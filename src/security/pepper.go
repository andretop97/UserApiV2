package security

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/utils"
)

type PepperProvider struct {
	current   int
	byVersion map[int][]byte
}

func NewPepperProvider(e *utils.PepperEnv) (core.PepperProvider, error) {
	byVersion := make(map[int][]byte)
	for _, pair := range strings.Split(e.Secret, ",") {
		parts := strings.SplitN(pair, ":", 2)

		if len(parts) != 2 {
			return nil, fmt.Errorf("pepper: env pepper with wrong format")
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, err
		}
		secret, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}
		byVersion[version] = secret
	}

	if _, ok := byVersion[e.Version]; !ok {
		return nil, fmt.Errorf("pepper: current version %d not found in PEPPER_SECRETS", e.Version)
	}

	return &PepperProvider{
		current:   e.Version,
		byVersion: byVersion,
	}, nil
}

func (p *PepperProvider) Current() (int, []byte) {
	return p.current, p.byVersion[p.current]
}

func (p *PepperProvider) Get(version int) ([]byte, bool) {
	pepper, ok := p.byVersion[version]
	if !ok {
		return nil, false
	}
	return pepper, true
}
