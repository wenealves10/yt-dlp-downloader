package tokens

import (
	"time"
)

type TokenCreator interface {
	CreateToken(username string, duration time.Duration) (string, error)
	VerifyToken(token string) (*Payload, error)
}
