package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/jobs"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/tokens"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
)

// resolveTimeout limita a resolução de metadados. Ela roda dentro da
// requisição HTTP — é rápida por natureza (não baixa o vídeo), mas uma
// plataforma lenta não pode segurar a conexão indefinidamente.
const resolveTimeout = 50 * time.Second

// cancelTTL é por quanto tempo o pedido de cancelamento fica válido. Cobre com
// folga o intervalo entre marcar e o worker perceber, e some sozinho depois —
// uma chave eterna cancelaria uma tentativa futura do mesmo identificador.
const cancelTTL = 10 * time.Minute

type resolveRequest struct {
	URL string `json:"url" binding:"required"`
}

type formatoResposta struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Kind            string `json:"kind"`
	Ext             string `json:"ext"`
	Height          int    `json:"height,omitempty"`
	FPS             int    `json:"fps,omitempty"`
	SizeBytes       int64  `json:"size_bytes,omitempty"`
	SizeApproximate bool   `json:"size_approximate,omitempty"`
}

// resolveMedia identifica a plataforma e devolve os metadados para a tela
// montar a escolha de qualidade. Não cria download e não conta contra o limite
// diário: é só a consulta que antecede a decisão do usuário.
func (s *Server) resolveMedia(ctx *gin.Context) {
	var req resolveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("informe uma URL")))
		return
	}

	if s.mediaRegistry == nil {
		ctx.JSON(http.StatusServiceUnavailable,
			errorResponse(errors.New("o mecanismo de download está indisponível")))
		return
	}

	requestCtx, cancelar := context.WithTimeout(ctx.Request.Context(), resolveTimeout)
	defer cancelar()

	metadata, normalizada, err := s.mediaRegistry.Metadata(requestCtx, req.URL)
	if err != nil {
		// O detalhe técnico fica no log; a tela recebe só a mensagem de
		// domínio, sem saída de processo nem caminho de arquivo.
		log.Printf("media: falha ao resolver plataforma=%s: %v", plataformaDe(err, req.URL), err)
		ctx.JSON(statusPara(err), gin.H{
			"error": media.UserMessage(err),
			"code":  codigoErro(err),
		})
		return
	}

	formatos := make([]formatoResposta, 0, len(metadata.Formats))
	for _, formato := range metadata.Formats {
		formatos = append(formatos, formatoResposta{
			ID: formato.ID, Label: formato.Label, Kind: string(formato.Kind),
			Ext: formato.Ext, Height: formato.Height, FPS: formato.FPS,
			SizeBytes: formato.SizeBytes, SizeApproximate: formato.SizeApproximate,
		})
	}

	s.guardarMetadata(ctx.Request.Context(), normalizada, metadata)

	log.Printf("media: resolvido plataforma=%s provider=%s formatos=%d",
		metadata.Platform, metadata.Provider, len(formatos))

	ctx.JSON(http.StatusOK, gin.H{
		"url":            normalizada,
		"platform":       string(metadata.Platform),
		"platform_label": metadata.Platform.Label(),
		"provider":       metadata.Provider,
		"content_id":     metadata.ContentID,
		"title":          metadata.Title,
		"description":    metadata.Description,
		"thumbnail":      metadata.Thumbnail,
		"duration":       metadata.Duration,
		"uploader":       metadata.Uploader,
		"uploader_id":    metadata.UploaderID,
		"uploader_url":   metadata.UploaderURL,
		"follower_count": metadata.FollowerCount,
		"view_count":     metadata.ViewCount,
		"like_count":     metadata.LikeCount,
		"upload_date":    metadata.UploadDate,
		"webpage_url":    metadata.WebpageURL,
		"formats":        formatos,
	})
}

type criarDownloadMediaRequest struct {
	URL string `json:"url" binding:"required"`
	// FormatID vem da resposta de /resolve. Vazio usa o padrão do tipo.
	FormatID string `json:"format_id"`
	// Kind separa vídeo de áudio; é o que decide a conversão para MP3.
	Kind string `json:"kind" binding:"omitempty,oneof=video audio"`
}

