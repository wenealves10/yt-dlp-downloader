package integrations

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// MaxIPsPermitidos limita o tamanho da lista. Uma lista imensa não é mais
// segura: ela é percorrida a cada requisição e, de tão grande, deixa de ser
// auditável por quem a mantém.
const MaxIPsPermitidos = 50

// IPAutorizado diz se a origem da requisição está na lista da integração.
//
// A lista VAZIA libera qualquer origem, e essa escolha merece justificativa:
// tratar vazio como "bloqueia tudo" faria toda integração nascer inutilizável,
// e o operador aprenderia a contornar isso cadastrando 0.0.0.0/0 — que é a
// mesma permissão, só mais difícil de notar no painel.
//
// Aceita endereço exato ou CIDR, IPv4 e IPv6.
func IPAutorizado(permitidos []string, ipCliente string) bool {
	if len(permitidos) == 0 {
		return true
	}

	ip := net.ParseIP(strings.TrimSpace(ipCliente))
	if ip == nil {
		// Sem saber de onde vem, não há como afirmar que está autorizado. Com
		// uma lista configurada, a resposta segura é não.
		return false
	}

	for _, entrada := range permitidos {
		entrada = strings.TrimSpace(entrada)
		if entrada == "" {
			continue
		}

		if strings.Contains(entrada, "/") {
			_, rede, err := net.ParseCIDR(entrada)
			if err != nil {
				// Uma entrada inválida é ignorada em vez de liberar tudo: ela
				// não deveria ter passado pela validação do cadastro, e se
				// passou, o erro não pode virar permissão.
				continue
			}
			if rede.Contains(ip) {
				return true
			}
			continue
		}

		if permitido := net.ParseIP(entrada); permitido != nil && permitido.Equal(ip) {
			return true
		}
	}
	return false
}

// NormalizarIPsPermitidos valida e limpa a lista vinda do painel.
//
// A validação acontece no cadastro, e não no uso, porque é o único momento em
// que existe alguém para corrigir o erro: uma entrada malformada descoberta na
// hora da requisição só produz um bloqueio inexplicável.
func NormalizarIPsPermitidos(entradas []string) ([]string, error) {
	limpos := make([]string, 0, len(entradas))
	vistos := make(map[string]bool, len(entradas))

	for _, entrada := range entradas {
		entrada = strings.TrimSpace(entrada)
		if entrada == "" {
			continue
		}

		if strings.Contains(entrada, "/") {
			_, rede, err := net.ParseCIDR(entrada)
			if err != nil {
				return nil, fmt.Errorf("%q não é um CIDR válido", entrada)
			}
			entrada = rede.String()
		} else {
			ip := net.ParseIP(entrada)
			if ip == nil {
				return nil, fmt.Errorf("%q não é um IP válido", entrada)
			}
			entrada = ip.String()
		}

		if vistos[entrada] {
			continue
		}
		vistos[entrada] = true
		limpos = append(limpos, entrada)
	}

	if len(limpos) > MaxIPsPermitidos {
		return nil, errors.New("a lista de IPs autorizados é longa demais")
	}
	return limpos, nil
}
