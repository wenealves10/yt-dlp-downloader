package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/helpers"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
	"github.com/wenealves10/yt-dlp-downloader/internal/ytaccounts"
)

const (
	// intervaloProgresso limita quanto o progresso trafega. O yt-dlp emite
	// várias atualizações por segundo; repassar todas encheria o Redis e a
	// conexão SSE sem a barra ficar mais suave para quem olha.
	intervaloProgresso = 700 * time.Millisecond

	// intervaloPersistencia é bem maior: o banco só guarda o último estado
	// conhecido, para a tela reabrir no meio de um download em andamento.
	intervaloPersistencia = 10 * time.Second

	// intervaloCancelamento é a frequência com que checamos o pedido de
	// cancelamento. Um download sem progresso (resolvendo, ou pós-processando)
	// não passa pelo callback, então a checagem precisa ser independente dele.
	intervaloCancelamento = 2 * time.Second
)

// JobDownloadMedia baixa qualquer plataforma. Ele não sabe o que é yt-dlp:
// conversa com o registry, que escolhe o provider.
type JobDownloadMedia struct {
	client   *asynq.Client
	store    db.Store
	rdStream stream.EventPublisher
	redis    *redis.Client
	registry *media.Registry
	accounts ytaccounts.Provider
	workDir  string
}

func NewJobDownloadMedia(
	client *asynq.Client,
	store db.Store,
	rdStream stream.EventPublisher,
	redisClient *redis.Client,
	registry *media.Registry,
	accounts ytaccounts.Provider,
	workDir string,
) *JobDownloadMedia {
	if workDir == "" {
		workDir = "./uploads/tmp"
	}
	return &JobDownloadMedia{
		client: client, store: store, rdStream: rdStream,
		redis: redisClient, registry: registry, accounts: accounts, workDir: workDir,
	}
}

func (p *JobDownloadMedia) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload tasks.DownloadMediaPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	downloadID, err := utils.ParseUUID(payload.DownloadID)
	if err != nil {
		return fmt.Errorf("identificador inválido: %v: %w", err, asynq.SkipRetry)
	}

	download, err := p.store.GetDownloadByID(ctx, downloadID)
	if err != nil {
		return fmt.Errorf("download não encontrado: %v: %w", err, asynq.SkipRetry)
	}

	// O usuário pode ter cancelado enquanto a tarefa esperava na fila.
	if download.Status == db.CoreDownloadStatusCANCELED {
		log.Printf("jobs: download já cancelado antes de começar id=%s", downloadID)
		return nil
	}

	if err := p.store.MarkDownloadStarted(ctx, downloadID); err != nil {
		return fmt.Errorf("falha ao marcar início: %v", err)
	}
	p.publicar(ctx, download, db.CoreDownloadStatusPROCESSING, media.Progress{}, "")

	resultado, err := p.baixar(ctx, download)
	if err != nil {
		return p.registrarFalha(ctx, download, err)
	}
	defer os.RemoveAll(filepath.Dir(resultado.FilePath))

	// Última checagem antes de o arquivo sair do diretório temporário. Sem ela,
	// um cancelamento pedido nos segundos finais ainda mandava o arquivo para o
	// storage: o download parava, mas o resultado ficava — exatamente o que o
	// usuário não quer quando cancela. O defer acima apaga o temporário.
	if p.cancelamentoPedido(context.WithoutCancel(ctx), download.ID) {
		log.Printf("jobs: cancelado após concluir o download; arquivo descartado id=%s", download.ID)
		return p.registrarFalha(ctx, download, media.ErrCanceled)
	}

	// A partir daqui o arquivo existe em disco; o envio ao storage segue pelo
	// caminho de upload que já existe.
	return p.encaminharParaUpload(ctx, download, resultado)
}

