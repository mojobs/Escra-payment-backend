//go:build cgo
// +build cgo

package services

import (
	"strings"
	"testing"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
)

func TestUserKYCUploadAndAdminVerification(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)

	user, err := userService.CreateUser(&models.RegisterRequest{
		Phone:     "08000000931",
		Password:  "password1",
		FirstName: "Test",
		LastName:  "Seller",
		Pin:       "1234",
		Email:     "kyc-seller@example.com",
		Role:      "Seller",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.Role != "seller" {
		t.Fatalf("role should be normalized to seller, got %q", user.Role)
	}

	upload, err := userService.UpdateKYCDocument(user.ID.String(), "CAC_CERTIFICATE", "http://localhost:8080/uploads/kyc/docs/cac.pdf")
	if err != nil {
		t.Fatalf("update kyc document: %v", err)
	}
	if !upload.Success || upload.Status != "UNDER_REVIEW" {
		t.Fatalf("unexpected upload response: %+v", upload)
	}

	profile, err := userService.GetProfile(user.ID.String())
	if err != nil {
		t.Fatalf("get profile after upload: %v", err)
	}
	if profile.KYCStatus != "UNDER_REVIEW" {
		t.Fatalf("kyc status after upload = %q", profile.KYCStatus)
	}
	if profile.KYCDocumentType != "CAC_CERTIFICATE" {
		t.Fatalf("document type = %q", profile.KYCDocumentType)
	}
	if profile.KYCDocumentURL == "" {
		t.Fatalf("expected document url in profile")
	}

	approved, err := userService.AdminVerifyUser(user.ID.String(), &models.AdminVerifyUserRequest{
		Status: "ACTIVE",
		Reason: "Documents approved successfully",
	})
	if err != nil {
		t.Fatalf("admin approve user: %v", err)
	}
	if approved.Status != "ACTIVE" || approved.KYCStatus != "VERIFIED" {
		t.Fatalf("unexpected approval response: %+v", approved)
	}

	profile, err = userService.GetProfile(user.ID.String())
	if err != nil {
		t.Fatalf("get profile after approval: %v", err)
	}
	if profile.Status != "ACTIVE" {
		t.Fatalf("account status = %q", profile.Status)
	}
	if profile.KYCStatus != "VERIFIED" {
		t.Fatalf("kyc status after approval = %q", profile.KYCStatus)
	}
}

func TestAdminCanRejectKYC(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)

	user, err := userService.CreateUser(&models.RegisterRequest{
		Phone:     "08000000932",
		Password:  "password1",
		FirstName: "Test",
		LastName:  "Buyer",
		Pin:       "1234",
		Email:     "kyc-reject@example.com",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	rejected, err := userService.AdminVerifyUser(user.ID.String(), &models.AdminVerifyUserRequest{
		Status: "REJECTED",
		Reason: "Document image is unclear",
	})
	if err != nil {
		t.Fatalf("admin reject user: %v", err)
	}
	if rejected.Status != "REJECTED" || rejected.KYCStatus != "REJECTED" {
		t.Fatalf("unexpected rejection response: %+v", rejected)
	}

	profile, err := userService.GetProfile(user.ID.String())
	if err != nil {
		t.Fatalf("get profile after rejection: %v", err)
	}
	if profile.KYCStatus != "REJECTED" {
		t.Fatalf("kyc status after rejection = %q", profile.KYCStatus)
	}
	if profile.KYCRejectionReason != "Document image is unclear" {
		t.Fatalf("rejection reason = %q", profile.KYCRejectionReason)
	}
}

func TestCreateUserReturnsFriendlyDuplicateEmail(t *testing.T) {
	db := setupEscrowTestDB(t)
	userService := NewUserService(db)

	_, err := userService.CreateUser(&models.RegisterRequest{
		Phone:     "08000000933",
		Password:  "password1",
		FirstName: "First",
		LastName:  "User",
		Pin:       "1234",
		Email:     "duplicate@example.com",
	})
	if err != nil {
		t.Fatalf("create first user: %v", err)
	}

	_, err = userService.CreateUser(&models.RegisterRequest{
		Phone:     "08000000934",
		Password:  "password1",
		FirstName: "Second",
		LastName:  "User",
		Pin:       "1234",
		Email:     "DUPLICATE@example.com",
	})
	if err == nil {
		t.Fatalf("expected duplicate email error")
	}
	if !strings.Contains(err.Error(), "Email address already registered") {
		t.Fatalf("expected friendly duplicate email error, got %q", err.Error())
	}
}
