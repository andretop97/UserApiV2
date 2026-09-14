package core

import "errors"

var (
	ErrBadRequest         = errors.New("bad request")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
)
