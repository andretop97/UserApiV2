package dto

import "github.com/andretop97/UserApiV2/src/core"

type AuthLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthLoginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
	NextStep  string `json:"next_step"`
}

func (r *AuthLoginResponse) FromLogin(login *core.LoginResult) {
	r.Token = login.Token
	r.TokenType = "Bearer"
	r.ExpiresIn = int(login.ExpiresIn.Seconds())
	r.NextStep = string(login.NextStep)
}
