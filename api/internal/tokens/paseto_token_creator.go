package tokens

import (
	"fmt"
	"time"

	"github.com/aead/chacha20poly1305"
	"github.com/o1egl/paseto"
)

type PasetoTokenCreator struct {
	paseto       *paseto.V2
	symmetricKey []byte
}

func NewPasetoTokenCreator(symmetricKey string) (TokenCreator, error) {
	if len(symmetricKey) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid key size, must be %d characters", chacha20poly1305.KeySize)
	}

	pasetoTokenCreator := &PasetoTokenCreator{
		paseto:       paseto.NewV2(),
		symmetricKey: []byte(symmetricKey),
	}
	return pasetoTokenCreator, nil
}

func (pasetoT *PasetoTokenCreator) CreateToken(email string, userID string, duration time.Duration) (string, error) {
	payload, err := NewPayload(email, userID, duration)
	if err != nil {
		return "", err
	}
	return pasetoT.paseto.Encrypt(pasetoT.symmetricKey, payload, nil)
}

func (pasetoT *PasetoTokenCreator) VerifyToken(token string) (*Payload, error) {
	payload := &Payload{}
	err := pasetoT.paseto.Decrypt(token, pasetoT.symmetricKey, payload, nil)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if err := payload.Valid(); err != nil {
		return nil, err
	}

	return payload, nil
}
