package utils

import (
	"crypto/rand"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func CheckPassword(password, hashedPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// senhaAlfabeto evita caracteres ambíguos (0/O, 1/l/I): a senha gerada é lida
// na tela e digitada por uma pessoa, muitas vezes ditada por telefone.
const senhaAlfabeto = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// senhaTamanho dá ~70 bits de entropia com esse alfabeto, folgado para uma
// senha temporária que será trocada pelo usuário.
const senhaTamanho = 12

// GenerateReadablePassword cria a senha temporária que o painel entrega ao
// administrador. Usa crypto/rand: math/rand daria senhas previsíveis a partir
// do horário de criação da conta.
func GenerateReadablePassword() (string, error) {
	limite := big.NewInt(int64(len(senhaAlfabeto)))
	buffer := make([]byte, senhaTamanho)

	for i := range buffer {
		n, err := rand.Int(rand.Reader, limite)
		if err != nil {
			return "", err
		}
		buffer[i] = senhaAlfabeto[n.Int64()]
	}
	return string(buffer), nil
}
