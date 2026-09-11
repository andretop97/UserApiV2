package bootstrap

import (
	"context"

	"github.com/andretop97/UserApiV2/src/controllers"
	"github.com/andretop97/UserApiV2/src/repositories"
	"github.com/andretop97/UserApiV2/src/repositories/postgres"
	"github.com/andretop97/UserApiV2/src/repositories/redis"
	"github.com/andretop97/UserApiV2/src/routes"
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
	postgresqlConfig, err := utils.NewPostgresqlEnv()
	if err != nil {
		return nil, err
	}
	pgxPool, err := pgxpool.New(context.Background(), postgresqlConfig.ConnectionString())
	if err != nil {
		return nil, err
	}
	postgresRepository := postgres.NewUserRepository(pgxPool)

	redisConfig, err := utils.NewRedisEnv()
	if err != nil {
		return nil, err
	}
	redisClient := rdb.NewClient(&rdb.Options{
		Addr:     redisConfig.Host + ":" + redisConfig.Port,
		Password: redisConfig.Password,
		DB:       redisConfig.Database,
	})
	redisRepository := redis.NewUserRepository(redisClient)

	userRepository := repositories.NewUserRepository(redisRepository, postgresRepository)

	userService := services.NewUserService(userRepository)

	controllers := &routes.Controllers{
		User: controllers.NewUserController(userService),
	}

	return &Container{
		Controllers: controllers,
		PgPool:      pgxPool,
		RedisClient: redisClient,
	}, nil

}
