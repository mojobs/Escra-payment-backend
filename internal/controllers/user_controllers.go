package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mojobs/lara-payment-backend.git/internal/models"
	"github.com/mojobs/lara-payment-backend.git/internal/services"
)

const maxKYCDocumentBytes = 10 << 20

type UserController struct {
	userService   *services.UserService
	publicBaseURL string
}

func NewUserController(userService *services.UserService, publicBaseURL ...string) *UserController {
	baseURL := ""
	if len(publicBaseURL) > 0 {
		baseURL = publicBaseURL[0]
	}
	return &UserController{
		userService:   userService,
		publicBaseURL: baseURL,
	}
}

func (ctrl *UserController) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	profile, err := ctrl.userService.GetProfile(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "user_not_found",
			Message: err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, profile)
}

func (ctrl *UserController) UpdateMerchantDetails(c *gin.Context) {
	userID := c.GetString("user_id")

	var req models.UpdateMerchantDetailsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	if err := ctrl.userService.UpdateMerchantDetails(userID, &req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "merchant_profile_update_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{
		Success: true,
		Message: "Merchant profile details updated successfully",
	})
}

func (ctrl *UserController) UploadKYCDocument(c *gin.Context) {
	userID := c.GetString("user_id")
	documentType := models.NormalizeKYCDocumentType(c.PostForm("document_type"))
	if !models.IsValidKYCDocumentType(documentType) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_document_type",
			Message: "document_type must be CAC_CERTIFICATE or PASSPORT",
		})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxKYCDocumentBytes+(1<<20))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "kyc_upload_failed",
			Message: "file is required and must be a valid multipart upload",
		})
		return
	}
	if fileHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "kyc_upload_failed",
			Message: "file is empty",
		})
		return
	}
	if fileHeader.Size > maxKYCDocumentBytes {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "kyc_upload_failed",
			Message: "file must not exceed 10MB",
		})
		return
	}

	extension := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedKYCDocumentExtension(extension) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "kyc_upload_failed",
			Message: "file must be a PDF, JPG, JPEG, or PNG document",
		})
		return
	}

	uploadDir := filepath.Join("uploads", "kyc", "docs")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "kyc_upload_failed",
			Message: "could not prepare upload storage",
		})
		return
	}

	filename := fmt.Sprintf(
		"user_%s_%d_%s%s",
		shortIDForFilename(userID),
		time.Now().UTC().UnixNano(),
		strings.ToLower(documentType),
		extension,
	)
	storagePath := filepath.Join(uploadDir, filename)
	if err := c.SaveUploadedFile(fileHeader, storagePath); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "kyc_upload_failed",
			Message: "could not save uploaded document",
		})
		return
	}

	urlPath := "/" + filepath.ToSlash(storagePath)
	response, err := ctrl.userService.UpdateKYCDocument(userID, documentType, ctrl.publicURL(c, urlPath))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "kyc_upload_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (ctrl *UserController) AdminVerifyUser(c *gin.Context) {
	var req models.AdminVerifyUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_error",
			Message: err.Error(),
		})
		return
	}

	response, err := ctrl.userService.AdminVerifyUser(c.Param("userId"), &req)
	if err != nil {
		status := http.StatusBadRequest
		if strings.EqualFold(err.Error(), "user not found") {
			status = http.StatusNotFound
		}
		c.JSON(status, models.ErrorResponse{
			Error:   "user_verification_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func allowedKYCDocumentExtension(extension string) bool {
	switch extension {
	case ".pdf", ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}

func shortIDForFilename(id string) string {
	cleaned := strings.ReplaceAll(id, "-", "")
	if len(cleaned) > 8 {
		return cleaned[:8]
	}
	if cleaned == "" {
		return "user"
	}
	return cleaned
}

func (ctrl *UserController) publicURL(c *gin.Context, path string) string {
	baseURL := strings.TrimRight(ctrl.publicBaseURL, "/")
	if baseURL == "" {
		scheme := "http"
		if forwardedProto := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0]); forwardedProto != "" {
			scheme = forwardedProto
		} else if c.Request.TLS != nil {
			scheme = "https"
		}

		host := c.Request.Host
		if forwardedHost := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Host"), ",")[0]); forwardedHost != "" {
			host = forwardedHost
		}
		baseURL = scheme + "://" + host
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}
