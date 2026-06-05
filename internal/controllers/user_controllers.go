package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
	"net/http"
)

type UserController struct {
	userService *services.UserService
}

func NewUserController(userService *services.UserService) *UserController {
	return &UserController{userService: userService}
}

func (ctrl *UserController) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	user, err := ctrl.userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "user_not_found",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, models.UserResponse{
		ID:        user.ID.String(),
		Phone:     user.Phone,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     emailString(user.Email),
		Status:    user.Status,
	})
}

func emailString(email *string) string {
	if email == nil {
		return ""
	}
	return *email
}
