package controllers

import (
	"fmt"
	"net/http"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/dto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserController struct {
	userService core.UserService
}

func NewUserController(userService core.UserService) *UserController {
	return &UserController{
		userService: userService,
	}
}

func (uc *UserController) CreateUser(c *gin.Context) {
	var user dto.CreateUserRequest
	if err := c.ShouldBind(&user); err != nil {
		respondError(c, fmt.Errorf("%w: %s", core.ErrBadRequest, validationErrorMessage(err)))
		return
	}
	createdUser, err := uc.userService.CreateUser(c.Request.Context(), user.ToUser())
	if err != nil {
		respondError(c, err)
		return
	}
	response := &dto.CreateUserResponse{}
	response.FromUser(createdUser)
	c.JSON(http.StatusCreated, response)
}

func (uc *UserController) GetUserByID(c *gin.Context) {
	var idParam string = c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		respondError(c, fmt.Errorf("%w: %s", core.ErrBadRequest, err))
		return
	}
	user, err := uc.userService.GetUserByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	response := &dto.GetUserResponse{}
	response.FromUser(user)
	c.JSON(http.StatusOK, response)
}

func (uc *UserController) GetUsersByName(c *gin.Context) {
	var nameParam string = c.Param("name")
	users, err := uc.userService.GetUsersByName(c.Request.Context(), nameParam)
	if err != nil {
		respondError(c, err)
		return
	}
	var userResponses []dto.GetUserResponse
	for _, user := range users {
		response := &dto.GetUserResponse{}
		response.FromUser(user)
		userResponses = append(userResponses, *response)
	}
	c.JSON(http.StatusOK, userResponses)
}

func (uc *UserController) GetUserByEmail(c *gin.Context) {
	var emailParam string = c.Param("email")
	user, err := uc.userService.GetUserByEmail(c.Request.Context(), emailParam)
	if err != nil {
		respondError(c, err)
		return
	}
	response := &dto.GetUserResponse{}
	response.FromUser(user)
	c.JSON(http.StatusOK, response)
}

func (uc *UserController) GetAllUsers(c *gin.Context) {
	users, err := uc.userService.GetAllUsers(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	var userResponses []dto.GetUserResponse
	for _, user := range users {
		response := &dto.GetUserResponse{}
		response.FromUser(user)
		userResponses = append(userResponses, *response)
	}
	c.JSON(http.StatusOK, userResponses)
}

func (uc *UserController) UpdateUser(c *gin.Context) {
	var user dto.UpdateUserRequest
	if err := c.ShouldBind(&user); err != nil {
		respondError(c, fmt.Errorf("%w: %s", core.ErrBadRequest, validationErrorMessage(err)))
		return
	}
	updatedUser, err := uc.userService.UpdateUser(c.Request.Context(), user.ToUser())
	if err != nil {
		respondError(c, err)
		return
	}
	response := &dto.UpdateUserResponse{}
	response.FromUser(updatedUser)
	c.JSON(http.StatusOK, response)
}
func (uc *UserController) DeleteUser(c *gin.Context) {
	var idParam string = c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		respondError(c, fmt.Errorf("%w: %s", core.ErrBadRequest, err))
		return
	}
	err = uc.userService.DeleteUser(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}
