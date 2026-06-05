package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/utils"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) CreateUser(req *models.RegisterRequest) (*models.User, error) {
	return s.createUser(s.db, req)
}

func (s *UserService) createUser(tx *gorm.DB, req *models.RegisterRequest) (*models.User, error) {
	var existingUser models.User
	if err := tx.Where("phone = ?", req.Phone).First(&existingUser).Error; err == nil {
		return nil, errors.New("Phone number already registered")
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	hashedPin, err := utils.HashPassword(req.Pin)
	if err != nil {
		return nil, err
	}

	var email *string
	if req.Email != "" {
		email = &req.Email
	}

	user := &models.User{
		Phone:     req.Phone,
		Password:  hashedPassword,
		PinHash:   hashedPin,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     email,
		Status:    "ACTIVE",
	}

	if err := tx.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) GetUserByPhone(phone string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("phone = ?", phone).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}
func (s *UserService) GetUserByWallet(walletID string) (*models.User, error) {
	// First, find the wallet by its ID
	var wallet models.Wallet
	if err := s.db.Where("id = ?", walletID).First(&wallet).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("wallet not found")
		}
		return nil, err
	}
	if wallet.UserID == nil {
		return nil, errors.New("wallet is not owned by a user")
	}

	// Then, find the user by the wallet's UserID and preload the wallet
	var user models.User
	if err := s.db.Where("id = ?", *wallet.UserID).Preload("Wallet").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserService) GetUserByID(id string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserService) ValidateUser(phone, password string) (*models.User, error) {
	user, err := s.GetUserByPhone(phone)
	if err != nil {
		return nil, err
	}

	// Check account lock
	if user.FailedLoginAttempts >= 5 {
		lockoutDuration := 15 * time.Minute
		if user.LastFailedLoginAt != nil {
			timeSinceLastFail := time.Since(*user.LastFailedLoginAt)
			if timeSinceLastFail < lockoutDuration {
				return nil, errors.New("account locked. try again in 15 minutes")
			}
			// Reset failed attempts after lockout period
			user.FailedLoginAttempts = 0
			s.db.Save(user)
		}
	}

	// Validate password
	if err := utils.CheckPassword(user.Password, password); err != nil {
		// Increment failed attempts
		user.FailedLoginAttempts++
		now := time.Now()
		user.LastFailedLoginAt = &now
		s.db.Save(user)
		return nil, errors.New("invalid credentials")
	}

	// Reset failed attempts on successful login
	if user.FailedLoginAttempts > 0 {
		user.FailedLoginAttempts = 0
		user.LastFailedLoginAt = nil
		s.db.Save(user)
	}

	return user, nil
}
