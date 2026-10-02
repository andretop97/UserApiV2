package controllers

import (
	"github.com/andretop97/UserApiV2/src/core"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	userService core.UserService
}

func NewAuthController() *AuthController {
	return &AuthController{}
}

func (ac *AuthController) Login(c *gin.Context) {
}

func (ac *AuthController) RegisterWebAuthnBegin(c *gin.Context) {
}

func (ac *AuthController) RegisterWebAuthnFinish(c *gin.Context) {
}

func (ac *AuthController) WebAuthnBegin(c *gin.Context) {
}

func (ac *AuthController) WebAuthnFinish(c *gin.Context) {
}
