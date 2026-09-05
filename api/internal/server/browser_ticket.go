package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// O WebSocket do navegador remoto não pode carregar o header Authorization, e
// mandar o token de acesso na query string o deixaria em logs de proxy e no
// histórico do navegador. Em vez disso a API emite um ticket de uso único e
// vida curta, amarrado ao super admin e à conta específica.
const (
	browserTicketTTL    = 60 * time.Second
	browserTicketPrefix = "browser:ticket:"
)

// consumeTicketScript lê e apaga o ticket em uma única operação atômica, de
// modo que duas requisições simultâneas nunca reaproveitem o mesmo ticket.
var consumeTicketScript = redis.NewScript(`
local value = redis.call('GET', KEYS[1])
if value then
  redis.call('DEL', KEYS[1])
end
return value
`)

var errInvalidTicket = errors.New("ticket inválido ou expirado")

// issueBrowserTicket cria o ticket para (usuário, conta).
func (s *Server) issueBrowserTicket(ctx context.Context, userID, accountID uuid.UUID) (string, error) {
	if s.redis == nil {
		return "", errors.New("emissão de ticket indisponível")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("não foi possível gerar o ticket: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)

	value := userID.String() + "|" + accountID.String()
	ok, err := s.redis.SetNX(ctx, browserTicketPrefix+ticket, value, browserTicketTTL).Result()
	if err != nil {
		return "", fmt.Errorf("não foi possível registrar o ticket: %w", err)
	}
	if !ok {
		return "", errors.New("não foi possível registrar o ticket")
	}

	return ticket, nil
}

// consumeBrowserTicket valida e invalida o ticket, devolvendo o usuário que o
// pediu e a conta autorizada.
func (s *Server) consumeBrowserTicket(ctx context.Context, ticket string) (uuid.UUID, uuid.UUID, error) {
	if s.redis == nil || ticket == "" {
		return uuid.Nil, uuid.Nil, errInvalidTicket
	}

	value, err := consumeTicketScript.Run(ctx, s.redis, []string{browserTicketPrefix + ticket}).Text()
	if err != nil || value == "" {
		return uuid.Nil, uuid.Nil, errInvalidTicket
	}

	parts := strings.SplitN(value, "|", 2)
	if len(parts) != 2 {
		return uuid.Nil, uuid.Nil, errInvalidTicket
	}

	userID, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, uuid.Nil, errInvalidTicket
	}
	accountID, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, uuid.Nil, errInvalidTicket
	}

	return userID, accountID, nil
}
