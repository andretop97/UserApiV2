package controllers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

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
	mfaToken, err := bearerToken(c)
	if err != nil {
		respondError(c, err)
		return
	}
	options, err := ac.authService.RegisterWebAuthnBegin(c.Request.Context(), mfaToken)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, options)
}

func (ac *AuthController) RegisterWebAuthnFinish(c *gin.Context) {
	mfaToken, err := bearerToken(c)
	if err != nil {
		respondError(c, err)
		return
	}

	response, err := readCeremonyResponse(c)
	if err != nil {
		respondError(c, err)
		return
	}
	session, err := ac.authService.RegisterWebAuthnFinish(c.Request.Context(), mfaToken, response)
	if err != nil {
		respondError(c, err)
		return
	}

	body := &dto.AuthSessionResponse{}
	body.FromSession(session)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, body)
}

func (ac *AuthController) WebAuthnBegin(c *gin.Context) {
	mfaToken, err := bearerToken(c)
	if err != nil {
		respondError(c, err)
		return
	}
	options, err := ac.authService.WebAuthnBegin(c.Request.Context(), mfaToken)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, options)
}

func (ac *AuthController) WebAuthnFinish(c *gin.Context) {
	mfaToken, err := bearerToken(c)
	if err != nil {
		respondError(c, err)
		return
	}

	response, err := readCeremonyResponse(c)
	if err != nil {
		respondError(c, err)
		return
	}
	session, err := ac.authService.WebAuthnFinish(c.Request.Context(), mfaToken, response)
	if err != nil {
		respondError(c, err)
		return
	}

	body := &dto.AuthSessionResponse{}
	body.FromSession(session)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, body)
}

func bearerToken(c *gin.Context) (string, error) {
	header := c.GetHeader("Authorization")
	scheme, token, ok := strings.Cut(header, " ")
	token = strings.TrimSpace(token)
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", core.ErrTokenInvalid
	}
	return token, nil
}

const maxCeremonyResponseBytes = 64 << 10 // 64 KB: uma resposta real tem poucos KB

func readCeremonyResponse(c *gin.Context) ([]byte, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCeremonyResponseBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, fmt.Errorf("%w: body too large", core.ErrBadRequest)
		}
		return nil, fmt.Errorf("%w: unreadable body", core.ErrBadRequest)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("%w: empty body", core.ErrBadRequest)
	}
	return body, nil
}
