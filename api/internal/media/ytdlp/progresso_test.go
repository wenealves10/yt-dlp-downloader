package ytdlp

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

// amostra simula uma linha do progress-template já interpretada.
func amostra(baixado, total int64) media.Progress {
	return media.Progress{DownloadedBytes: baixado, TotalBytes: total}
}

// O bug relatado: vídeo e áudio são faixas separadas, o yt-dlp reinicia a
// contagem na segunda, e a barra voltava para zero depois de chegar ao fim.
func TestAgregadorNaoVoltaAoTrocarDeFaixa(t *testing.T) {
	// 200 MB de vídeo + 5 MB de áudio.
	agregador := novoAgregador(205 << 20)

	primeiro := agregador.aplicar(amostra(100<<20, 200<<20), "137", "downloading")
	require.InDelta(t, 48.8, primeiro.Percent, 1.0)

	fimDoVideo := agregador.aplicar(amostra(200<<20, 200<<20), "137", "finished")
	require.InDelta(t, 97.6, fimDoVideo.Percent, 1.0)
	require.False(t, fimDoVideo.Postprocess,
		"ainda falta o áudio: marcar pós-processamento aqui é o que fazia a barra parecer concluída")

	// A faixa de áudio começa do zero — e é exatamente aqui que a barra caía.
	inicioDoAudio := agregador.aplicar(amostra(0, 5<<20), "140", "downloading")
	require.GreaterOrEqual(t, inicioDoAudio.Percent, fimDoVideo.Percent,
		"o percentual não pode retroceder quando começa a segunda faixa")
	require.Equal(t, int64(200<<20), inicioDoAudio.DownloadedBytes,
		"os bytes da faixa concluída continuam contando")

	fim := agregador.aplicar(amostra(5<<20, 5<<20), "140", "finished")
	require.InDelta(t, tetoDownload, fim.Percent, 0.01)
	require.True(t, fim.Postprocess, "agora sim: baixou tudo, começou a junção")
}

// Sem o id do formato (template antigo, ou plataforma que não informa), a
// contagem voltando para trás é o único sinal de troca de faixa.
func TestAgregadorDetectaTrocaSemIDDeFaixa(t *testing.T) {
	agregador := novoAgregador(150)

	agregador.aplicar(amostra(100, 100), "", "downloading")
	depois := agregador.aplicar(amostra(10, 50), "", "downloading")

	require.Equal(t, int64(110), depois.DownloadedBytes)
	require.Equal(t, int64(150), depois.TotalBytes)
}

// A barra nunca chega a 100% durante o download: os 100% são do desfecho do
// job, depois da junção das faixas e da conversão.
func TestAgregadorNaoAtinge100DuranteODownload(t *testing.T) {
	agregador := novoAgregador(100)

	final := agregador.aplicar(amostra(100, 100), "137", "downloading")

	require.InDelta(t, tetoDownload, final.Percent, 0.01)
	require.Less(t, final.Percent, 100.0)
}

// Estimativa curta é comum: o yt-dlp calcula por taxa de bits. O denominador
// cresce, e sem a trava o percentual cairia no meio do download.
func TestAgregadorSobreviveAEstimativaCurta(t *testing.T) {
	agregador := novoAgregador(100)

	antes := agregador.aplicar(amostra(90, 100), "137", "downloading")
	depois := agregador.aplicar(amostra(95, 300), "137", "downloading")

	require.GreaterOrEqual(t, depois.Percent, antes.Percent)
	require.Equal(t, int64(300), depois.TotalBytes, "o total observado corrige a estimativa")
}

// Sem tamanho esperado o agregador ainda funciona; só descobre o denominador
// conforme as faixas aparecem.
func TestAgregadorSemTamanhoEsperado(t *testing.T) {
	agregador := novoAgregador(0)

	meio := agregador.aplicar(amostra(50, 100), "137", "downloading")
	require.InDelta(t, 50.0, meio.Percent, 0.01)

	// Sem saber o total do conteúdo, o fim de uma faixa é o melhor palpite de
	// que o pós-processamento começou.
	fim := agregador.aplicar(amostra(100, 100), "137", "finished")
	require.True(t, fim.Postprocess)

	// E mesmo assim a segunda faixa não derruba a barra.
	segunda := agregador.aplicar(amostra(0, 20), "140", "downloading")
	require.GreaterOrEqual(t, segunda.Percent, meio.Percent)
}

// Velocidade e ETA são da faixa atual: repassá-los é correto, agregá-los não
// faria sentido.
func TestAgregadorRepassaVelocidadeEEta(t *testing.T) {
	agregador := novoAgregador(1000)

	saida := agregador.aplicar(media.Progress{
		DownloadedBytes: 100, TotalBytes: 1000, SpeedBPS: 4096, ETASeconds: 30,
	}, "137", "downloading")

	require.Equal(t, int64(4096), saida.SpeedBPS)
	require.Equal(t, 30, saida.ETASeconds)
}

// Uma única faixa (áudio puro, ou um MP4 progressivo) é o caso simples e não
// pode regredir por causa da lógica de troca.
func TestAgregadorFaixaUnica(t *testing.T) {
	agregador := novoAgregador(1000)

	var anterior float64
	for _, baixado := range []int64{100, 250, 500, 750, 1000} {
		atual := agregador.aplicar(amostra(baixado, 1000), "140", "downloading")
		require.GreaterOrEqual(t, atual.Percent, anterior)
		anterior = atual.Percent
	}
	require.InDelta(t, tetoDownload, anterior, 0.01)
}

// Trace real capturado do yt-dlp baixando um vídeo do YouTube em 360p: faixa
// de vídeo (formato 396) e depois a de áudio (251), cada uma contando do zero.
// É a sequência exata que fazia a barra ir a 100% e voltar.
func TestAgregadorComTraceRealDoYtDlp(t *testing.T) {
	linhas := []string{
		"downloading|1024|15298808|NA|79385|192|396",
		"downloading|8387584|15298808|NA|14097479|0|396",
		"downloading|12537348|15298808|NA|6061248|0|396",
		"finished|15298808|15298808|NA|9375503|NA|396",
		"downloading|523264|10202210|NA|2474399|3|251",
		"downloading|5242880|10202210|NA|9000000|1|251",
		"finished|10202210|10202210|NA|11186157|NA|251",
	}

	// O que a API grava em total_bytes na criação: vídeo + a faixa de áudio.
	agregador := novoAgregador(15298808 + 10202210)

	var anterior float64
	var vistos []float64
	for _, linha := range linhas {
		bruto, faixa, status, ok := interpretarProgresso(linha)
		require.True(t, ok, linha)

		atual := agregador.aplicar(bruto, faixa, status)
		require.GreaterOrEqual(t, atual.Percent, anterior,
			"a barra retrocedeu em %q: %.1f%% depois de %.1f%%", linha, atual.Percent, anterior)
		anterior = atual.Percent
		vistos = append(vistos, atual.Percent)
	}

	// No fim da faixa de vídeo a barra está a pouco menos de 60%: é aqui que a
	// versão antiga anunciava 100% e depois recomeçava do zero.
	require.InDelta(t, 60.0, vistos[3], 2.0)
	// E o fim de verdade encosta no teto, sem nunca ter passado dele.
	require.InDelta(t, tetoDownload, vistos[len(vistos)-1], 0.01)
}
