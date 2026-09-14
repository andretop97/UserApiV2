package dto

import (
	"time"

	"github.com/andretop97/UserApiV2/src/core"
)

type CreateUserRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (r *CreateUserRequest) ToUser() *core.User {
	return &core.User{
		Name:     r.Name,
		Email:    r.Email,
		Password: r.Password,
	}
}

type CreateUserResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt"`
}

func (r *CreateUserResponse) FromUser(user *core.User) {
	r.ID = user.ID.String()
	r.Name = user.Name
	r.Email = user.Email
	r.CreatedAt = user.CreatedAt
	r.UpdatedAt = user.UpdatedAt
	r.DeletedAt = user.DeletedAt
}

type UpdateUserRequest struct {
	Name string `json:"name" binding:"required"`
}

func (r *UpdateUserRequest) ToUser() *core.User {
	return &core.User{
		Name: r.Name,
	}
}

type UpdateUserResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt"`
}

func (r *UpdateUserResponse) FromUser(user *core.User) {
	r.ID = user.ID.String()
	r.Name = user.Name
	r.Email = user.Email
	r.CreatedAt = user.CreatedAt
	r.UpdatedAt = user.UpdatedAt
	r.DeletedAt = user.DeletedAt
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func (r *LoginResponse) FromUser(token *string) {
	r.Token = *token
}

type GetUserResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt"`
}

func (r *GetUserResponse) FromUser(user *core.User) {
	r.ID = user.ID.String()
	r.Name = user.Name
	r.Email = user.Email
	r.CreatedAt = user.CreatedAt
	r.UpdatedAt = user.UpdatedAt
	r.DeletedAt = user.DeletedAt
}
