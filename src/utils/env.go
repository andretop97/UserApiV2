package utils

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

type PostgresqlEnv struct {
	Host     string `env:"POSTGRESQL_HOST" envDefault:"localhost"`
	Port     string `env:"POSTGRESQL_PORT" envDefault:"5432"`
	Username string `env:"POSTGRESQL_USERNAME" envDefault:"root"`
	Password string `env:"POSTGRESQL_PASSWORD" envDefault:""`
	Database string `env:"POSTGRESQL_DATABASE" envDefault:"test"`
}

func NewPostgresqlEnv() (*PostgresqlEnv, error) {
	postgresqlEnv, err := env.ParseAs[PostgresqlEnv]()
	if err != nil {
		return nil, err
	}
	return &postgresqlEnv, nil
}

func (p *PostgresqlEnv) ConnectionString() string {
	u := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(p.Username, p.Password),
		Host:   p.Host + ":" + p.Port,
		Path:   "/" + p.Database,
	}

	return u.String()
}

type RedisEnv struct {
	Host     string `env:"REDIS_HOST" envDefault:"localhost"`
	Port     string `env:"REDIS_PORT" envDefault:"6379"`
	Password string `env:"REDIS_PASSWORD" envDefault:""`
	Database int    `env:"REDIS_DATABASE" envDefault:"0"`
}

func NewRedisEnv() (*RedisEnv, error) {
	redisEnv, err := env.ParseAs[RedisEnv]()
	if err != nil {
		return nil, err
	}

	return &redisEnv, nil
}

type CacheEnv struct {
	UserTTL time.Duration `env:"CACHE_USER_TTL" envDefault:"10m"`
}

func NewCacheEnv() (*CacheEnv, error) {
	cacheEnv, err := env.ParseAs[CacheEnv]()
	if err != nil {
		return nil, err
	}
	return &cacheEnv, nil
}

type AuthEnv struct {
	LoginTokenTTL      time.Duration `env:"LOGIN_TOKEN_TTL" envDefault:"10m"`
	SessionIdleTTL     time.Duration `env:"SESSION_IDLE_TTL" envDefault:"24h"`
	SessionAbsoluteTTL time.Duration `env:"SESSION_ABSOLUTE_TTL" envDefault:"72h"`
	MfaTokenSecret     string        `env:"MFA_TOKEN_SECRET,notEmpty"`
	SessionTokenSecret string        `env:"SESSION_TOKEN_SECRET,notEmpty"`
}

func NewAuthEnv() (*AuthEnv, error) {
	authEnv, err := env.ParseAs[AuthEnv]()
	if err != nil {
		return nil, err
	}
	return &authEnv, nil
}

type PepperEnv struct {
	Version int    `env:"PEPPER_VERSION" envDefault:"0"`
	Secret  string `env:"PEPPER_SECRETS,notEmpty"`
}

func NewPepperEnv() (*PepperEnv, error) {
	pepperEnv, err := env.ParseAs[PepperEnv]()
	if err != nil {
		return nil, err
	}
	return &pepperEnv, nil
}

type Argon2Env struct {
	Memory      uint32 `env:"ARGON2_MEMORY" envDefault:"65536"`
	Iterations  uint32 `env:"ARGON2_ITERATIONS" envDefault:"3"`
	Parallelism uint8  `env:"ARGON2_PARALLELISM" envDefault:"2"`
	SaltLength  uint32 `env:"ARGON2_SALT_LENGTH" envDefault:"16"`
	KeyLength   uint32 `env:"ARGON2_KEY_LENGTH" envDefault:"32"`
}

func NewArgon2Env() (*Argon2Env, error) {
	argon2Env, err := env.ParseAs[Argon2Env]()
	if err != nil {
		return nil, err
	}
	return &argon2Env, nil
}

type WebAuthnEnv struct {
	RPID        string        `env:"WEBAUTHN_RP_ID,notEmpty"`
	RPName      string        `env:"WEBAUTHN_RP_NAME" envDefault:"UserApiV2"`
	RPOrigins   []string      `env:"WEBAUTHN_RP_ORIGINS,notEmpty" envSeparator:","`
	CeremonyTTL time.Duration `env:"WEBAUTHN_CEREMONY_TTL" envDefault:"5m"`
}

func (we *WebAuthnEnv) validate() error {
	if strings.Contains(we.RPID, "://") || strings.Contains(we.RPID, ":") || strings.Contains(we.RPID, "/") {
		return fmt.Errorf("WEBAUTHN_RP_ID must be a bare host (no scheme, port or path): %q", we.RPID)
	}
	for _, origin := range we.RPOrigins {
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
			return fmt.Errorf("WEBAUTHN_RP_ORIGINS has an invalid origin (expected scheme://host[:port], no trailing slash): %q", origin)
		}
	}
	if we.CeremonyTTL <= 0 {
		return errors.New("WEBAUTHN_CEREMONY_TTL must be positive")
	}
	return nil
}

func NewWebAuthnEnv() (*WebAuthnEnv, error) {
	webAuthnEnv, err := env.ParseAs[WebAuthnEnv]()
	if err != nil {
		return nil, err
	}
	if err := webAuthnEnv.validate(); err != nil {
		return nil, err
	}

	return &webAuthnEnv, nil
}
