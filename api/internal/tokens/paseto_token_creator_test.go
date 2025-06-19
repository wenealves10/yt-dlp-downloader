package tokens

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

func TestPasetoTokenCreator(t *testing.T) {
	tokenCreator, err := NewPasetoTokenCreator(utils.RandomString(32))
	require.NoError(t, err)

	email := fmt.Sprintf("%s@zmail.com", utils.RandomOwner())
	userID := utils.RandomString(10)
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

func TestExpiredPasetoTokenCreator(t *testing.T) {
	email := fmt.Sprintf("%s@zmail.com", utils.RandomOwner())
	userID := utils.RandomString(10)
	tokenCreator, err := NewPasetoTokenCreator(utils.RandomString(32))
	require.NoError(t, err)

	token, err := tokenCreator.CreateToken(email, userID, -time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	payload, err := tokenCreator.VerifyToken(token)
	require.Error(t, err)
	require.EqualError(t, err, ErrExpiredToken.Error())
	require.Nil(t, payload)
}