// baixar cuida do ciclo de vida completo: diretório temporário, sessão da
// conta, progresso, cancelamento e limpeza.
func (p *JobDownloadMedia) baixar(ctx context.Context, download db.Download) (*media.Result, error) {
	provider := p.registry.ProviderByName(download.Provider.String)
	if provider == nil {
		// O provider gravado pode não existir mais (removido em um deploy).
		// Resolver de novo é melhor do que falhar por um nome antigo.
		_, escolhido, _, err := p.registry.Resolve(download.OriginalUrl)
		if err != nil {
			return nil, err
		}
		provider = escolhido
	}

	// MkdirTemp exige que o diretório pai já exista; num container recém-criado
	// ele ainda não existe.
	if err := os.MkdirAll(p.workDir, 0o700); err != nil {
		return nil, fmt.Errorf("não foi possível preparar o diretório de trabalho: %w", err)
	}

	// Um diretório por download: a limpeza vira um RemoveAll e nenhum job pisa
	// no arquivo temporário de outro.
	dir, err := os.MkdirTemp(p.workDir, "dl-")
	if err != nil {
		return nil, fmt.Errorf("não foi possível preparar o diretório de trabalho: %w", err)
	}
	limparEmFalha := true
	defer func() {
		if limparEmFalha {
			os.RemoveAll(dir)
		}
	}()

	kind := media.KindVideo
	if download.Format == db.CoreFormatTypeMP3 {
		kind = media.KindAudio
	}

	// A sessão gerenciada é emprestada só durante o download e o arquivo de
	// cookies é destruído no Release. A conta é a da plataforma DESTE conteúdo:
	// uma sessão do YouTube não autentica no Vimeo.
	lease := ytaccounts.Acquire(ctx, p.accounts, download.Platform)
	defer lease.Release()

	cookieFile := ""
	if lease != nil {
		cookieFile = lease.CookieFile
	}

	// O cancelamento chega por uma chave no Redis, e não pelo asynq: ele
	// precisa funcionar mesmo quando o download não emite progresso, como
	// durante a resolução ou o pós-processamento.
	ctx, cancelar := context.WithCancel(ctx)
	defer cancelar()
	parar := p.observarCancelamento(ctx, download.ID, cancelar)
	defer close(parar)

	inicio := time.Now()
	ultimoEvento := time.Time{}
	ultimaGravacao := time.Time{}

	resultado, err := provider.Download(ctx, media.Request{
		URL:        download.OriginalUrl,
		FormatID:   download.FormatID.String,
		MaxHeight:  int(download.FormatHeight),
		Kind:       kind,
		OutputDir:  dir,
		Filename:   "media_" + download.ID.String(),
		CookieFile: cookieFile,
		// O tamanho estimado na criação já soma vídeo + áudio; é o denominador
		// estável que impede a barra de reiniciar quando o yt-dlp troca de
		// faixa.
		ExpectedBytes: download.TotalBytes,
	}, func(progresso media.Progress) {
		agora := time.Now()
		if agora.Sub(ultimoEvento) >= intervaloProgresso {
			ultimoEvento = agora
			p.publicarProgresso(ctx, download, progresso)
		}
		if agora.Sub(ultimaGravacao) >= intervaloPersistencia {
			ultimaGravacao = agora
			p.gravarProgresso(ctx, download.ID, progresso)
		}
	})
	if err != nil {
		// Sessão recusada pelo YouTube tira a conta do rodízio; a próxima
		// tentativa já escolhe outra.
		ytaccounts.ReportAuthFailure(ctx, p.accounts, lease, []byte(err.Error()))
		return nil, err
	}

	limparEmFalha = false
	log.Printf("jobs: download concluído id=%s plataforma=%s provider=%s tamanho=%d duração=%s",
		download.ID, download.Platform, provider.Name(), resultado.SizeBytes,
		time.Since(inicio).Round(time.Second))

	return resultado, nil
}

// observarCancelamento cancela o contexto quando o usuário pede. Devolve um
// canal que interrompe a observação.
func (p *JobDownloadMedia) observarCancelamento(
	ctx context.Context, downloadID uuid.UUID, cancelar context.CancelFunc,
) chan struct{} {
	parar := make(chan struct{})

	go func() {
		ticker := time.NewTicker(intervaloCancelamento)
		defer ticker.Stop()

		for {
			select {
			case <-parar:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if p.cancelamentoPedido(ctx, downloadID) {
					log.Printf("jobs: cancelamento solicitado id=%s", downloadID)
					cancelar()
					return
				}
			}
		}
	}()

	return parar
}

func (p *JobDownloadMedia) cancelamentoPedido(ctx context.Context, downloadID uuid.UUID) bool {
	if p.redis == nil {
		return false
	}
	existe, err := p.redis.Exists(ctx, ChaveCancelamento(downloadID.String())).Result()
	return err == nil && existe > 0
}

