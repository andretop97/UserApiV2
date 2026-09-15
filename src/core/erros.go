package core

import "errors"

var (
	ErrUserCreationFailed = errors.New("creation failed")
	ErrBadRequest         = errors.New("bad request")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
)
