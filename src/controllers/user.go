package controllers

import (
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	createdUser, err := uc.userService.CreateUser(c.Request.Context(), user.ToUser())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response := &dto.CreateUserResponse{}
	response.FromUser(&createdUser)
	c.JSON(http.StatusCreated, response)
}

func (uc *UserController) GetUserByID(c *gin.Context) {
	var idParam string = c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := uc.userService.GetUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response := &dto.GetUserResponse{}
	response.FromUser(&user)
	c.JSON(http.StatusOK, response)
}

func (uc *UserController) GetUserByName(c *gin.Context) {
	var nameParam string = c.Param("name")
	user, err := uc.userService.GetUserByName(c.Request.Context(), nameParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response := &dto.GetUserResponse{}
	response.FromUser(&user)
	c.JSON(http.StatusOK, response)
}

func (uc *UserController) GetUserByEmail(c *gin.Context) {
	var emailParam string = c.Param("email")
	user, err := uc.userService.GetUserByEmail(c.Request.Context(), emailParam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response := &dto.GetUserResponse{}
	response.FromUser(&user)
	c.JSON(http.StatusOK, response)
}

func (uc *UserController) GetAllUsers(c *gin.Context) {
	users, err := uc.userService.GetAllUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var userResponses []dto.GetUserResponse
	for _, user := range users {
		response := &dto.GetUserResponse{}
		response.FromUser(&user)
		userResponses = append(userResponses, *response)
	}
	c.JSON(http.StatusOK, userResponses)
}

func (uc *UserController) UpdateUser(c *gin.Context) {
	var user dto.UpdateUserRequest
	if err := c.ShouldBind(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedUser, err := uc.userService.UpdateUser(c.Request.Context(), user.ToUser())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response := &dto.UpdateUserResponse{}
	response.FromUser(&updatedUser)
	c.JSON(http.StatusOK, response)
}
func (uc *UserController) DeleteUser(c *gin.Context) {
	var idParam string = c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = uc.userService.DeleteUser(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (uc *UserController) LoginUser(c *gin.Context) {
	var user dto.LoginRequest
	if err := c.ShouldBind(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := uc.userService.LoginUser(c.Request.Context(), user.Email, user.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	loginResponse := &dto.LoginResponse{
		Token: token,
	}
	c.JSON(http.StatusOK, loginResponse)
}
