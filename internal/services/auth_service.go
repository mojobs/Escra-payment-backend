package services

import (
	"time"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/pkg/jwt"
)

type AuthService struct {
	userService *UserService
	walletService *WalletService
	jwtService  *jwt.JWTService
}

func NewAuthService(userService *UserService, walletService *WalletService, jwtService *jwt.JWTService) *AuthService {
	return &AuthService{
		userService: userService,
		walletService : walletService,
		jwtService:  jwtService,
	}
}

func (s *AuthService) Register(req *models.RegisterRequest) (*models.AuthResponse, error) {
	user, err := s.userService.CreateUser(req)
	if err != nil {
		return nil, err
	}

	// Create wallet for user
	_, err = s.walletService.CreateWallet(user.ID, "NGN")
	if err != nil {
		return nil, err
	}

	//Generate tokens
	accessToken, err := s.jwtService.GenerateToken(user.ID.String(), user.Phone, 15*time.Minute)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtService.GenerateToken(user.ID.String(), user.Phone, 168*time.Hour)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		User: models.UserResponse{
			ID:        user.ID.String(),
			Phone:     user.Phone,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
			Status:    user.Status,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
func (s *AuthService) Login(req *models.LoginRequest) (*models.AuthResponse, error) {
	// Validate user
	user, err := s.userService.ValidateUser(req.Phone, req.Pin)
	if err != nil {
		return nil, err
	}

	// Generate tokens
	accessToken, err := s.jwtService.GenerateToken(user.ID.String(), user.Phone, 15*time.Minute)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtService.GenerateToken(user.ID.String(), user.Phone, 168*time.Hour)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		User: models.UserResponse{
			ID:        user.ID.String(),
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Phone:     user.Phone,
			Status:    user.Status,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
