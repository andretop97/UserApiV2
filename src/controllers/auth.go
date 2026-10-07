package controllers

import (
	"fmt"
	"net/http"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/dto"
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
	var loginRequest dto.AuthLoginRequest
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		respondError(c, fmt.Errorf("%w: %s", core.ErrBadRequest, validationErrorMessage(err)))
		return
	}
	loginResponse, err := ac.authService.Login(c.Request.Context(), loginRequest.Email, loginRequest.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	response := &dto.AuthLoginResponse{}
	response.FromLogin(loginResponse)
	c.JSON(http.StatusOK, response)
}

func (ac *AuthController) RegisterWebAuthnBegin(c *gin.Context) {
}

func (ac *AuthController) RegisterWebAuthnFinish(c *gin.Context) {
}

func (ac *AuthController) WebAuthnBegin(c *gin.Context) {
}

func (ac *AuthController) WebAuthnFinish(c *gin.Context) {
}
