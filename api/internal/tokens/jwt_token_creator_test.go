package tokens

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

func TestJWTTokenCreator(t *testing.T) {
	tokenCreator, err := NewJWTTokenCreator(utils.RandomString(32))
	require.NoError(t, err)

	email := fmt.Sprintf("%s@zmail.com", utils.RandomOwner())
	userID := utils.RandomString(10) // Not used in JWTTokenCreator, but can be added if needed
	duration := time.Minute

	issuedAt := time.Now()
	expiresAt := issuedAt.Add(duration)

	token, err := tokenCreator.CreateToken(email, userID, duration)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	payload, err := tokenCreator.VerifyToken(token)
	require.NoError(t, err)
	require.NotNil(t, payload)

	require.Equal(t, email, payload.Email)
	require.NotZero(t, payload.ID)
	require.WithinDuration(t, issuedAt, payload.IssuedAt, time.Second)
	require.WithinDuration(t, expiresAt, payload.ExpiredAt, time.Second)
}

func TestExpiredJWTTokenCreator(t *testing.T) {
	tokenCreator, err := NewJWTTokenCreator(utils.RandomString(32))
	require.NoError(t, err)

	token, err := tokenCreator.CreateToken(utils.RandomOwner(), utils.RandomString(10), -time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	payload, err := tokenCreator.VerifyToken(token)
	require.Error(t, err)
	require.EqualError(t, err, ErrExpiredToken.Error())
	require.Nil(t, payload)
}

func TestInvalidJWTTokenCreatorAlgNone(t *testing.T) {
	payload, err := NewPayload(utils.RandomOwner(), utils.RandomString(10), time.Minute)
	require.NoError(t, err)

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"id":         payload.ID,
		"email":      payload.Email,
		"user_id":    payload.UserID,
		"issued_at":  payload.IssuedAt,
		"expired_at": payload.ExpiredAt,
	})
	token, err := jwtToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	tokenCreator, err := NewJWTTokenCreator(utils.RandomString(32))
	require.NoError(t, err)

	payload, err = tokenCreator.VerifyToken(token)
	require.Error(t, err)
	require.EqualError(t, err, ErrInvalidToken.Error())
	require.Nil(t, payload)
}
