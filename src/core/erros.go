package core

import "net/http"

type DomainError struct {
	Code    string
	Status  int
	message string
}

func (e *DomainError) Error() string { return e.message }

var (
	ErrUserCreationFailed = &DomainError{Code: "USER_CREATION_FAILED", Status: http.StatusInternalServerError, message: "creation failed"}
	ErrBadRequest         = &DomainError{Code: "BAD_REQUEST", Status: http.StatusBadRequest, message: "bad request"}
	ErrUserNotFound       = &DomainError{Code: "USER_NOT_FOUND", Status: http.StatusNotFound, message: "user not found"}
	ErrEmailAlreadyExists = &DomainError{Code: "EMAIL_ALREADY_EXISTS", Status: http.StatusConflict, message: "email already registered"}
	ErrInvalidCredentials = &DomainError{Code: "INVALID_CREDENTIALS", Status: http.StatusUnauthorized, message: "invalid email or password"}
)
