package server

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

// fusoPainel alinha o corte dos dias com o do limite diário, que já usa
// America/Sao_Paulo. Sem isso, "hoje" no gráfico começaria três horas antes de
// "hoje" no contador de downloads.
var fusoPainel = time.FixedZone("America/Sao_Paulo", -3*60*60)

// periodo é o intervalo pedido pelo painel, já com a granularidade escolhida.
type periodo struct {
	De          time.Time
	Ate         time.Time
	Granularity string
}

// resolverPeriodo aceita tanto os atalhos (7d, 30d…) quanto um intervalo
// personalizado. A granularidade é derivada do tamanho do intervalo: 90 dias em
// barras de hora daria 2160 pontos e um gráfico ilegível.
func resolverPeriodo(ctx *gin.Context) (periodo, error) {
	agora := time.Now().In(fusoPainel)
	inicioDoDia := time.Date(agora.Year(), agora.Month(), agora.Day(), 0, 0, 0, 0, fusoPainel)

	if de, ate := ctx.Query("from"), ctx.Query("to"); de != "" && ate != "" {
		inicio, err := time.ParseInLocation("2006-01-02", de, fusoPainel)
		if err != nil {
			return periodo{}, errors.New("data inicial inválida (use AAAA-MM-DD)")
		}
		fim, err := time.ParseInLocation("2006-01-02", ate, fusoPainel)
		if err != nil {
			return periodo{}, errors.New("data final inválida (use AAAA-MM-DD)")
		}
		// O fim é exclusivo na consulta; somar um dia inclui o dia escolhido.
		fim = fim.AddDate(0, 0, 1)
		if !fim.After(inicio) {
			return periodo{}, errors.New("a data final precisa ser posterior à inicial")
		}
		if fim.Sub(inicio) > 366*24*time.Hour {
			return periodo{}, errors.New("o intervalo não pode passar de um ano")
		}
		return periodo{De: inicio, Ate: fim, Granularity: granularidadePara(fim.Sub(inicio))}, nil
	}

	switch ctx.DefaultQuery("range", "30d") {
	case "today":
		return periodo{De: inicioDoDia, Ate: inicioDoDia.AddDate(0, 0, 1), Granularity: "hour"}, nil
	case "yesterday":
		ontem := inicioDoDia.AddDate(0, 0, -1)
		return periodo{De: ontem, Ate: inicioDoDia, Granularity: "hour"}, nil
	case "7d":
		return periodo{De: inicioDoDia.AddDate(0, 0, -6), Ate: inicioDoDia.AddDate(0, 0, 1), Granularity: "day"}, nil
	case "15d":
		return periodo{De: inicioDoDia.AddDate(0, 0, -14), Ate: inicioDoDia.AddDate(0, 0, 1), Granularity: "day"}, nil
	case "90d":
		return periodo{De: inicioDoDia.AddDate(0, 0, -89), Ate: inicioDoDia.AddDate(0, 0, 1), Granularity: "week"}, nil
	case "12m":
		return periodo{De: inicioDoDia.AddDate(-1, 0, 1), Ate: inicioDoDia.AddDate(0, 0, 1), Granularity: "month"}, nil
	default: // 30d
		return periodo{De: inicioDoDia.AddDate(0, 0, -29), Ate: inicioDoDia.AddDate(0, 0, 1), Granularity: "day"}, nil
	}
}

func granularidadePara(duracao time.Duration) string {
	switch {
	case duracao <= 48*time.Hour:
		return "hour"
	case duracao <= 45*24*time.Hour:
		return "day"
	case duracao <= 180*24*time.Hour:
		return "week"
	default:
		return "month"
	}
}

type pontoSerie struct {
	Bucket           time.Time `json:"bucket"`
	Total            int64     `json:"total"`
	Completed        int64     `json:"completed"`
	Failed           int64     `json:"failed"`
	TransferredBytes int64     `json:"transferred_bytes"`
}

