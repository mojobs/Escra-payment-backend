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
	var existingUser models.User
	if err:= s.db.Where("phone = ?", req.Phone).First(&existingUser).Error; err == nil {
		return nil, errors.New("Phone number already registered")
	}

	hashedPin, err := utils.HashPassword(req.Pin)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Phone : req.Phone,
        Password: req.Password,
		PinHash: hashedPin,
		FirstName: req.FirstName,
		LastName: req.LastName,
		Email: req.Email,
		Status: "ACTIVE",
	}

	if err := s.db.Create(user).Error; err != nil {
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
func (s *UserService) GetUserByWallet(id string) (*models.User, error) {
    var user models.User
    if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
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

func (s *UserService) ValidateUser(phone, pin string) (*models.User, error) {
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

    // Validate PIN
    if err := utils.CheckPassword(user.PinHash, pin); err != nil {
        // Increment failed attempts
        user.FailedLoginAttempts++
        now := time.Now()
        user.LastFailedLoginAt = &now
        s.db.Save(user)
        return nil, errors.New("invalid pin")
    }

    // Reset failed attempts on successful login
    if user.FailedLoginAttempts > 0 {
        user.FailedLoginAttempts = 0
        user.LastFailedLoginAt = nil
        s.db.Save(user)
    }

    return user, nil
}