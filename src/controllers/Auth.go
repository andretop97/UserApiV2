package controllers

import (
	"github.com/andretop97/UserApiV2/src/core"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authService core.AuthService
}

func NewAuthController(authService core.AuthService) *AuthController {
	return &AuthController{
		authService: authService,
	}
}

func (ac *AuthController) Login(c *gin.Context) {
	var loginRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	token, err := ac.authService.Login(c.Request.Context(), loginRequest.Email, loginRequest.Password)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"token": token})
}

func (ac *AuthController) RegisterWebAuthnBegin(c *gin.Context) {
}

func (ac *AuthController) RegisterWebAuthnFinish(c *gin.Context) {
}

func (ac *AuthController) WebAuthnBegin(c *gin.Context) {
}

func (ac *AuthController) WebAuthnFinish(c *gin.Context) {
}
