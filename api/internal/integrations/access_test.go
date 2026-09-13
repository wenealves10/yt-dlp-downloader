package integrations

import (
	"testing"
)

// A lista vazia libera tudo, e isso é uma decisão — não um descuido. Se alguém
// invertê-la para "vazio bloqueia", toda integração nova nasce sem conseguir
// fazer uma única chamada, e o sintoma (403 em tudo) não aponta para a causa.
func TestListaVaziaAceitaQualquerOrigem(t *testing.T) {
	if !IPAutorizado(nil, "203.0.113.7") {
		t.Error("sem lista configurada, qualquer origem deve ser aceita")
	}
	if !IPAutorizado([]string{}, "203.0.113.7") {
		t.Error("lista vazia deve ser tratada como ausência de restrição")
	}
}

func TestAutorizacaoPorIPExatoEPorFaixa(t *testing.T) {
	permitidos := []string{"203.0.113.10", "198.51.100.0/24", "2001:db8::/32"}

	aceitos := []string{"203.0.113.10", "198.51.100.1", "198.51.100.254", "2001:db8::1"}
	for _, ip := range aceitos {
		if !IPAutorizado(permitidos, ip) {
			t.Errorf("%s está na lista e deveria ser aceito", ip)
		}
	}

	recusados := []string{"203.0.113.11", "198.51.101.1", "8.8.8.8", "2001:db9::1"}
	for _, ip := range recusados {
		if IPAutorizado(permitidos, ip) {
			t.Errorf("%s não está na lista e deveria ser recusado", ip)
		}
	}
}

// Com lista configurada, não saber a origem é motivo para recusar. O contrário
// transformaria um IP ilegível (proxy mal configurado, header estranho) em uma
// forma de contornar a lista inteira.
func TestOrigemDesconhecidaEhRecusadaQuandoHaLista(t *testing.T) {
	if IPAutorizado([]string{"203.0.113.10"}, "") {
		t.Error("origem vazia não pode passar por uma lista configurada")
	}
	if IPAutorizado([]string{"203.0.113.10"}, "nao-e-um-ip") {
		t.Error("origem ilegível não pode passar por uma lista configurada")
	}
}

// Uma entrada inválida que sobrou no banco não pode virar permissão. Ignorá-la
// mantém a lista restritiva; tratá-la como coringa abriria tudo por causa de um
// erro de digitação antigo.
func TestEntradaInvalidaNaListaEhIgnorada(t *testing.T) {
	permitidos := []string{"isto-nao-e-ip", "999.999.999.999", "203.0.113.10"}

	if !IPAutorizado(permitidos, "203.0.113.10") {
		t.Error("a entrada válida da lista deveria continuar funcionando")
	}
	if IPAutorizado(permitidos, "8.8.8.8") {
		t.Error("entrada inválida não pode liberar origens que não estão na lista")
	}
}

func TestNormalizacaoDaListaDeIPs(t *testing.T) {
	limpos, err := NormalizarIPsPermitidos([]string{
		" 203.0.113.10 ", "203.0.113.10", "", "198.51.100.0/24",
	})
	if err != nil {
		t.Fatalf("lista válida foi recusada: %v", err)
	}
	if len(limpos) != 2 {
		t.Errorf("esperava 2 entradas após remover espaços, vazios e duplicata; veio %v", limpos)
	}

	// A validação existe no cadastro porque é o único momento em que há alguém
	// para corrigir o erro. Descoberto no uso, ele só produz bloqueio sem causa
	// aparente.
	if _, err := NormalizarIPsPermitidos([]string{"10.0.0"}); err == nil {
		t.Error("um IP incompleto deveria ser recusado no cadastro")
	}
	if _, err := NormalizarIPsPermitidos([]string{"10.0.0.0/64"}); err == nil {
		t.Error("um CIDR impossível deveria ser recusado no cadastro")
	}

	// Uma lista imensa não é mais segura: ela é percorrida a cada requisição e
	// deixa de ser auditável por quem a mantém.
	acimaDoLimite := make([]string, 0, MaxIPsPermitidos+5)
	for i := 0; i < MaxIPsPermitidos+5; i++ {
		acimaDoLimite = append(acimaDoLimite, formatarIPv4Teste(i))
	}
	if _, err := NormalizarIPsPermitidos(acimaDoLimite); err == nil {
		t.Error("uma lista acima do limite deveria ser recusada")
	}
}

// formatarIPv4Teste gera endereços distintos para o teste de tamanho da lista.
func formatarIPv4Teste(indice int) string {
	return "203.0." + itoaTeste(indice/256) + "." + itoaTeste(indice%256)
}

func itoaTeste(valor int) string {
	if valor == 0 {
		return "0"
	}
	digitos := ""
	for valor > 0 {
		digitos = string(rune('0'+valor%10)) + digitos
		valor /= 10
	}
	return digitos
}
