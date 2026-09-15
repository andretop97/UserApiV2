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
	Secret  string `env:"PEPPER_SECRETS" envDefault:"0:base64(TestePepper)"`
}
