package server

import (
	"testing"

	"github.com/wenealves10/yt-dlp-downloader/internal/media"
)

func formatosDeTeste() []media.Format {
	return []media.Format{
		{ID: "137", Kind: media.KindVideo, Height: 1080, SizeBytes: 200, VideoCodec: "avc1", AudioCodec: "none"},
		{ID: "18", Kind: media.KindVideo, Height: 360, SizeBytes: 60, VideoCodec: "avc1", AudioCodec: "mp4a"},
		{ID: "140", Kind: media.KindAudio, SizeBytes: 5, AudioCodec: "mp4a"},
	}
}

// O caso que travava a barra: 1080p vem sem áudio, e o download soma a melhor
// faixa de áudio. Guardar só os 200 dava um denominador curto.
func TestTamanhoEstimadoSomaAudioEmFormatoSemSom(t *testing.T) {
	formatos := formatosDeTeste()

	total := tamanhoEstimado(formatos, formatos[0], media.KindVideo)

	if total != 205 {
		t.Fatalf("esperado 205 (vídeo + áudio), obtido %d", total)
	}
}

// Formato progressivo já traz áudio embutido: somar de novo inflaria o total e
// a barra ficaria eternamente atrás.
func TestTamanhoEstimadoNaoSomaEmFormatoProgressivo(t *testing.T) {
	formatos := formatosDeTeste()

	total := tamanhoEstimado(formatos, formatos[1], media.KindVideo)

	if total != 60 {
		t.Fatalf("esperado 60, obtido %d", total)
	}
}

// Pedido de áudio baixa uma faixa só.
func TestTamanhoEstimadoAudioUsaApenasAFaixa(t *testing.T) {
	formatos := formatosDeTeste()

	total := tamanhoEstimado(formatos, formatos[2], media.KindAudio)

	if total != 5 {
		t.Fatalf("esperado 5, obtido %d", total)
	}
}

// Sem tamanho informado não há o que estimar: zero significa "descubra durante
// o download", e inventar um número seria pior.
func TestTamanhoEstimadoSemTamanhoConhecido(t *testing.T) {
	escolhido := media.Format{ID: "137", Kind: media.KindVideo, AudioCodec: "none"}

	total := tamanhoEstimado(formatosDeTeste(), escolhido, media.KindVideo)

	if total != 0 {
		t.Fatalf("esperado 0, obtido %d", total)
	}
}

// Nem toda plataforma expõe uma faixa de áudio separada; aí o vídeo é tudo que
// existe para estimar.
func TestTamanhoEstimadoSemFaixaDeAudioDisponivel(t *testing.T) {
	formatos := []media.Format{
		{ID: "hls-720", Kind: media.KindVideo, SizeBytes: 80, AudioCodec: "none"},
	}

	total := tamanhoEstimado(formatos, formatos[0], media.KindVideo)

	if total != 80 {
		t.Fatalf("esperado 80, obtido %d", total)
	}
}
