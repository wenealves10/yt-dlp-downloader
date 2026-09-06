package ytdlp

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// marcaProgresso prefixa as linhas de progresso para separá-las do resto da
// saída. O yt-dlp mistura tudo no stdout; um prefixo próprio evita parsing
// frágil de mensagens dele.
const marcaProgresso = "@@PROGRESSO@@"

// marcaArquivo prefixa o caminho final que o próprio yt-dlp informa. Adivinhar
// o arquivo varrendo o diretório erra quando ele baixa vídeo e áudio em faixas
// separadas: sobram os intermediários `base.f137.mp4` e `base.f140.m4a` ao lado
// do resultado, e o maior nem sempre é o final.
const marcaArquivo = "@@ARQUIVO@@"

// formatoIDValido restringe o seletor de formato ao vocabulário do yt-dlp:
// dígitos, letras, hífen, ponto, `+` (junção de faixas) e `/` (alternativa).
// É o que impede um "format_id" vindo do cliente de virar outra opção de linha
// de comando.
var formatoIDValido = regexp.MustCompile(`^[A-Za-z0-9_.+/-]{1,120}$`)

// nomeArquivoValido restringe o nome base do arquivo. Ele é montado pelo
// servidor, mas validar aqui fecha a porta para path traversal caso um dia
// venha de outro lugar.
var nomeArquivoValido = regexp.MustCompile(`^[A-Za-z0-9_-]{1,120}$`)

// Download executa o download e devolve o arquivo produzido.
func (p *Provider) Download(
	ctx context.Context,
	req media.Request,
	onProgress media.ProgressFunc,
) (*media.Result, error) {
	if !nomeArquivoValido.MatchString(req.Filename) {
		return nil, &media.Error{Kind: media.ErrDownloadFailed, Detail: "nome de arquivo inválido"}
	}
	if req.FormatID != "" && !formatoIDValido.MatchString(req.FormatID) {
		return nil, &media.Error{Kind: media.ErrFormatUnavailable, Detail: "seletor de formato inválido"}
	}

	// O diretório de saída é do servidor, mas o caminho é confirmado antes de
	// virar argumento: o processo só escreve onde mandamos.
	saidaAbs, err := filepath.Abs(req.OutputDir)
	if err != nil {
		return nil, &media.Error{Kind: media.ErrDownloadFailed, Detail: "diretório de saída inválido"}
	}
	if err := os.MkdirAll(saidaAbs, 0o700); err != nil {
		return nil, &media.Error{Kind: media.ErrDownloadFailed, Detail: err.Error()}
	}

	inicio := time.Now()

	// Extração à parte quando há proxy de resolução: ela sai por ele, e os
	// bytes da mídia não.
	infoJSON := p.extrairInfoSeparada(ctx, req, saidaAbs)

	args := p.argsArquivo(req, saidaAbs, infoJSON)

	// O agregador é quem vê o download como um todo; o yt-dlp só sabe falar de
	// uma faixa por vez.
	agregador := novoAgregador(req.ExpectedBytes)

	var informado string
	stderr, err := p.download.executar(ctx, args, func(linha string) {
		switch {
		case strings.HasPrefix(linha, marcaArquivo):
			// A última ocorrência é a que vale: com faixas separadas o yt-dlp
			// imprime uma por faixa antes do arquivo unido.
			informado = strings.TrimSpace(strings.TrimPrefix(linha, marcaArquivo))
		case strings.HasPrefix(linha, marcaProgresso):
			bruto, faixa, status, ok := interpretarProgresso(strings.TrimPrefix(linha, marcaProgresso))
			if ok && onProgress != nil {
				onProgress(agregador.aplicar(bruto, faixa, status))
			}
		}
	})
	if err != nil {
		return nil, p.explicarBloqueio(classificar(err, stderr), plataformaDaURL(req.URL))
	}

	caminho, tamanho, err := arquivoFinal(saidaAbs, req.Filename, informado)
	if err != nil {
		return nil, &media.Error{Kind: media.ErrDownloadFailed, Detail: err.Error()}
	}

	return &media.Result{
		FilePath:  caminho,
		SizeBytes: tamanho,
		Ext:       strings.TrimPrefix(filepath.Ext(caminho), "."),
		Duration:  time.Since(inicio),
	}, nil
}

