package ytdlp

import "github.com/wenealves10/yt-dlp-downloader/internal/media"

// tetoDownload é onde a barra para enquanto ainda há processo rodando. Os 100%
// pertencem ao desfecho do job, não ao fim do download bruto: depois dele ainda
// vem a junção das faixas e a conversão, que podem levar minutos.
const tetoDownload = 99.0

// agregadorProgresso transforma o progresso por FAIXA que o yt-dlp emite em
// progresso do download inteiro.
//
// Acima de 720p o YouTube (e a maioria das plataformas) serve vídeo e áudio
// separados, e o yt-dlp baixa um de cada vez reiniciando a contagem: a barra
// subia até o fim, voltava para zero e subia de novo. Ninguém consegue ler isso
// como "está progredindo" — parece que o download recomeçou do nada.
//
// A saída daqui é monotônica por construção: o numerador só cresce, e o
// percentual nunca é menor que o maior já reportado.
type agregadorProgresso struct {
	// esperado é o tamanho total estimado do conteúdo, somando as faixas que
	// serão baixadas. Zero significa desconhecido — aí o denominador é
	// descoberto conforme as faixas aparecem, e a barra anda mais devagar no
	// começo em vez de mentir.
	esperado int64

	concluido    int64 // bytes das faixas que já terminaram
	atualBaixado int64
	atualTotal   int64
	faixaAtual   string
	temFaixa     bool
	maiorPercent float64
}

func novoAgregador(esperado int64) *agregadorProgresso {
	if esperado < 0 {
		esperado = 0
	}
	return &agregadorProgresso{esperado: esperado}
}

// aplicar recebe uma amostra bruta e devolve o progresso do download inteiro.
func (a *agregadorProgresso) aplicar(bruto media.Progress, faixa string, status string) media.Progress {
	a.trocarFaixaSePreciso(bruto, faixa)

	a.atualBaixado = bruto.DownloadedBytes
	if bruto.TotalBytes > 0 {
		a.atualTotal = bruto.TotalBytes
	}

	baixado := a.concluido + a.atualBaixado
	total := a.denominador(baixado)

	agregado := media.Progress{
		DownloadedBytes: baixado,
		TotalBytes:      total,
		SpeedBPS:        bruto.SpeedBPS,
		ETASeconds:      bruto.ETASeconds,
	}

	if total > 0 {
		agregado.Percent = float64(baixado) / float64(total) * 100
	}
	if agregado.Percent > tetoDownload {
		agregado.Percent = tetoDownload
	}
	// A trava para trás é o que sobrevive a uma estimativa curta: quando o
	// denominador cresce no meio do caminho, o percentual recalculado cairia.
	if agregado.Percent < a.maiorPercent {
		agregado.Percent = a.maiorPercent
	}
	a.maiorPercent = agregado.Percent

	// "finished" no download bruto é o fim de UMA faixa. Só vira "finalizando"
	// quando não se espera mais nada: com o total estimado em mãos dá para
	// saber; sem ele, o fim de qualquer faixa é o melhor palpite disponível.
	if status == "finished" {
		agregado.Postprocess = a.esperado <= 0 || baixado >= a.esperado
	}

	return agregado
}

// trocarFaixaSePreciso fecha a faixa anterior quando o yt-dlp começa a próxima.
func (a *agregadorProgresso) trocarFaixaSePreciso(bruto media.Progress, faixa string) {
	novaFaixa := false
	switch {
	case faixa != "":
		// O id do formato é o sinal explícito, e o único confiável quando duas
		// faixas por acaso têm o mesmo tamanho.
		novaFaixa = a.temFaixa && faixa != a.faixaAtual
	default:
		// Sem o id, a contagem voltando para trás é o que resta: o yt-dlp nunca
		// decresce dentro de uma mesma faixa.
		novaFaixa = a.temFaixa && bruto.DownloadedBytes < a.atualBaixado
	}

	if novaFaixa {
		a.concluido += maiorInt64(a.atualTotal, a.atualBaixado)
		a.atualBaixado, a.atualTotal = 0, 0
	}

	a.faixaAtual = faixa
	a.temFaixa = true
}

// denominador escolhe o total mais confiável disponível. Nunca fica abaixo do
// que já foi baixado: um percentual acima de 100 seria pior que uma estimativa
// curta.
func (a *agregadorProgresso) denominador(baixado int64) int64 {
	total := a.esperado
	if observado := a.concluido + a.atualTotal; observado > total {
		total = observado
	}
	if baixado > total {
		total = baixado
	}
	return total
}

func maiorInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
