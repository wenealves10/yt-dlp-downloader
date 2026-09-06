package browser

import "testing"

// Cada plataforma precisa saber onde abrir o login e o que exportar. Um perfil
// pela metade só apareceria como bug em produção, na hora do cadastro.
func TestPerfisEstaoCompletos(t *testing.T) {
	for _, perfil := range PlataformasSuportadas() {
		if perfil.ID == "" || perfil.Label == "" {
			t.Errorf("perfil sem identificação: %+v", perfil)
		}
		if perfil.LoginURL == "" {
			t.Errorf("[%s] sem tela de login: o navegador remoto não teria onde abrir", perfil.ID)
		}
		if len(perfil.CookieDomains) == 0 {
			t.Errorf("[%s] sem domínios de cookie: a sessão sairia vazia", perfil.ID)
		}
		if len(perfil.CookiesDeSessao) == 0 {
			t.Errorf("[%s] sem cookies de sessão: nada distinguiria logado de deslogado", perfil.ID)
		}
	}
}

// Verificação por HTTP exige as duas peças. Com CheckURL e sem a marca, o
// health check leria qualquer resposta como "não autenticada" e derrubaria a
// conta.
func TestVerificacaoPorHTTPExigeUrlEMarca(t *testing.T) {
	for _, perfil := range PlataformasSuportadas() {
		if perfil.CheckURL != "" && perfil.MarcaAutenticada == "" {
			t.Errorf("[%s] tem CheckURL sem marca: toda sessão viraria inválida", perfil.ID)
		}
		if perfil.MarcaAutenticada != "" && perfil.CheckURL == "" {
			t.Errorf("[%s] tem marca sem CheckURL: não há o que verificar", perfil.ID)
		}
	}
}

// Um domínio de cookie de uma plataforma não pode aparecer em outra: é o que
// impede a sessão do Vimeo levar junto os cookies do Google.
func TestDominiosNaoSeSobrepoemEntrePlataformas(t *testing.T) {
	dono := make(map[string]string)

	for _, perfil := range PlataformasSuportadas() {
		for _, dominio := range perfil.CookieDomains {
			// Instagram e Facebook compartilham o login de propósito: sem o
			// domínio do Facebook a sessão do Instagram sai pela metade.
			if dominio == "facebook.com" {
				continue
			}
			if anterior, existe := dono[dominio]; existe {
				t.Errorf("domínio %q em %s e %s", dominio, anterior, perfil.ID)
			}
			dono[dominio] = perfil.ID
		}
	}
}

// Plataforma desconhecida cai no padrão, e não em um perfil vazio: um perfil
// zerado exportaria cookie nenhum e a conta pareceria sempre deslogada.
func TestPerfilDesconhecidoCaiNoPadrao(t *testing.T) {
	perfil := PerfilDe("plataforma-que-nao-existe")

	if perfil.ID != PlataformaPadrao {
		t.Fatalf("esperado %q, obtido %q", PlataformaPadrao, perfil.ID)
	}
	if PlataformaSuportada("plataforma-que-nao-existe") {
		t.Error("uma plataforma sem perfil não pode ser aceita no cadastro")
	}
}

// O YouTube não paga o custo de buscar cookies na resolução: ele resolve bem
// anônimo, e subir um Chrome headless em todo link colado seria uma regressão.
func TestYoutubeNaoExigeSessaoParaMetadados(t *testing.T) {
	if PerfilDe("youtube").MetadadosExigemSessao {
		t.Error("resolver YouTube não deveria exigir sessão")
	}
	if !PerfilDe("vimeo").MetadadosExigemSessao {
		t.Error("o Vimeo recusa a leitura de metadados sem sessão")
	}
}

// A ordem da lista é estável: o padrão primeiro, o resto alfabético. Sem isso a
// tela embaralharia os itens do seletor a cada requisição.
func TestOrdemDasPlataformasEhEstavel(t *testing.T) {
	primeira := PlataformasSuportadas()
	for i := 0; i < 5; i++ {
		atual := PlataformasSuportadas()
		for j := range atual {
			if atual[j].ID != primeira[j].ID {
				t.Fatalf("ordem instável na posição %d: %q vs %q", j, atual[j].ID, primeira[j].ID)
			}
		}
	}
	if primeira[0].ID != PlataformaPadrao {
		t.Errorf("a plataforma padrão deveria vir primeiro, veio %q", primeira[0].ID)
	}
}