// argsArquivo monta os argumentos do download. Tudo estruturado: nenhuma parte
// vem de concatenação com entrada do usuário.
//
// Quando infoJSON está preenchido, o yt-dlp NÃO volta à plataforma para extrair
// nada: ele lê a extração pronta do arquivo e vai direto aos bytes da mídia. É
// isso que permite a extração sair por um proxy e o download não.
func (p *Provider) argsArquivo(req media.Request, saidaAbs, infoJSON string) []string {
	args := p.argsBase(plataformaDaURL(req.URL), faseMidia)

	// --newline força uma linha por atualização; sem isso o yt-dlp reescreve a
	// mesma linha com \r e o scanner nunca entrega nada.
	args = append(args,
		"--newline",
		"--progress",
		"--progress-template",
		// O id do formato no fim é o que identifica a FAIXA: com vídeo e áudio
		// separados, é ele que diz que uma acabou e outra começou. Sem esse
		// campo só restava adivinhar pela contagem voltando a zero.
		marcaProgresso+"%(progress.status)s|%(progress.downloaded_bytes)s|%(progress.total_bytes)s|%(progress.total_bytes_estimate)s|%(progress.speed)s|%(progress.eta)s|%(info.format_id)s",
		// "after_move" é obrigatório: sem o momento, --print dispara ANTES do
		// download e imprime um caminho vazio. --no-simulate é igualmente
		// obrigatório, porque --print sozinho implica simulação e nada seria
		// baixado.
		"--print", "after_move:"+marcaArquivo+"%(filepath)s",
		"--no-simulate",
		"-o", filepath.Join(saidaAbs, req.Filename+".%(ext)s"),
	)

	// --ffmpeg-location espera um CAMINHO (do binário ou do diretório). Passar
	// o nome "ffmpeg" faz o yt-dlp avisar que não existe e seguir SEM ele — e
	// aí todo vídeo que precisa juntar vídeo e áudio sai como faixa solta, sem
	// erro nenhum. Com o nome puro, deixamos ele procurar no PATH.
	if strings.ContainsRune(p.cfg.FFmpegBinary, os.PathSeparator) {
		args = append(args, "--ffmpeg-location", p.cfg.FFmpegBinary)
	}

	if req.CookieFile != "" {
		args = append(args, "--cookies", req.CookieFile)
	}

	switch req.Kind {
	case media.KindAudio:
		args = append(args, "-x", "--audio-format", "mp3", "--audio-quality", "0")
		args = append(args, "-f", seletorAudio(req.FormatID))
	default:
		args = append(args, "-f", seletorVideo(req.FormatID, req.MaxHeight))
		args = append(args, "--merge-output-format", "mp4")
	}

	if infoJSON != "" {
		// Com a extração já em mãos, a URL não entra: passá-la faria o yt-dlp
		// ir à plataforma de novo, que é justamente o que se quer evitar.
		return append(args, "--load-info-json", infoJSON)
	}

	// O "--" encerra as opções: o que vier depois é tratado como URL, mesmo que
	// comece com hífen.
	return append(args, "--", req.URL)
}

// nomeInfoJSON é onde a extração é gravada dentro do diretório do download.
// Fica junto do resto e some com ele na limpeza.
const nomeInfoJSON = "extracao.info.json"

