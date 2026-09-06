package ytdlp

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// alvoImpersonacao é o navegador que o yt-dlp imita. "chrome" resolve para a
// versão mais recente que o curl_cffi instalado suporta, o que evita fixar uma
// versão que some numa atualização.
const alvoImpersonacao = "chrome"

// plataformasQueExigemImpersonacao são as que recusam um cliente HTTP comum
// mesmo com sessão autenticada.
//
// O bloqueio delas é por fingerprint de TLS, e não por conta: o Reddit responde
// `HTTP Error 403: Blocked` já na PRIMEIRA requisição, antes de olhar cookie
// nenhum — por isso enviar a sessão não resolvia sozinho. Imitar o handshake de
// um navegador real é o que faz a requisição parecer o que de fato é.
//
// É uma lista, e não um padrão global: o YouTube funciona bem sem isso, e
// impersonar onde não é preciso só adiciona uma dependência ao caminho crítico.
var plataformasQueExigemImpersonacao = map[media.Platform]bool{
	media.PlatformReddit:      true,
	media.PlatformTwitter:     true,
	media.PlatformPinterest:   true,
	media.PlatformVimeo:       true,
	media.PlatformInstagram:   true,
	media.PlatformFacebook:    true,
	media.PlatformTikTok:      true,
	media.PlatformDailymotion: true,
}

// deteccaoImpersonacao guarda o resultado da sondagem. Ela custa um processo, e
// a resposta não muda enquanto o container viver.
//
// O estado é POR PROVIDER, e não do pacote: dois providers podem apontar para
// binários diferentes, e um cache global faria o primeiro a sondar decidir pelos
// outros — além de deixar testes dependentes da ordem de execução.
type deteccaoImpersonacao struct {
	uma        sync.Once
	disponivel bool
}

// suportaImpersonacao sonda o yt-dlp uma única vez por provider.
//
// A sondagem é obrigatória porque `--impersonate` com alvo indisponível é ERRO
// FATAL, não aviso: sem ela, uma imagem construída sem o extra `curl-cffi`
// quebraria todo download das plataformas da lista — inclusive os que hoje
// funcionam sem impersonação nenhuma.
func (p *Provider) suportaImpersonacao() bool {
	p.impersonacao.uma.Do(func() {
		ctx, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelar()

		saida, err := exec.CommandContext(ctx, p.cfg.Binary, "--list-impersonate-targets").Output()
		if err != nil {
			log.Printf("ytdlp: impersonação indisponível (%v); plataformas que a exigem podem ser recusadas", err)
			return
		}
		// A tabela sempre traz um cabeçalho; o que importa é haver alvo de
		// Chrome listado abaixo dele.
		p.impersonacao.disponivel = strings.Contains(strings.ToLower(string(saida)), "chrome")
		if !p.impersonacao.disponivel {
			log.Printf("ytdlp: nenhum alvo de impersonação disponível; instale o extra curl-cffi do yt-dlp")
		}
	})
	return p.impersonacao.disponivel
}

// argsImpersonacao devolve os argumentos de impersonação para a plataforma, ou
// nada quando ela não precisa ou o suporte não existe.
func (p *Provider) argsImpersonacao(plataforma media.Platform) []string {
	if !plataformasQueExigemImpersonacao[plataforma] || !p.suportaImpersonacao() {
		return nil
	}
	return []string{"--impersonate", alvoImpersonacao}
}

// usaImpersonacao diz se o User-Agent configurado deve ser suprimido.
//
// Imitando um navegador, o yt-dlp já envia o User-Agent correspondente. Enviar
// o nosso por cima cria um cliente com handshake de Chrome e cabeçalho de outra
// coisa — uma incoerência que é, ela mesma, sinal de automação.
func (p *Provider) usaImpersonacao(plataforma media.Platform) bool {
	return len(p.argsImpersonacao(plataforma)) > 0
}
