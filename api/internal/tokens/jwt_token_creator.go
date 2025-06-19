package tokens

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const minSecretKeySize = 32

type JWTTokenCreator struct {
	secretKey string
}

func NewJWTTokenCreator(secretKey string) (TokenCreator, error) {
	if len(secretKey) < minSecretKeySize {
		return nil, fmt.Errorf("secret key must be at least %d characters", minSecretKeySize)
	}
	return &JWTTokenCreator{secretKey}, nil
}

func (jwtT *JWTTokenCreator) CreateToken(email string, userID string, duration time.Duration) (string, error) {
	payload, err := NewPayload(email, userID, duration)
	if err != nil {
		return "", err
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	return jwtToken.SignedString([]byte(jwtT.secretKey))
}

func (jwtT *JWTTokenCreator) VerifyToken(token string) (*Payload, error) {
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(jwtT.secretKey), nil
	}
	jwtToken, err := jwt.ParseWithClaims(token, &Payload{}, keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if !jwtToken.Valid {
		return nil, ErrInvalidToken
	}

	payload, ok := jwtToken.Claims.(*Payload)
	if !ok {
		return nil, ErrInvalidToken
	}
	return payload, nil
}
