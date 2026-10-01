package utils

import (
	"net/url"

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

type PepperEnv struct {
	Version int    `env:"PEPPER_VERSION" envDefault:"0"`
	Secret  string `env:"PEPPER_SECRETS" envDefault:"0:VGVzdGVQZXBwZXI="`
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
