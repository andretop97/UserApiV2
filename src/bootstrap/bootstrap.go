package bootstrap

import (
	"context"
	"errors"
	"log/slog"

	"github.com/andretop97/UserApiV2/src/controllers"
	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/migrations"
	"github.com/andretop97/UserApiV2/src/repositories"
	"github.com/andretop97/UserApiV2/src/repositories/postgres"
	"github.com/andretop97/UserApiV2/src/repositories/redis"
	"github.com/andretop97/UserApiV2/src/routes"
	"github.com/andretop97/UserApiV2/src/security"
	"github.com/andretop97/UserApiV2/src/services"
	"github.com/andretop97/UserApiV2/src/utils"
	"github.com/jackc/pgx/v5/pgxpool"
	rdb "github.com/redis/go-redis/v9"
)

type Container struct {
	Controllers *routes.Controllers
	closers     []func() error
}

func (c *Container) Close() error {
	var err error
	for i := len(c.closers) - 1; i >= 0; i-- {
		err = errors.Join(err, c.closers[i]())
	}
	return err
}

func NewContainer() (container *Container, err error) {
	redisConfig, err := utils.NewRedisEnv()
	if err != nil {
		return nil, err
	}

	postgresqlConfig, err := utils.NewPostgresqlEnv()
	if err != nil {
		return nil, err
	}

	cacheEnv, err := utils.NewCacheEnv()
	if err != nil {
		return nil, err
	}

	pepperConfig, err := utils.NewPepperEnv()
	if err != nil {
		return nil, err
	}

	argonEnv, err := utils.NewArgon2Env()
	if err != nil {
		return nil, err
	}

	pepperProvider, err := security.NewPepperProvider(pepperConfig)
	if err != nil {
		return nil, err
	}

	authEnv, err := utils.NewAuthEnv()
	if err != nil {
		return nil, err
	}

	var closers []func() error

	defer func() {
		if err != nil {
			for i := len(closers) - 1; i >= 0; i-- { // ordem inversa
				err = errors.Join(err, closers[i]())
			}
		}
	}()

	redisClient := rdb.NewClient(&rdb.Options{
		Addr:     redisConfig.Host + ":" + redisConfig.Port,
		Password: redisConfig.Password,
		DB:       redisConfig.Database,
	})
	closers = append(closers, redisClient.Close)

	healthCheck := redis.NewHealthCheck(redisClient)
	err = healthCheck.HandShake()
	if err != nil {
		return nil, err
	}

	err = migrations.AutoMigrate()
	if err != nil {
		return nil, err
	}

	pgxPool, err := pgxpool.New(context.Background(), postgresqlConfig.ConnectionString())
	if err != nil {
		return nil, err
	}
	closers = append(closers, func() error { pgxPool.Close(); return nil })

	postgresRepository := postgres.NewUserRepository(pgxPool)

	redisRepository, err := redis.NewUserCache(redisClient, cacheEnv.UserTTL)
	if err != nil {
		return nil, err
	}

	userRepository := repositories.NewUserRepository(redisRepository, postgresRepository, slog.Default())

	argonConfig := security.NewArgon2Params(argonEnv)
	passwordEncryption := security.NewPasswordEncryption(argonConfig, pepperProvider)
	userService := services.NewUserService(userRepository, passwordEncryption)

	authRepository := redis.NewAuthRepository(redisClient, authEnv)
	mfaTokens, err := security.NewJwtProvider[core.MfaClaims](security.JwtConfig{
		SecretKey: []byte("your-secret-key-here-should-be-at-least-32-bytes-long"),
		TTL:       authEnv.LoginTokenTTL, // 15 minutes
		Issuer:    "user-api",
		Audience:  "mfa",
	})
	if err != nil {
		return nil, err
	}
	sessionTokens, err := security.NewJwtProvider[core.SessionClaims](security.JwtConfig{
		SecretKey: []byte("your-secret-key-here-should-be-at-least-32-bytes-long"),
		TTL:       authEnv.SessionIdleTTL, // 24 hours
		Issuer:    "user-api",
		Audience:  "session",
	})
	if err != nil {
		return nil, err
	}

	authService := services.NewAuthService(userRepository, authRepository, passwordEncryption, mfaTokens, sessionTokens)

	routerControllers := &routes.Controllers{
		User: controllers.NewUserController(userService),
		Auth: controllers.NewAuthController(authService),
	}

	return &Container{
		Controllers: routerControllers,
		closers:     closers,
	}, nil

}
