package dto

import "github.com/andretop97/UserApiV2/src/core"

type CreateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

func (r *CreateUserRequest) ToUser() *core.User {
	return &core.User{
		Name:  r.Name,
		Email: r.Email,
	}
}

type CreateUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (r *CreateUserResponse) FromUser(user *core.User) {
	r.ID = user.ID.String()
	r.Name = user.Name
	r.Email = user.Email
}

type UpdateUserRequest struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required,email"`
}

func (r *UpdateUserRequest) ToUser() *core.User {
	return &core.User{
		Name:  r.Name,
		Email: r.Email,
	}
}

type UpdateUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (r *UpdateUserResponse) FromUser(user *core.User) {
	r.ID = user.ID.String()
	r.Name = user.Name
	r.Email = user.Email
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type GetUserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (r *GetUserResponse) FromUser(user *core.User) {
	r.ID = user.ID.String()
	r.Name = user.Name
	r.Email = user.Email
}