// createMediaDownload resolve, valida os limites e enfileira. A resolução
// acontece aqui, e não no worker, porque o usuário precisa do título e do
// tamanho para saber se o pedido foi aceito.
func (s *Server) createMediaDownload(ctx *gin.Context) {
	var req criarDownloadMediaRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("pedido inválido")))
		return
	}
	s.criarDownloadMedia(ctx, req)
}

// criarDownloadMedia é o caminho único de criação de download, compartilhado
// pela rota nova e pela antiga.
func (s *Server) criarDownloadMedia(ctx *gin.Context, req criarDownloadMediaRequest) {
	if s.mediaRegistry == nil {
		ctx.JSON(http.StatusServiceUnavailable,
			errorResponse(errors.New("o mecanismo de download está indisponível")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*tokens.Payload)
	userID, err := utils.ParseUUID(authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("usuário inválido")))
		return
	}

	user, err := s.store.GetUserByID(ctx.Request.Context(), userID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("usuário não encontrado")))
		return
	}

	normalizada, _, err := media.NormalizeURL(req.URL)
	if err != nil {
		ctx.JSON(statusPara(err), gin.H{"error": media.UserMessage(err), "code": codigoErro(err)})
		return
	}

	// A resolução da tela é reaproveitada: além de poupar uma ida à
	// plataforma, é o que mantém o formato escolhido válido — os ids não são
	// estáveis entre duas resoluções do mesmo conteúdo.
	metadata := s.metadataEmCache(ctx.Request.Context(), normalizada)
	if metadata == nil {
		requestCtx, cancelar := context.WithTimeout(ctx.Request.Context(), resolveTimeout)
		defer cancelar()

		metadata, normalizada, err = s.mediaRegistry.Metadata(requestCtx, req.URL)
		if err != nil {
			log.Printf("media: falha ao resolver na criação: %v", err)
			ctx.JSON(statusPara(err), gin.H{"error": media.UserMessage(err), "code": codigoErro(err)})
			return
		}
	}

	kind := media.KindVideo
	formato := db.CoreFormatTypeMP4
	if req.Kind == string(media.KindAudio) {
		kind, formato = media.KindAudio, db.CoreFormatTypeMP3
	}

	// O formato precisa ser um dos que ESTE conteúdo oferece: aceitar um id
	// qualquer deixaria o cliente escolher o que o provider vai executar.
	escolhido, ok := escolherFormato(metadata.Formats, req.FormatID, kind)
	if req.FormatID != "" && !ok {
		// O id pode ter sumido entre a resolução e o clique, o que é normal:
		// o provider não garante ids estáveis. Recusar só quando ele nunca
		// poderia ter existido — um valor fora do vocabulário de formatos.
		if !pareceFormatoDoProvider(req.FormatID) {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": media.ErrFormatUnavailable.Error(),
				"code":  "format_unavailable",
			})
			return
		}
		log.Printf("media: formato %q não está mais disponível; usando o melhor de %s",
			req.FormatID, kind)
		escolhido, _ = escolherFormato(metadata.Formats, "", kind)
	}

	if resposta, excedeu := s.limiteExcedido(user, escolhido.SizeBytes); excedeu {
		ctx.JSON(http.StatusBadRequest, resposta)
		return
	}

	download, err := s.store.CreateMediaDownload(ctx.Request.Context(), db.CreateMediaDownloadParams{
		ID:              utils.GenerateUUID(),
		UserID:          userID,
		OriginalUrl:     normalizada,
		Title:           metadata.Title,
		Format:          formato,
		Status:          db.CoreDownloadStatusPENDING,
		ThumbnailUrl:    pgtype.Text{String: metadata.Thumbnail, Valid: metadata.Thumbnail != ""},
		DurationSeconds: pgtype.Int4{Int32: int32(metadata.Duration), Valid: true},
		Platform:        string(metadata.Platform),
		Provider:        pgtype.Text{String: metadata.Provider, Valid: true},
		FormatID:        pgtype.Text{String: escolhido.ID, Valid: escolhido.ID != ""},
		QualityLabel:    pgtype.Text{String: escolhido.Label, Valid: escolhido.Label != ""},
		Uploader:        pgtype.Text{String: metadata.Uploader, Valid: metadata.Uploader != ""},
		TotalBytes:      escolhido.SizeBytes,
	})
	if err != nil {
		log.Printf("media: falha ao criar download: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao criar o download")))
		return
	}

	task, err := tasks.NewDownloadMediaTask(download.ID.String())
	if err == nil {
		_, err = s.queueClient.EnqueueContext(ctx.Request.Context(), task,
			asynq.Queue(queues.TypeDownloadMediaQueue))
	}
	if err != nil {
		log.Printf("media: falha ao enfileirar id=%s: %v", download.ID, err)
		// Sem o job, o download ficaria PENDING para sempre.
		_ = s.store.MarkDownloadFinished(ctx.Request.Context(), db.MarkDownloadFinishedParams{
			ID:           download.ID,
			Status:       db.CoreDownloadStatusFAILED,
			ErrorMessage: pgtype.Text{String: "não foi possível iniciar o download", Valid: true},
		})
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("não foi possível iniciar o download")))
		return
	}

	log.Printf("media: download criado id=%s plataforma=%s provider=%s formato=%s",
		download.ID, metadata.Platform, metadata.Provider, escolhido.Label)

	ctx.JSON(http.StatusCreated, gin.H{
		"id":             download.ID.String(),
		"status":         download.Status,
		"title":          download.Title,
		"platform":       string(metadata.Platform),
		"platform_label": metadata.Platform.Label(),
		"quality_label":  escolhido.Label,
		"thumbnail_url":  metadata.Thumbnail,
		"created_at":     download.CreatedAt,
	})
}