// extrairInfoSeparada faz a extração de metadados em um processo próprio, para
// que ela possa sair por um caminho de rede diferente do download.
//
// Devolve o caminho do arquivo, ou "" para o download seguir do jeito normal —
// sem proxy de resolução configurado, ou numa plataforma que não precisa dele,
// não há motivo para pagar um processo a mais.
//
// Falhar aqui NÃO derruba o download: ele cai no caminho de uma fase só, que é
// o que sempre foi. Um proxy mal configurado degrada, não quebra.
func (p *Provider) extrairInfoSeparada(ctx context.Context, req media.Request, saidaAbs string) string {
	plataforma := plataformaDaURL(req.URL)
	if p.cfg.ResolveProxyURL == "" || !plataformasQueRecusamDatacenter[plataforma] {
		return ""
	}

	args := p.argsBase(plataforma, faseExtracao)
	args = append(args, "--dump-single-json", "--no-playlist", "--skip-download")
	if req.CookieFile != "" {
		args = append(args, "--cookies", req.CookieFile)
	}
	args = append(args, "--", req.URL)

	var bruto strings.Builder
	stderr, err := p.runner.executar(ctx, args, func(linha string) {
		bruto.WriteString(linha)
	})
	if err != nil {
		log.Printf("ytdlp: extração pelo proxy falhou (%s); seguindo sem ela: %s",
			plataforma, resumirStderr(stderr))
		return ""
	}

	caminho := filepath.Join(saidaAbs, nomeInfoJSON)
	if err := os.WriteFile(caminho, []byte(bruto.String()), 0o600); err != nil {
		log.Printf("ytdlp: não foi possível gravar a extração: %v", err)
		return ""
	}
	return caminho
}

// plataformaDaURL identifica a plataforma da URL já normalizada. Um erro aqui
// não impede o download: só faz o provider tratá-la como desconhecida, o que é
// o comportamento mais conservador.
func plataformaDaURL(bruta string) media.Platform {
	alvo, err := url.Parse(bruta)
	if err != nil {
		return media.PlatformUnknown
	}
	return media.PlatformFor(alvo)
}

// seletorVideo monta a cadeia de alternativas do `-f`.
//
// Ela existe porque os ids do yt-dlp NÃO são estáveis entre duas extrações do
// mesmo conteúdo: em HLS eles carregam o CDN sorteado na hora, e o id que a
// tela ofereceu pode não existir quando o worker vai baixar. Antes isso
// terminava em "o formato escolhido não está disponível" — um erro sobre a
// escolha do usuário para um problema que não era dele.
//
// A ordem vai do mais específico ao mais genérico, e o yt-dlp para na primeira
// que casar:
//
//  1. o formato pedido somado à melhor faixa de áudio (o formato pode ser mudo);
//  2. o formato pedido sozinho (quando ele já traz áudio);
//  3. a mesma resolução por outro caminho, se o id sumiu;
//  4. qualquer coisa, para não voltar de mãos vazias.
func seletorVideo(formatoID string, alturaMax int) string {
	var alternativas []string

	if formatoID != "" {
		alternativas = append(alternativas, formatoID+"+bestaudio", formatoID)
	}

	if alturaMax > 0 {
		limite := strconv.Itoa(alturaMax)
		alternativas = append(alternativas,
			"bestvideo[height<="+limite+"]+bestaudio",
			"best[height<="+limite+"]")
	}

	// Sem preferência nenhuma, o MP4 primeiro: é o que toca em qualquer lugar.
	alternativas = append(alternativas,
		"bestvideo[ext=mp4]+bestaudio[ext=m4a]", "best[ext=mp4]",
		"bestvideo+bestaudio", "best")

	return strings.Join(alternativas, "/")
}

// seletorAudio segue a mesma ideia: o id pedido pode ter deixado de existir, e
// aí qualquer faixa de áudio é melhor do que falhar.
func seletorAudio(formatoID string) string {
	if formatoID == "" {
		return "bestaudio/best"
	}
	return formatoID + "/bestaudio/best"
}

// interpretarProgresso lê a linha do progress-template. Campos ausentes chegam
// como "NA"; o yt-dlp não conhece o tamanho total de todo conteúdo.
//
// O que sai daqui é o progresso de UMA FAIXA, cru como o yt-dlp reporta. Quem
// junta as faixas num progresso único é o agregador — misturar as duas coisas
// aqui foi o que fez a barra reiniciar do zero no meio do download.
func interpretarProgresso(linha string) (progresso media.Progress, faixa, status string, ok bool) {
	campos := strings.Split(linha, "|")
	if len(campos) < 6 {
		return media.Progress{}, "", "", false
	}

	status = campos[0]
	baixado := paraInt(campos[1])
	total := paraInt(campos[2])
	if total == 0 {
		total = paraInt(campos[3]) // estimativa, quando o exato não vem
	}
	if len(campos) >= 7 {
		faixa = idDeFaixa(campos[6])
	}

	progresso = media.Progress{
		DownloadedBytes: baixado,
		TotalBytes:      total,
		SpeedBPS:        paraInt(campos[4]),
		ETASeconds:      int(paraInt(campos[5])),
	}

	if total > 0 {
		progresso.Percent = float64(baixado) / float64(total) * 100
		if progresso.Percent > 100 {
			progresso.Percent = 100
		}
	}

	return progresso, faixa, status, true
}

