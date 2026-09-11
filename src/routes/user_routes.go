package routes

import (
	"github.com/andretop97/UserApiV2/src/controllers"
	"github.com/gin-gonic/gin"
)

func UserRoutes(routes *gin.RouterGroup, userController *controllers.UserController) {
	user := routes.Group("/user")
	user.GET("/", userController.GetAllUsers)
	user.GET("/:id", userController.GetUserByID)
	user.GET("/name/:name", userController.GetUserByName)
	user.GET("/email/:email", userController.GetUserByEmail)
	user.POST("/", userController.CreateUser)
	user.PUT("/:id", userController.UpdateUser)
	user.DELETE("/:id", userController.DeleteUser)
	user.POST("/login", userController.LoginUser)
}
