package models

type RegisterRequest struct {
	Phone     string `json:"phone" binding:"required,min=10,max=15"`
	Password  string `json:"password" binding:"required,min=8,max=15"`
	FirstName string `json:"first_name" binding:"required,min=2,max=100"`
	LastName  string `json:"last_name" binding:"required,min=2,max=100"`
	Pin       string `json:"pin" binding:"required,min=4,max=6,numeric"`
	Email     string `json:"email" binding:"omitempty,email"`
}

type LoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required,min=8,max=15"`
	Pin      string `json:"pin" binding:"required"`
}

type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
}

type UserResponse struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email,omitempty"`
	Status    string `json:"status" binding:"omitempty,oneof=ACTIVE INACTIVE BLOCKED"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
