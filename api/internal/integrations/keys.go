// Package integrations concentra o que é próprio do consumo da plataforma por
// OUTRO SISTEMA: as chaves que autenticam, o controle de abuso por chave, os
// eventos enviados por webhook e a assinatura que prova que o POST veio de nós.
//
// Nada aqui conhece gin nem o banco: são as regras, isoladas de onde são
// aplicadas. É o que permite testá-las sem subir API, Postgres nem Redis.
package integrations

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// KeyPrefix identifica de longe o que é uma chave desta API. Serve para
	// duas coisas práticas: um segredo colado no lugar errado é reconhecível
	// (dá para varrer repositório e log procurando por ele), e o suporte
	// distingue na hora uma chave nossa de um token de outro sistema.
	KeyPrefix = "adk_live_"

	// 32 bytes = 256 bits de entropia vinda de crypto/rand. É o que dispensa
	// qualquer proteção contra força bruta no lado do servidor: não existe
	// dicionário nem tempo suficiente para adivinhar.
	keyRandomBytes = 32

	// Quantos caracteres do corpo da chave entram na parte visível guardada no
	// banco. Bastam para o operador casar "a chave adk_live_7Gk2mQ…" com a
	// linha do painel, e são insuficientes para reconstruir o restante.
	prefixVisibleChars = 6
)

// NewKey é o resultado da emissão. O Plaintext existe apenas neste valor, em
// memória, no caminho da resposta que o devolve: nada do que vai para o banco
// permite recuperá-lo depois.
type NewKey struct {
	Plaintext string
	Hash      string
	Prefix    string
	LastFour  string
}

// GenerateKey emite uma chave nova.
//
// O erro não é decorativo: se crypto/rand falhar, a alternativa seria gerar uma
// chave previsível, e uma chave previsível é pior do que nenhuma chave.
func GenerateKey() (NewKey, error) {
	bruto := make([]byte, keyRandomBytes)
	if _, err := rand.Read(bruto); err != nil {
		return NewKey{}, fmt.Errorf("não foi possível gerar entropia para a chave: %w", err)
	}

	corpo := base64.RawURLEncoding.EncodeToString(bruto)
	chave := KeyPrefix + corpo

	return NewKey{
		Plaintext: chave,
		Hash:      HashKey(chave),
		Prefix:    KeyPrefix + corpo[:prefixVisibleChars],
		LastFour:  corpo[len(corpo)-4:],
	}, nil
}

// HashKey devolve o que o banco guarda no lugar da chave.
//
// SHA-256 e não bcrypt/argon2, ao contrário da senha de usuário, e a razão é a
// origem do segredo: a chave é gerada por nós com 256 bits aleatórios, não
// escolhida por uma pessoa. Não há senha fraca para proteger nem dicionário a
// atrasar — e um hash deliberadamente lento entraria no caminho de CADA
// requisição autenticada, transformando a defesa em gargalo.
func HashKey(chave string) string {
	soma := sha256.Sum256([]byte(chave))
	return hex.EncodeToString(soma[:])
}

// LooksLikeKey diz se a string tem a forma de uma chave nossa.
//
// É só um filtro de forma, aplicado ANTES de tocar o banco: pede-se o hash de
// qualquer coisa que chegue no header, e sem esta checagem cada requisição com
// um token de outro sistema (ou um Bearer de usuário colado por engano) viraria
// uma consulta ao Postgres. Não é, e nunca deve ser tratado como, validação.
func LooksLikeKey(valor string) bool {
	if !strings.HasPrefix(valor, KeyPrefix) {
		return false
	}
	corpo := valor[len(KeyPrefix):]
	if len(corpo) != base64.RawURLEncoding.EncodedLen(keyRandomBytes) {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(corpo)
	return err == nil
}

// MaskKey formata uma chave para aparecer em tela ou log sem ser exposta.
func MaskKey(prefixo, ultimos string) string {
	return prefixo + "…" + ultimos
}