// idDeFaixa descarta os marcadores de "não sei" do yt-dlp. Tratá-los como um id
// de verdade faria toda amostra parecer uma faixa nova.
func idDeFaixa(valor string) string {
	valor = strings.TrimSpace(valor)
	if valor == "NA" || valor == "None" || valor == "-" {
		return ""
	}
	return valor
}

// paraInt aceita inteiro, decimal e os "NA"/"None" que o yt-dlp emite quando
// não sabe o valor.
func paraInt(valor string) int64 {
	valor = strings.TrimSpace(valor)
	if valor == "" || valor == "NA" || valor == "None" || valor == "-" {
		return 0
	}
	if inteiro, err := strconv.ParseInt(valor, 10, 64); err == nil {
		return inteiro
	}
	if decimal, err := strconv.ParseFloat(valor, 64); err == nil && decimal > 0 {
		return int64(decimal)
	}
	return 0
}

// arquivoFinal resolve o arquivo produzido. Prefere o caminho que o yt-dlp
// informou; a varredura é a rede de proteção para o caso de ele não imprimir.
func arquivoFinal(dir, base, informado string) (string, int64, error) {
	if informado != "" {
		// O caminho vem do processo filho, mas precisa estar dentro do
		// diretório que nós criamos: é o que impede um nome inesperado de
		// apontar para fora.
		absoluto := informado
		if !filepath.IsAbs(absoluto) {
			absoluto = filepath.Join(dir, absoluto)
		}
		limpo := filepath.Clean(absoluto)
		if strings.HasPrefix(limpo, dir+string(os.PathSeparator)) {
			if info, err := os.Stat(limpo); err == nil && !info.IsDir() && info.Size() > 0 {
				return limpo, info.Size(), nil
			}
		}
	}
	return arquivoProduzido(dir, base)
}

// arquivoProduzido varre o diretório procurando o resultado. Os intermediários
// de faixa (`base.f137.mp4`) são descartados: com vídeo e áudio separados eles
// convivem com o arquivo final, e o maior deles nem sempre é o certo — num
// vídeo curto a faixa de áudio pode passar a de vídeo.
func arquivoProduzido(dir, base string) (string, int64, error) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return "", 0, err
	}

	var melhor string
	var maior int64

	for _, entrada := range entradas {
		nome := entrada.Name()
		if entrada.IsDir() || !strings.HasPrefix(nome, base+".") {
			continue
		}
		if strings.HasSuffix(nome, ".part") || strings.HasSuffix(nome, ".ytdl") ||
			strings.HasSuffix(nome, ".temp") {
			continue
		}
		if intermediarioDeFaixa(nome, base) {
			continue
		}

		info, err := entrada.Info()
		if err != nil {
			continue
		}
		if info.Size() > maior {
			maior, melhor = info.Size(), filepath.Join(dir, nome)
		}
	}

	if melhor == "" {
		return "", 0, fmt.Errorf("nenhum arquivo produzido para %q", base)
	}
	return melhor, maior, nil
}

// intermediarioDeFaixa reconhece `base.f137.mp4`: o segmento do meio é o id do
// formato prefixado por "f".
func intermediarioDeFaixa(nome, base string) bool {
	resto := strings.TrimPrefix(nome, base+".")
	partes := strings.Split(resto, ".")
	if len(partes) < 2 {
		return false
	}
	meio := partes[0]
	if !strings.HasPrefix(meio, "f") || len(meio) < 2 {
		return false
	}
	for _, r := range meio[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