// ChaveCancelamento é a chave que a API grava e o worker observa. Fica aqui
// para os dois lados não divergirem no formato.
func ChaveCancelamento(downloadID string) string {
	return "download:cancel:" + downloadID
}

// bigFromFloat converte o percentual para o inteiro com duas casas que a
// coluna NUMERIC(5,2) espera.
func bigFromFloat(valor float64) *big.Int {
	if valor < 0 {
		valor = 0
	}
	if valor > 100 {
		valor = 100
	}
	return big.NewInt(int64(valor*100 + 0.5))
}

func (p *JobDownloadMedia) gravarProgresso(ctx context.Context, downloadID uuid.UUID, progresso media.Progress) {
	// O contexto do job pode já ter sido cancelado; a gravação do último
	// estado ainda interessa, então usa um prazo próprio.
	gravacaoCtx, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelar()

	if err := p.store.UpdateDownloadProgress(gravacaoCtx, db.UpdateDownloadProgressParams{
		ID:              downloadID,
		ProgressPercent: pgtype.Numeric{Int: bigFromFloat(progresso.Percent), Exp: -2, Valid: true},
		DownloadedBytes: progresso.DownloadedBytes,
		TotalBytes:      progresso.TotalBytes,
		SpeedBps:        progresso.SpeedBPS,
		EtaSeconds:      int32(progresso.ETASeconds),
	}); err != nil {
		log.Printf("jobs: falha ao gravar progresso id=%s: %v", downloadID, err)
	}
}

func (p *JobDownloadMedia) publicarProgresso(ctx context.Context, download db.Download, progresso media.Progress) {
	p.publicar(ctx, download, db.CoreDownloadStatusPROCESSING, progresso, "")
}

func (p *JobDownloadMedia) publicar(
	ctx context.Context, download db.Download,
	status db.CoreDownloadStatus, progresso media.Progress, mensagemErro string,
) {
	evento := stream.DownloadEvent{
		ID:            download.ID.String(),
		UserID:        download.UserID.String(),
		Status:        status,
		Title:         download.Title,
		Platform:      download.Platform,
		PlatformLabel: media.Platform(download.Platform).Label(),
		ErrorMessage:  mensagemErro,
		Progress: &stream.ProgressEvent{
			Percent:         progresso.Percent,
			DownloadedBytes: progresso.DownloadedBytes,
			TotalBytes:      progresso.TotalBytes,
			SpeedBPS:        progresso.SpeedBPS,
			ETASeconds:      progresso.ETASeconds,
			Postprocess:     progresso.Postprocess,
		},
	}

	if err := p.rdStream.Publish(context.WithoutCancel(ctx), stream.StreamName, evento); err != nil {
		log.Printf("jobs: falha ao publicar evento id=%s: %v", download.ID, err)
	}
}

// registrarFalha grava o desfecho e devolve o erro que o asynq deve ver. Um
// cancelamento não é falha: reenfileirar seria desfazer o pedido do usuário.
func (p *JobDownloadMedia) registrarFalha(ctx context.Context, download db.Download, err error) error {
	gravacaoCtx := context.WithoutCancel(ctx)

	status := db.CoreDownloadStatusFAILED
	if errors.Is(err, media.ErrCanceled) {
		status = db.CoreDownloadStatusCANCELED
	}

	// O histórico e o evento de tempo real são lidos pelo CLIENTE FINAL. Só a
	// mensagem pública entra aí; o motivo técnico fica no log, que é nosso.
	mensagem := media.PublicMessage(err)
	log.Printf("jobs: download falhou id=%s plataforma=%s status=%s motivo=%q erro=%v",
		download.ID, download.Platform, status, media.UserMessage(err), err)
	if detalhe := media.Detail(err); detalhe != "" {
		log.Printf("jobs: detalhe id=%s: %s", download.ID, detalhe)
	}

	// O detalhe fica gravado ao lado da mensagem pública, e não no lugar dela:
	// é o que permite ao super admin ver no histórico por que um download
	// falhou, sem precisar caçar a linha no log do container.
	detalhe := media.Detail(err)
	if detalhe == "" {
		detalhe = media.UserMessage(err)
	}

	if erroGravacao := p.store.MarkDownloadFinished(gravacaoCtx, db.MarkDownloadFinishedParams{
		ID:           download.ID,
		Status:       status,
		ErrorMessage: pgtype.Text{String: mensagem, Valid: mensagem != ""},
		ErrorDetail:  pgtype.Text{String: detalhe, Valid: detalhe != ""},
	}); erroGravacao != nil {
		log.Printf("jobs: falha ao gravar desfecho id=%s: %v", download.ID, erroGravacao)
	}

	p.publicar(gravacaoCtx, download, status, media.Progress{}, mensagem)

	if status == db.CoreDownloadStatusCANCELED {
		// Nada a repetir: o usuário pediu para parar.
		return nil
	}
	// Erros de conteúdo não melhoram com nova tentativa; só os de
	// disponibilidade valem repetir.
	if !vaiRepetir(err) {
		return fmt.Errorf("%s: %w", mensagem, asynq.SkipRetry)
	}
	return errors.New(mensagem)
}

