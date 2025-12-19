package models

type RegisterRequest struct {
	Phone     string `json:"phone" binding:"required,min=10,max=15"`
	Pin       string `json:"pin" binding:"required,min=4,max=6,numeric"`
	FirstName string `json:"first_name" binding:"omitempty,min=2,max=100"`
	LastName  string `json:"last_name" binding:"omitempty,min=2,max=100"`
	Email     string `json:"email" binding:"omitempty,email"`
}

type LoginRequest struct {
	Phone string `json:"phone" binding:"required"`
	Pin   string `json:"pin" binding:"required"`
}

type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Phone     string `json:"phone"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
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