// overview é tudo que a tela inicial do painel precisa, em uma requisição só.
func (s *Server) overview(ctx *gin.Context) {
	intervalo, err := resolverPeriodo(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	requestCtx := ctx.Request.Context()
	de, ate := intervalo.De, intervalo.Ate

	resumo, err := s.store.AdminDownloadsSummary(requestCtx, db.AdminDownloadsSummaryParams{From: &de, To: &ate})
	if err != nil {
		log.Printf("admin: falha no resumo de downloads: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar métricas")))
		return
	}

	armazenamento, err := s.store.AdminStorageSummary(requestCtx)
	if err != nil {
		log.Printf("admin: falha no resumo de armazenamento: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar métricas")))
		return
	}

	totais, err := s.store.AdminPlatformTotals(requestCtx)
	if err != nil {
		log.Printf("admin: falha nos totais da plataforma: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar métricas")))
		return
	}

	linhas, err := s.store.AdminDownloadsTimeSeries(requestCtx, db.AdminDownloadsTimeSeriesParams{
		Granularity: intervalo.Granularity, From: &de, To: &ate,
	})
	if err != nil {
		log.Printf("admin: falha na série temporal: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar métricas")))
		return
	}

	serie := make([]pontoSerie, 0, len(linhas))
	for _, linha := range linhas {
		ponto := pontoSerie{
			Total: linha.Total, Completed: linha.Completed,
			Failed: linha.Failed, TransferredBytes: linha.TransferredBytes,
		}
		if linha.Bucket != nil {
			ponto.Bucket = linha.Bucket.In(fusoPainel)
		}
		serie = append(serie, ponto)
	}

	formatos, err := s.store.AdminDownloadsByFormat(requestCtx, db.AdminDownloadsByFormatParams{From: &de, To: &ate})
	if err != nil {
		log.Printf("admin: falha na quebra por formato: %v", err)
		formatos = nil
	}

	topUsuarios, err := s.store.AdminTopUsers(requestCtx, db.AdminTopUsersParams{From: &de, To: &ate, Limit: 5})
	if err != nil {
		log.Printf("admin: falha no ranking de usuários: %v", err)
		topUsuarios = nil
	}

	ctx.JSON(http.StatusOK, gin.H{
		"period": gin.H{
			"from":        intervalo.De,
			"to":          intervalo.Ate,
			"granularity": intervalo.Granularity,
		},
		"downloads": gin.H{
			"total":             resumo.Total,
			"completed":         resumo.Completed,
			"failed":            resumo.Failed,
			"processing":        resumo.Processing,
			"expired":           resumo.Expired,
			"active_users":      resumo.ActiveUsers,
			"transferred_bytes": resumo.TransferredBytes,
		},
		"storage": gin.H{
			"stored_bytes":   armazenamento.StoredBytes,
			"stored_files":   armazenamento.StoredFiles,
			"lifetime_bytes": armazenamento.LifetimeBytes,
			"freed_bytes":    armazenamento.FreedBytes,
			"freed_files":    armazenamento.FreedFiles,
		},
		"platform": gin.H{
			"users_total":     totais.UsersTotal,
			"users_active":    totais.UsersActive,
			"users_premium":   totais.UsersPremium,
			"users_new_30d":   totais.UsersNew30d,
			"downloads_total": totais.DownloadsTotal,
		},
		"series":    serie,
		"formats":   formatos,
		"top_users": topUsuarios,
	})
}

