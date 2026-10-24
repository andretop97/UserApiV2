package dto

import "github.com/andretop97/UserApiV2/src/core"

type AuthLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthLoginResponse struct {
	Status    string `json:"status"`
	Token     string `json:"mfa_token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
	NextStep  string `json:"next_step"`
}

func (r *AuthLoginResponse) FromLogin(login *core.LoginResult) {
	r.Status = "mfa_required"
	r.Token = login.Token
	r.TokenType = "Bearer"
	r.ExpiresIn = int(login.ExpiresIn.Seconds())
	r.NextStep = string(login.NextStep)
}

type AuthSessionResponse struct {
	Status    string `json:"status"`
	Token     string `json:"session_token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
}

func (r *AuthSessionResponse) FromSession(session *core.SessionResult) {
	r.Status = "authenticated"
	r.Token = session.Token
	r.TokenType = "Bearer"
	r.ExpiresIn = int(session.ExpiresIn.Seconds())
}