// vaiRepetir decide se vale a pena o asynq tentar de novo.
func vaiRepetir(err error) bool {
	switch {
	case errors.Is(err, media.ErrContentUnavailable),
		errors.Is(err, media.ErrContentPrivate),
		errors.Is(err, media.ErrAuthRequired),
		errors.Is(err, media.ErrGeoBlocked),
		errors.Is(err, media.ErrLiveContent),
		errors.Is(err, media.ErrUnsupportedPlatform),
		errors.Is(err, media.ErrInvalidURL),
		errors.Is(err, media.ErrFormatUnavailable):
		return false
	}
	return true
}

// encaminharParaUpload move o arquivo para onde o job de upload já existente o
// espera e enfileira a etapa seguinte, preservando o fluxo do projeto.
func (p *JobDownloadMedia) encaminharParaUpload(
	ctx context.Context, download db.Download, resultado *media.Result,
) error {
	audio := download.Format == db.CoreFormatTypeMP3

	destinoDir := "./uploads/videos"
	if audio {
		destinoDir = "./uploads/musics"
	}
	if err := utils.CreateFolder(destinoDir); err != nil {
		return fmt.Errorf("falha ao preparar o diretório de upload: %v", err)
	}

	nomeFinal := fmt.Sprintf("media_%s.%s", download.ID.String(), resultado.Ext)
	destino := filepath.Join(destinoDir, nomeFinal)
	if err := moverArquivo(resultado.FilePath, destino); err != nil {
		return fmt.Errorf("falha ao mover o arquivo baixado: %v", err)
	}

	// A miniatura é opcional: sem ela o card fica sem imagem, mas o arquivo já
	// está pronto e não faz sentido perder o download por causa disso.
	banner := filepath.Join(destinoDir, "banners", download.ID.String()+"_banner.jpg")
	if download.ThumbnailUrl.String != "" {
		if err := helpers.DownloadImage(ctx, download.ThumbnailUrl.String, banner); err != nil {
			log.Printf("jobs: miniatura indisponível id=%s: %v", download.ID, err)
			banner = ""
		}
	} else {
		banner = ""
	}

	var task *asynq.Task
	var fila string
	var err error

	if audio {
		task, err = tasks.NewUploadMusicTask(destino, download.ID.String(), nomeFinal, banner)
		fila = queues.TypeUploadMusicQueue
	} else {
		task, err = tasks.NewUploadVideoTask(destino, download.ID.String(), nomeFinal, banner)
		fila = queues.TypeUploadVideoQueue
	}
	if err != nil {
		return fmt.Errorf("falha ao criar a tarefa de upload: %v", err)
	}

	if _, err := p.client.EnqueueContext(ctx, task, asynq.Queue(fila)); err != nil {
		return fmt.Errorf("falha ao enfileirar o upload: %v", err)
	}
	return nil
}

// moverArquivo tenta o rename e cai para cópia quando origem e destino estão em
// sistemas de arquivos diferentes — o que acontece se o diretório temporário
// for um volume separado.
func moverArquivo(origem, destino string) error {
	if err := os.Rename(origem, destino); err == nil {
		return nil
	}

	entrada, err := os.Open(origem)
	if err != nil {
		return err
	}
	defer entrada.Close()

	saida, err := os.OpenFile(destino, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer saida.Close()

	if _, err := io.Copy(saida, entrada); err != nil {
		return err
	}
	return saida.Sync()
}