// listDownloads é a visão global do histórico, com filtro por usuário, status e
// texto.
func (s *Server) listDownloads(ctx *gin.Context) {
	limit, offset, page, perPage := paginacao(ctx)
	busca := filtroTexto(ctx, "search")

	var usuario pgtype.UUID
	if bruto := ctx.Query("user_id"); bruto != "" {
		parsed, err := uuid.Parse(bruto)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("user_id inválido")))
			return
		}
		usuario = db.ToPgUUID(parsed)
	}

	var status db.NullCoreDownloadStatus
	if bruto := ctx.Query("status"); bruto != "" {
		switch db.CoreDownloadStatus(bruto) {
		case db.CoreDownloadStatusPENDING, db.CoreDownloadStatusPROCESSING,
			db.CoreDownloadStatusCOMPLETED, db.CoreDownloadStatusFAILED,
			db.CoreDownloadStatusCANCELED, db.CoreDownloadStatusEXPIRED,
			db.CoreDownloadStatusRETRYING:
			status = db.NullCoreDownloadStatus{CoreDownloadStatus: db.CoreDownloadStatus(bruto), Valid: true}
		default:
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("status inválido")))
			return
		}
	}

	requestCtx := ctx.Request.Context()

	total, err := s.store.AdminCountDownloads(requestCtx, db.AdminCountDownloadsParams{
		UserID: usuario, Status: status, Search: busca,
	})
	if err != nil {
		log.Printf("admin: falha ao contar downloads: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar downloads")))
		return
	}

	linhas, err := s.store.AdminListDownloads(requestCtx, db.AdminListDownloadsParams{
		Limit: int32(limit), Offset: int32(offset),
		UserID: usuario, Status: status, Search: busca,
	})
	if err != nil {
		log.Printf("admin: falha ao listar downloads: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar downloads")))
		return
	}

	type item struct {
		ID              string    `json:"id"`
		UserID          string    `json:"user_id"`
		UserName        string    `json:"user_name"`
		UserEmail       string    `json:"user_email"`
		Title           string    `json:"title"`
		OriginalUrl     string    `json:"original_url"`
		Format          string    `json:"format"`
		Status          string    `json:"status"`
		FileSizeBytes   int64     `json:"file_size_bytes"`
		DurationSeconds int32     `json:"duration_seconds"`
		ErrorMessage    string    `json:"error_message,omitempty"`
		ExpiresAt       string    `json:"expires_at,omitempty"`
		CreatedAt       time.Time `json:"created_at"`
	}

	downloads := make([]item, 0, len(linhas))
	for _, linha := range linhas {
		atual := item{
			ID: linha.ID.String(), UserID: linha.UserID.String(),
			UserName: linha.UserName, UserEmail: linha.UserEmail,
			Title: linha.Title, OriginalUrl: linha.OriginalUrl,
			Format: string(linha.Format), Status: string(linha.Status),
			FileSizeBytes: linha.FileSizeBytes, DurationSeconds: linha.DurationSeconds.Int32,
			ErrorMessage: linha.ErrorMessage.String,
		}
		if linha.ExpiresAt.Valid {
			atual.ExpiresAt = linha.ExpiresAt.Time.Format(time.RFC3339)
		}
		if linha.CreatedAt != nil {
			atual.CreatedAt = *linha.CreatedAt
		}
		downloads = append(downloads, atual)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"downloads": downloads,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		"next_page": int64(offset+len(linhas)) < total,
		"prev_page": page > 1,
	})
}

// deleteDownloadAdmin remove qualquer download, de qualquer usuário. A remoção
// do arquivo no bucket é enfileirada, e não feita aqui: o painel não pode
// travar esperando o R2.
func (s *Server) deleteDownloadAdmin(ctx *gin.Context) {
	downloadID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador inválido")))
		return
	}

	download, err := s.store.GetDownloadByID(ctx.Request.Context(), downloadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("download não encontrado")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar download")))
		return
	}

	if err := s.store.DeleteDownload(ctx.Request.Context(), downloadID); err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao remover download")))
		return
	}

	// Só há arquivo no bucket se o download chegou a concluir. A falha aqui não
	// desfaz a remoção: o registro já saiu da listagem, e um objeto órfão no R2
	// é menos grave do que um download que reaparece na tela.
	if download.FileUrl.Valid && download.FileUrl.String != "" {
		if err := s.storage.DeleteFile(ctx.Request.Context(), download.FileUrl.String); err != nil {
			log.Printf("admin: falha ao remover arquivo do storage download_id=%s: %v", downloadID, err)
		}
	}

	admin, _ := currentUser(ctx)
	log.Printf("admin: download removido download_id=%s por=%s", downloadID, admin.ID)
	ctx.JSON(http.StatusOK, gin.H{"message": "download removido"})
}
