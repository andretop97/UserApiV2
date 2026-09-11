package main

import (
	"github.com/andretop97/UserApiV2/src/controllers"
	"github.com/andretop97/UserApiV2/src/middlewares"
	"github.com/andretop97/UserApiV2/src/routes"
	"github.com/andretop97/UserApiV2/src/services"
	"github.com/andretop97/UserApiV2/src/utils"
	"github.com/gin-gonic/gin"
)

// func GetCachedRepository() core.UserRepository {
// 	redis := rdb.NewClient(&rdb.Options{
// 		Addr:     "localhost:6379",
// 		Password: "",
// 		DB:       0,
// 	})
// 	redisRepository := redis.NewUserRepository(redis)

// 	pgxPool, err := pgxpool.New(context.Background(), "postgresql://user:password@localhost/dbname")
// 	if err != nil {
// 		panic(err)
// 	}
// 	postgresRepository := postgres.NewUserRepository(pgxPool)

// 	return repositories.NewUserRepository(redisRepository, postgresRepository)
// }

func main() {
	utils.SetLoggerSettings()

	userService := services.NewUserService()
	controllers := &routes.Controllers{
		User: controllers.NewUserController(userService),
	}
	router := gin.New()
	router.Use(middlewares.SlogLogger())
	router.Use(middlewares.SlogRecovery())

	router = routes.Routes(router, controllers)
	router.Run(":8080")
}
