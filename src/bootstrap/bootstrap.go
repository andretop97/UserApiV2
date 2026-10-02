package bootstrap

import (
	"context"

	"github.com/andretop97/UserApiV2/src/controllers"
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
	PgPool      *pgxpool.Pool
	RedisClient *rdb.Client
}

func NewContainer() (*Container, error) {
	redisConfig, err := utils.NewRedisEnv()
	if err != nil {
		return nil, err
	}
	redisClient := rdb.NewClient(&rdb.Options{
		Addr:     redisConfig.Host + ":" + redisConfig.Port,
		Password: redisConfig.Password,
		DB:       redisConfig.Database,
	})

	healthCheck := redis.NewHealthCheck(redisClient)
	err = healthCheck.HandShake()
	if err != nil {
		return nil, err
	}

	postgresqlConfig, err := utils.NewPostgresqlEnv()
	if err != nil {
		redisClient.Close()
		return nil, err
	}

	err = migrations.AutoMigrate()
	if err != nil {
		redisClient.Close()
		return nil, err
	}

	pgxPool, err := pgxpool.New(context.Background(), postgresqlConfig.ConnectionString())
	if err != nil {
		redisClient.Close()
		return nil, err
	}

	postgresRepository := postgres.NewUserRepository(pgxPool)

	redisRepository := redis.NewUserCache(redisClient)

	userRepository := repositories.NewUserRepository(redisRepository, postgresRepository)

	pepperConfig, err := utils.NewPepperEnv()
	if err != nil {
		return nil, err
	}
	pepperProvider, err := security.NewPepperProvider(pepperConfig)
	if err != nil {
		return nil, err
	}

	argonEnv, err := utils.NewArgon2Env()
	if err != nil {
		return nil, err
	}
	argonConfig := security.NewArgon2Params(argonEnv)
	passwordEncryption := security.NewPasswordEncryption(argonConfig, pepperProvider)
	userService := services.NewUserService(userRepository, passwordEncryption)

	controllers := &routes.Controllers{
		User: controllers.NewUserController(userService),
	}

	return &Container{
		Controllers: controllers,
		PgPool:      pgxPool,
		RedisClient: redisClient,
	}, nil

}
