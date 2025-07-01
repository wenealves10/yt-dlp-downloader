package server

import (
	"database/sql"
	"errors"
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
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
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
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
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

	user, err := s.store.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
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
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	rsp := convertUserToResponse(user)
	ctx.JSON(http.StatusOK, rsp)
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
