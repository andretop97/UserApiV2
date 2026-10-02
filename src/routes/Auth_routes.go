package routes

import (
	"github.com/andretop97/UserApiV2/src/controllers"
	"github.com/gin-gonic/gin"
)

func AuthRoutes(routes *gin.RouterGroup, authController *controllers.AuthController) {
	user := routes.Group("/auth")
	user.POST("/login", authController.Login)
	user.POST("/webauthn/begin", authController.WebAuthnBegin)
	user.POST("/webauthn/finish", authController.WebAuthnFinish)

	user.POST("/webauthn/register/begin", authController.RegisterWebAuthnBegin)
	user.POST("/webauthn/register/finish", authController.RegisterWebAuthnFinish)
}
