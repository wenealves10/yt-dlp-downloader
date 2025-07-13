package server

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

type createUserRequest struct {
	FullName       string `json:"full_name" binding:"required"`
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required,min=6"`
	TurnstileToken string `json:"turnstileToken" binding:"required"`
}

type userResponse struct {
	ID                uuid.UUID  `json:"id"`
	FullName          string     `json:"full_name"`
	Email             string     `json:"email"`
	PasswordChangedAt *time.Time `json:"password_changed_at"`
	PhotoUrl          string     `json:"photo_url,omitempty"`
	Plan              string     `json:"plan"`
	DailyLimit        int32      `json:"daily_limit"`
	IsVerified        bool       `json:"is_verified"`
	CreatedAt         *time.Time `json:"created_at"`
}

func newUserResponse(user db.User) userResponse {
	return userResponse{
		ID:                user.ID,
		PhotoUrl:          user.PhotoUrl.String,
		Plan:              string(user.Plan),
		DailyLimit:        user.DailyLimit,
		IsVerified:        user.IsVerified,
		FullName:          user.FullName,
		Email:             user.Email,
		PasswordChangedAt: user.PasswordChangedAt,
		CreatedAt:         user.CreatedAt,
	}
}

func (s *Server) register(ctx *gin.Context) {
	var req createUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	ip := ctx.ClientIP()
	if !s.validateTurnstile(req.TurnstileToken, ip) {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("invalid Turnstile token")))
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		if bcrypt.ErrPasswordTooLong == err {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	arg := db.CreateUserParams{
		ID:                utils.GenerateUUID(),
		Plan:              db.CorePlanTypeFree,
		DailyLimit:        2,
		Active:            true,
		IsVerified:        true,
		PasswordChangedAt: utils.GetCurrentTime(),
		FullName:          req.FullName,
		Email:             req.Email,
		HashedPassword:    hashedPassword,
	}

	user, err := s.store.CreateUser(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New(db.HandleDBError(err))))
		return
	}

	rsp := newUserResponse(user)
	ctx.JSON(http.StatusOK, rsp)
}

type loginUserRequest struct {
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required,min=6"`
	TurnstileToken string `json:"turnstileToken" binding:"required"`
}

type loginUserResponse struct {
	AccessToken string       `json:"access_token"`
	User        userResponse `json:"user"`
}

func (s *Server) login(ctx *gin.Context) {
	var req loginUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	ip := ctx.ClientIP()
	if !s.validateTurnstile(req.TurnstileToken, ip) {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("invalid Turnstile token")))
		return
	}

	user, err := s.store.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("user not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	err = utils.CheckPassword(req.Password, user.HashedPassword)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, errorResponse(err))
		return
	}

	accessToken, err := s.tokenCreator.CreateToken(user.Email, user.ID.String(), s.config.AccessTokenDuration)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// Update last login time
	if err := s.store.UpdateUserLoginInfo(ctx.Request.Context(), user.ID); err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	rsp := loginUserResponse{
		AccessToken: accessToken,
		User:        newUserResponse(user),
	}

	ctx.JSON(http.StatusOK, rsp)
}

func (s *Server) getProfile(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*tokens.Payload)
	userID, err := utils.ParseUUID(authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid user ID")))
		return
	}
	user, err := s.store.GetUserByID(ctx.Request.Context(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("user not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	rsp := convertUserToResponse(user)
	ctx.JSON(http.StatusOK, rsp)
}

type updateUserProfileRequest struct {
	FullName string                `form:"full_name" binding:"omitempty,min=3"`
	Photo    *multipart.FileHeader `form:"photo,omitempty"`
}

func (s *Server) updateProfile(ctx *gin.Context) {
	var req updateUserProfileRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Photo != nil {
		if !utils.ValidateImage(req.Photo) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid image file")))
			return
		}
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*tokens.Payload)
	userID, err := utils.ParseUUID(authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid user ID")))
		return
	}

	user, err := s.store.GetUserByID(ctx.Request.Context(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("user not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	var fullName *string
	if req.FullName != "" && user.FullName != req.FullName {
		fullName = &req.FullName
	}

	var profilePathDest *string
	if req.Photo != nil {
		if user.PhotoUrl.String != "" {
			if err := s.storage.DeleteFile(ctx.Request.Context(), user.PhotoUrl.String); err != nil {
				log.Println("failed to delete old profile photo:", err)
			}
		}

		filePathDest := fmt.Sprintf("uploads/profile/%s/photo_%s.jpg", user.ID.String(), utils.GenerateUUID().String())
		fileBytes, err := getFileBytes(req.Photo)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("failed to read file bytes")))
			return
		}
		if err := s.storage.UploadFileByte(ctx.Request.Context(), fileBytes, filePathDest); err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("failed to upload profile photo")))
			return
		}
		profilePathDest = &filePathDest
	}

	if err := s.store.UpdateUser(ctx.Request.Context(), db.UpdateUserParams{
		ID:       user.ID,
		FullName: db.ToPgText(fullName),
		PhotoUrl: db.ToPgText(profilePathDest),
	}); err != nil {
		log.Printf("Failed to update user profile: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("failed to update user profile")))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

type updatePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required,min=6"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

func (s *Server) updatePassword(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*tokens.Payload)
	userID, err := utils.ParseUUID(authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid user ID")))
		return
	}

	var req updatePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	user, err := s.store.GetUserByID(ctx.Request.Context(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("user not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	if err := utils.CheckPassword(req.CurrentPassword, user.HashedPassword); err != nil {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("current password is incorrect")))
		return
	}

	hashedNewPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		if bcrypt.ErrPasswordTooLong == err {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("new password is too long")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("failed to hash new password")))
		return
	}

	if err := s.store.UpdateUser(ctx.Request.Context(), db.UpdateUserParams{
		ID:                user.ID,
		HashedPassword:    db.ToPgText(&hashedNewPassword),
		PasswordChangedAt: db.ToPgTimestamptz(utils.GetCurrentTime()),
	}); err != nil {
		log.Printf("Failed to update user password: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("failed to update user password")))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

func convertUserToResponse(user db.User) userResponse {
	return userResponse{
		ID:                user.ID,
		FullName:          user.FullName,
		Email:             user.Email,
		PhotoUrl:          user.PhotoUrl.String,
		Plan:              string(user.Plan),
		DailyLimit:        user.DailyLimit,
		IsVerified:        user.IsVerified,
		PasswordChangedAt: user.PasswordChangedAt,
		CreatedAt:         user.CreatedAt,
	}
}