// cancelDownload marca o pedido e avisa o worker. A marcação no banco é o que
// vale para a tela; a chave no Redis é o que alcança um processo já em
// andamento.
func (s *Server) cancelDownload(ctx *gin.Context) {
	downloadID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador inválido")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*tokens.Payload)
	userID, err := utils.ParseUUID(authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("usuário inválido")))
		return
	}

	// O UPDATE já filtra por dono e por status: um download de outro usuário,
	// ou já concluído, simplesmente não casa.
	download, err := s.store.CancelDownload(ctx.Request.Context(), db.CancelDownloadParams{
		ID: downloadID, UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusConflict,
				errorResponse(errors.New("este download não pode mais ser cancelado")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao cancelar")))
		return
	}

	if s.redis != nil {
		if err := s.redis.Set(ctx.Request.Context(),
			jobs.ChaveCancelamento(downloadID.String()), "1", cancelTTL).Err(); err != nil {
			// O banco já marcou; o processo em andamento é que vai seguir até
			// o fim e ter o resultado descartado.
			log.Printf("media: falha ao sinalizar cancelamento id=%s: %v", downloadID, err)
		}
	}

	log.Printf("media: cancelamento pedido id=%s por=%s", downloadID, userID)
	ctx.JSON(http.StatusOK, gin.H{"id": download.ID.String(), "status": download.Status})
}

// escolherFormato valida o id contra os formatos deste conteúdo e devolve o
// padrão quando nenhum foi pedido.
func escolherFormato(formatos []media.Format, formatoID string, kind media.Kind) (media.Format, bool) {
	if formatoID != "" {
		for _, formato := range formatos {
			if formato.ID == formatoID {
				return formato, true
			}
		}
		return media.Format{}, false
	}

	// Sem escolha: o melhor do tipo pedido. A lista já vem ordenada da maior
	// resolução para a menor.
	for _, formato := range formatos {
		if formato.Kind == kind {
			return formato, true
		}
	}
	return media.Format{}, true
}

// pareceFormatoDoProvider aceita só o vocabulário que um provider emitiria.
// É a barreira contra um cliente mandar qualquer coisa no lugar do id: sem
// ela, a degradação silenciosa acima aceitaria entrada arbitrária.
func pareceFormatoDoProvider(formatoID string) bool {
	if formatoID == "" || len(formatoID) > 120 {
		return false
	}
	for _, r := range formatoID {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '-', r == '_', r == '.', r == '+', r == '/':
		default:
			return false
		}
	}
	// Um id nunca começa com hífen; isso é opção de linha de comando.
	return formatoID[0] != '-'
}

