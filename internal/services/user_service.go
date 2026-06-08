package services

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/utils"
	"github.com/mojobs/lara-payment-backend.git/pkg/money"
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
		Role:      models.NormalizeUserRole(req.Role),
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

func (s *UserService) GetProfile(id string) (*models.UserProfileResponse, error) {
	user, err := s.GetUserByID(id)
	if err != nil {
		return nil, err
	}

	metrics, err := s.profileMetrics(user)
	if err != nil {
		return nil, err
	}

	return &models.UserProfileResponse{
		ID:        user.ID.String(),
		Phone:     user.Phone,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     userEmail(user),
		Role:      models.NormalizeUserRole(user.Role),
		Status:    user.Status,
		KYCStatus: firstNonEmpty(user.KYCStatus, "UNVERIFIED"),
		Metrics:   metrics,
		MerchantDetails: models.MerchantDetailsResponse{
			BusinessName: user.BusinessName,
			BusinessType: user.BusinessType,
			RCNumber:     user.RCNumber,
			Website:      user.Website,
			Address:      user.BusinessAddress,
			City:         user.BusinessCity,
			Country:      user.BusinessCountry,
			SupportPhone: user.BusinessSupportPhone,
		},
	}, nil
}

func (s *UserService) UpdateMerchantDetails(userID string, req *models.UpdateMerchantDetailsRequest) error {
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return errors.New("invalid user ID")
	}

	var user models.User
	if err := s.db.Where("id = ?", parsedUserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	updates := map[string]interface{}{
		"business_name":          strings.TrimSpace(req.BusinessName),
		"business_type":          strings.TrimSpace(req.BusinessType),
		"rc_number":              strings.TrimSpace(req.RCNumber),
		"website":                strings.TrimSpace(req.Website),
		"business_address":       strings.TrimSpace(req.Address),
		"business_city":          strings.TrimSpace(req.City),
		"business_country":       strings.TrimSpace(firstNonEmpty(req.Country, "Nigeria")),
		"business_support_phone": strings.TrimSpace(req.SupportPhone),
	}

	return s.db.Model(&user).Updates(updates).Error
}

func (s *UserService) profileMetrics(user *models.User) (models.UserProfileMetrics, error) {
	role := models.NormalizeUserRole(user.Role)
	var completedOrders int64
	var disputedOrders int64
	var salesVolume int64

	completedQuery := s.db.Model(&models.EscrowOrder{}).Where("status = ?", "RELEASED")
	disputeQuery := s.db.Model(&models.EscrowDispute{}).
		Joins("JOIN escrow_orders ON escrow_orders.id = escrow_disputes.escrow_id")

	if role == "seller" {
		completedQuery = completedQuery.Where("seller_id = ?", user.ID)
		disputeQuery = disputeQuery.Where("escrow_orders.seller_id = ?", user.ID)
		if err := s.db.Model(&models.EscrowOrder{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("seller_id = ? AND status = ?", user.ID, "RELEASED").
			Scan(&salesVolume).Error; err != nil {
			return models.UserProfileMetrics{}, err
		}
	} else {
		completedQuery = completedQuery.Where("buyer_id = ?", user.ID)
		disputeQuery = disputeQuery.Where("escrow_orders.buyer_id = ?", user.ID)
	}

	if err := completedQuery.Count(&completedOrders).Error; err != nil {
		return models.UserProfileMetrics{}, err
	}
	if err := disputeQuery.Count(&disputedOrders).Error; err != nil {
		return models.UserProfileMetrics{}, err
	}

	return models.UserProfileMetrics{
		CompletedOrders: completedOrders,
		TrustScore:      trustScoreFromActivity(user.TrustScore, completedOrders, disputedOrders),
		SalesVolume:     money.FromMinorUnits(salesVolume),
		Rating:          user.Rating,
	}, nil
}

func trustScoreFromActivity(storedScore int, completedOrders, disputedOrders int64) int {
	total := completedOrders + disputedOrders
	if total == 0 {
		return clampScore(storedScore)
	}
	score := int((completedOrders * 100) / total)
	return clampScore(score)
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
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

	if !strings.EqualFold(user.Status, "ACTIVE") {
		return nil, errors.New("account is not active")
	}

	// Reset failed attempts on successful login
	if user.FailedLoginAttempts > 0 {
		user.FailedLoginAttempts = 0
		user.LastFailedLoginAt = nil
		s.db.Save(user)
	}

	return user, nil
}
