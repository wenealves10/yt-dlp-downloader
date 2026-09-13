package integrations

import (
	"strings"
	"testing"
)

// A chave em claro não pode ser derivável do que fica guardado. Se um dia
// alguém "otimizar" o armazenamento guardando o valor em vez do hash, é este
// teste que acusa.
func TestChaveGuardadaNaoContemOSegredo(t *testing.T) {
	chave, err := GenerateKey()
	if err != nil {
		t.Fatalf("geração falhou: %v", err)
	}

	if strings.Contains(chave.Hash, chave.Plaintext) {
		t.Error("o hash guardado não pode conter a chave em claro")
	}
	if chave.Hash == chave.Plaintext {
		t.Error("o hash é igual à chave: nada foi hasheado")
	}
	// O prefixo é exibido no painel; ele revela o começo, e só o começo.
	if len(chave.Prefix) >= len(chave.Plaintext) {
		t.Errorf("o prefixo visível (%d) não pode cobrir a chave inteira (%d)",
			len(chave.Prefix), len(chave.Plaintext))
	}
	if !strings.HasPrefix(chave.Plaintext, chave.Prefix) {
		t.Error("o prefixo guardado deveria ser o começo da chave, para identificá-la no painel")
	}
	if !strings.HasSuffix(chave.Plaintext, chave.LastFour) {
		t.Error("os últimos quatro caracteres deveriam ser os da chave")
	}
}

// Duas chaves emitidas em sequência têm de ser diferentes. Um gerador
// determinístico (um rand mal semeado, por exemplo) daria a mesma chave a duas
// integrações, e uma leria os downloads da outra.
func TestChavesEmitidasSaoDistintas(t *testing.T) {
	vistas := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		chave, err := GenerateKey()
		if err != nil {
			t.Fatalf("geração falhou na iteração %d: %v", i, err)
		}
		if vistas[chave.Plaintext] {
			t.Fatal("duas chaves idênticas foram emitidas")
		}
		vistas[chave.Plaintext] = true
	}
}

// O hash é o que a autenticação procura no índice. Se ele não fosse estável, a
// chave entregue ao cliente deixaria de funcionar na requisição seguinte.
func TestHashEhEstavel(t *testing.T) {
	const chave = KeyPrefix + "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	if HashKey(chave) != HashKey(chave) {
		t.Error("o mesmo valor produziu hashes diferentes")
	}
	if HashKey(chave) == HashKey(chave+"x") {
		t.Error("valores diferentes produziram o mesmo hash")
	}
	if len(HashKey(chave)) != 64 {
		t.Errorf("esperava 64 caracteres hexadecimais de SHA-256, veio %d", len(HashKey(chave)))
	}
}

// LooksLikeKey é o filtro que evita uma consulta ao banco por requisição
// inválida. Ele precisa aceitar o que emitimos e recusar o resto — em
// particular um token de usuário colado no lugar errado, que é o erro mais
// comum de quem está integrando.
func TestFormatoDeChave(t *testing.T) {
	valida, err := GenerateKey()
	if err != nil {
		t.Fatalf("geração falhou: %v", err)
	}

	if !LooksLikeKey(valida.Plaintext) {
		t.Error("uma chave que acabamos de emitir deveria passar pelo filtro de forma")
	}

	recusadas := map[string]string{
		"vazio":                    "",
		"sem prefixo":              "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
		"prefixo errado":           "adk_test_abcdefghijklmnopqrstuvwxyz0123456789ABC",
		"curta demais":             KeyPrefix + "abc",
		"longa demais":             valida.Plaintext + "aaaa",
		"token paseto de usuário":  "v2.local.abcdefghijklmnop",
		"caractere fora do base64": KeyPrefix + strings.Repeat("$", 43),
	}
	for nome, valor := range recusadas {
		if LooksLikeKey(valor) {
			t.Errorf("%s deveria ser recusado pelo filtro de forma: %q", nome, valor)
		}
	}
}
