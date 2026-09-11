package routes

import (
	"github.com/andretop97/UserApiV2/src/controllers"
	"github.com/gin-gonic/gin"
)

type Controllers struct {
	User *controllers.UserController
}

func Routes(routes *gin.Engine, c *Controllers) *gin.Engine {
	v1 := routes.Group("/v1")
	UserRoutes(v1, c.User)
	return routes
}
