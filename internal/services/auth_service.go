package services

import (
	"time"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/pkg/jwt"
	"gorm.io/gorm"
)

type AuthService struct {
	userService             *UserService
	walletService           *WalletService
	transactionLimitService *TransactionLimitService
	jwtService              *jwt.JWTService
}

func NewAuthService(userService *UserService, walletService *WalletService, limitService *TransactionLimitService, jwtService *jwt.JWTService) *AuthService {
	return &AuthService{
		userService:             userService,
		walletService:           walletService,
		transactionLimitService: limitService,
		jwtService:              jwtService,
	}
}

func (s *AuthService) Register(req *models.RegisterRequest) (*models.AuthResponse, error) {
	var user *models.User

	err := s.userService.db.Transaction(func(tx *gorm.DB) error {
		var err error
		user, err = s.userService.createUser(tx, req)
		if err != nil {
			return err
		}

		// Create wallet for user
		if _, err = s.walletService.createWallet(tx, user.ID, "NGN"); err != nil {
			return err
		}

		// Create default transaction limits
		if err = s.transactionLimitService.createDefaultLimit(tx, user.ID); err != nil {
			return err
		}
		return nil
	})
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
			Email:     userEmail(user),
			Status:    user.Status,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
func (s *AuthService) Login(req *models.LoginRequest) (*models.AuthResponse, error) {
	// Validate user
	user, err := s.userService.ValidateUser(req.Phone, req.Password)
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
			Email:     userEmail(user),
			Status:    user.Status,
		},
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func userEmail(user *models.User) string {
	if user.Email == nil {
		return ""
	}
	return *user.Email
}
