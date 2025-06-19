package tokens

import (
	"time"
)

type TokenCreator interface {
	CreateToken(email string, userID string, duration time.Duration) (string, error)
	VerifyToken(token string) (*Payload, error)
}