// limiteExcedido aplica os tetos por plano que o projeto já tem.
func (s *Server) limiteExcedido(user db.User, tamanho int64) (gin.H, bool) {
	if user.Role == db.CoreUserRoleSuperAdmin || tamanho <= 0 {
		return nil, false
	}

	limite := int64(0)
	switch user.Plan {
	case db.CorePlanTypeFree:
		limite = s.config.LimitDownloadFree
	case db.CorePlanTypePremium:
		limite = s.config.LimitDownloadPremium
	default:
		return nil, false
	}

	if limite > 0 && tamanho > limite {
		return gin.H{
			"code":    "limit_exceeded",
			"message": "O tamanho do arquivo excede o limite do seu plano",
			"limit":   limite,
			"size":    tamanho,
		}, true
	}
	return nil, false
}

// statusPara traduz o erro de domínio para o código HTTP correspondente.
func statusPara(err error) int {
	switch {
	case errors.Is(err, media.ErrInvalidURL), errors.Is(err, media.ErrFormatUnavailable):
		return http.StatusBadRequest
	case errors.Is(err, media.ErrUnsupportedPlatform), errors.Is(err, media.ErrContentUnavailable):
		return http.StatusNotFound
	case errors.Is(err, media.ErrContentPrivate), errors.Is(err, media.ErrAuthRequired),
		errors.Is(err, media.ErrGeoBlocked), errors.Is(err, media.ErrLiveContent):
		return http.StatusUnprocessableEntity
	case errors.Is(err, media.ErrRateLimited):
		return http.StatusTooManyRequests
	case errors.Is(err, media.ErrTimeout):
		return http.StatusGatewayTimeout
	case errors.Is(err, media.ErrProviderUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

// codigoErro dá à tela um identificador estável, independente do texto — a
// mensagem pode mudar sem quebrar o tratamento no frontend.
func codigoErro(err error) string {
	switch {
	case errors.Is(err, media.ErrInvalidURL):
		return "invalid_url"
	case errors.Is(err, media.ErrUnsupportedPlatform):
		return "unsupported_platform"
	case errors.Is(err, media.ErrContentUnavailable):
		return "content_unavailable"
	case errors.Is(err, media.ErrContentPrivate):
		return "content_private"
	case errors.Is(err, media.ErrAuthRequired):
		return "auth_required"
	case errors.Is(err, media.ErrGeoBlocked):
		return "geo_blocked"
	case errors.Is(err, media.ErrLiveContent):
		return "live_content"
	case errors.Is(err, media.ErrFormatUnavailable):
		return "format_unavailable"
	case errors.Is(err, media.ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, media.ErrTimeout):
		return "timeout"
	case errors.Is(err, media.ErrProviderUnavailable):
		return "provider_unavailable"
	default:
		return "download_failed"
	}
}

// plataformaDe identifica a plataforma só para o log, sem falhar quando a URL
// já foi recusada.
func plataformaDe(err error, bruta string) media.Platform {
	if _, parsed, erroURL := media.NormalizeURL(bruta); erroURL == nil {
		return media.PlatformFor(parsed)
	}
	return media.PlatformUnknown
}
