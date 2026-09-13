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
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/jobs"
	"github.com/wenealves10/yt-dlp-downloader/internal/libs/stream"
	"github.com/wenealves10/yt-dlp-downloader/internal/media"
	"github.com/wenealves10/yt-dlp-downloader/internal/queues"
	"github.com/wenealves10/yt-dlp-downloader/internal/tasks"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
	"github.com/wenealves10/yt-dlp-downloader/internal/ytaccounts"
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
			errorResponse(errors.New(media.PublicMessage(media.ErrProviderUnavailable))))
		return
	}

	requestCtx, cancelar := context.WithTimeout(ctx.Request.Context(), resolveTimeout)
	defer cancelar()

	// A sessão é emprestada já na resolução, e não só no download: o Vimeo
	// recusa a leitura de metadados sem estar logado, então sem isto o usuário
	// nem chegaria à tela de escolher a qualidade.
	lease, sessao := s.sessaoPara(requestCtx, req.URL)
	defer lease.Release()

	metadata, normalizada, err := s.mediaRegistry.Metadata(requestCtx, req.URL, media.MetadataOptions{
		CookieFile: arquivoDeSessao(lease),
	})
	if err != nil {
		// O detalhe técnico fica no log; a tela recebe só a mensagem de
		// domínio, sem saída de processo nem caminho de arquivo.
		log.Printf("media: falha ao resolver plataforma=%s sessao=%q: %v",
			plataformaDe(err, req.URL), sessao.Descricao(), err)

		corpo := respostaDeErro(ctx, err)
		// A primeira pergunta quando falha numa plataforma que exige login é
		// "a conta que eu cadastrei foi usada?". Sem responder isso, o
		// administrador não tem por onde começar — e o cliente final não pode
		// nem saber que existe conta gerenciada.
		if user, ok := currentUser(ctx); ok && user.Role == db.CoreUserRoleSuperAdmin {
			corpo["session"] = sessao.Descricao()
		}
		ctx.JSON(statusPara(err), corpo)
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

	log.Printf("media: resolvido plataforma=%s provider=%s formatos=%d sessao=%q",
		metadata.Platform, metadata.Provider, len(formatos), sessao.Descricao())

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

// criarDownloadMedia responde a criação para a TELA (usuário autenticado por
// token). A preparação em si está em prepararDownloadMedia, que é compartilhada
// com a API de integrações: as duas superfícies criam o download exatamente do
// mesmo jeito e diferem só no formato da resposta.
func (s *Server) criarDownloadMedia(ctx *gin.Context, req criarDownloadMediaRequest) {
	download, ok := s.prepararDownloadMedia(ctx, req)
	if !ok {
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"id":             download.ID.String(),
		"status":         download.Status,
		"title":          download.Title,
		"platform":       download.Platform,
		"platform_label": media.Platform(download.Platform).Label(),
		"quality_label":  download.QualityLabel.String,
		"thumbnail_url":  download.ThumbnailUrl.String,
		"created_at":     download.CreatedAt,
	})
}

// prepararDownloadMedia resolve, valida os limites, grava e enfileira.
//
// Devolve o download criado e um booleano que diz se a requisição já foi
// respondida com erro. Quem chama renderiza a resposta de sucesso na forma da
// sua superfície — e é só isso que as duas não compartilham.
func (s *Server) prepararDownloadMedia(ctx *gin.Context, req criarDownloadMediaRequest) (db.Download, bool) {
	if s.mediaRegistry == nil {
		ctx.JSON(http.StatusServiceUnavailable,
			errorResponse(errors.New(media.PublicMessage(media.ErrProviderUnavailable))))
		return db.Download{}, false
	}

	// Pessoa ou sistema: o middleware que autenticou já deixou a linha de
	// `users` no contexto, e a criação do download não precisa saber qual dos
	// dois foi. É o que permite à API de integrações reaproveitar exatamente
	// este caminho, com toda a lógica de formato e sessão que ele já carrega.
	user, autenticado := usuarioAtual(ctx)
	if !autenticado {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("requisição não autenticada")))
		return db.Download{}, false
	}
	userID := user.ID

	normalizada, _, err := media.NormalizeURL(req.URL)
	if err != nil {
		ctx.JSON(statusPara(err), respostaDeErro(ctx, err))
		return db.Download{}, false
	}

	// A resolução da tela é reaproveitada: além de poupar uma ida à
	// plataforma, é o que mantém o formato escolhido válido — os ids não são
	// estáveis entre duas resoluções do mesmo conteúdo.
	metadata := s.metadataEmCache(ctx.Request.Context(), normalizada)
	if metadata == nil {
		requestCtx, cancelar := context.WithTimeout(ctx.Request.Context(), resolveTimeout)
		defer cancelar()

		lease, _ := s.sessaoPara(requestCtx, req.URL)
		defer lease.Release()

		metadata, normalizada, err = s.mediaRegistry.Metadata(requestCtx, req.URL, media.MetadataOptions{
			CookieFile: arquivoDeSessao(lease),
		})
		if err != nil {
			log.Printf("media: falha ao resolver na criação: %v", err)
			ctx.JSON(statusPara(err), respostaDeErro(ctx, err))
			return db.Download{}, false
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
			return db.Download{}, false
		}
		log.Printf("media: formato %q não está mais disponível; usando o melhor de %s",
			req.FormatID, kind)
		escolhido, _ = escolherFormato(metadata.Formats, "", kind)
	}

	if resposta, excedeu := s.limiteExcedido(ctx, user, escolhido.SizeBytes); excedeu {
		ctx.JSON(http.StatusBadRequest, resposta)
		return db.Download{}, false
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
		TotalBytes:      tamanhoEstimado(metadata.Formats, escolhido, kind),
		// A altura sobrevive ao id: os ids do yt-dlp não são estáveis entre
		// duas extrações, e é ela que permite ao worker cair na resolução mais
		// próxima em vez de falhar.
		FormatHeight: int32(escolhido.Height),
	})
	if err != nil {
		log.Printf("media: falha ao criar download: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao criar o download")))
		return db.Download{}, false
	}

	task, err := tasks.NewDownloadMediaTask(download.ID.String())
	if err == nil {
		_, err = s.queueClient.EnqueueContext(ctx.Request.Context(), task,
			asynq.Queue(queues.TypeDownloadMediaQueue),
			asynq.MaxRetry(queues.MaxTentativasDownload))
	}
	if err != nil {
		log.Printf("media: falha ao enfileirar id=%s: %v", download.ID, err)
		// Sem o job, o download ficaria PENDING para sempre.
		_ = s.store.MarkDownloadFinished(ctx.Request.Context(), db.MarkDownloadFinishedParams{
			ID:           download.ID,
			Status:       db.CoreDownloadStatusFAILED,
			ErrorMessage: pgtype.Text{String: media.PublicMessage(media.ErrDownloadFailed), Valid: true},
			ErrorDetail:  pgtype.Text{String: "falha ao enfileirar a tarefa de download", Valid: true},
		})
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("não foi possível iniciar o download")))
		return db.Download{}, false
	}

	log.Printf("media: download criado id=%s plataforma=%s provider=%s formato=%s",
		download.ID, metadata.Platform, metadata.Provider, escolhido.Label)

	return download, true
}

// cancelDownload marca o pedido e avisa o worker. A marcação no banco é o que
// vale para a tela; a chave no Redis é o que alcança um processo já em
// andamento.
func (s *Server) cancelDownload(ctx *gin.Context) {
	download, ok := s.cancelarDownload(ctx)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"id": download.ID.String(), "status": download.Status})
}

// cancelarDownload faz o cancelamento em si, para as duas superfícies.
//
// O corpo da resposta é de quem chama: a tela quer o par (id, status) e a API
// de integrações devolve o download inteiro, como em todas as outras rotas
// dela. O que NÃO pode divergir é isto aqui — a ordem entre a chave do Redis e
// o UPDATE, que é o que faz o cancelamento alcançar um processo em andamento.
func (s *Server) cancelarDownload(ctx *gin.Context) (db.Download, bool) {
	downloadID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		registrarCodigoErro(ctx, CodeInvalidRequest)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "identificador inválido", "code": CodeInvalidRequest,
		})
		return db.Download{}, false
	}

	userID, autenticado := usuarioAtualID(ctx)
	if !autenticado {
		return db.Download{}, false
	}

	// A chave do Redis vem ANTES do UPDATE, e é gravada mesmo que o banco
	// recuse: ela é o que alcança um processo já em andamento, e o worker
	// checa a cada 2 s. Gravar depois abriria uma janela em que o download
	// consta cancelado no banco e segue baixando em disco.
	if s.redis != nil {
		if err := s.redis.Set(ctx.Request.Context(),
			jobs.ChaveCancelamento(downloadID.String()), "1", cancelTTL).Err(); err != nil {
			log.Printf("media: falha ao sinalizar cancelamento id=%s: %v", downloadID, err)
		}
	}

	// O UPDATE filtra por dono, e é idempotente quanto ao status: cancelar algo
	// que acabou de terminar sozinho não é erro do usuário, é uma corrida que
	// ele não tem como evitar. A resposta devolve o status que de fato vigora.
	download, err := s.store.CancelDownload(ctx.Request.Context(), db.CancelDownloadParams{
		ID: downloadID, UserID: userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Só sobra o caso de o download não existir ou não ser deste
			// usuário — aí é 404 mesmo, e não um conflito.
			registrarCodigoErro(ctx, CodeNotFound)
			ctx.JSON(http.StatusNotFound, gin.H{
				"error": "download não encontrado", "code": CodeNotFound,
			})
			return db.Download{}, false
		}
		registrarCodigoErro(ctx, CodeInternalError)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "falha ao cancelar", "code": CodeInternalError,
		})
		return db.Download{}, false
	}

	// A tela não espera o worker perceber: quem cancelou precisa ver o card
	// mudar na hora. O worker ainda publica o dele ao parar de verdade, e os
	// dois eventos são idênticos.
	if download.Status == db.CoreDownloadStatusCANCELED {
		s.publicarCancelamento(ctx.Request.Context(), download)
	}

	log.Printf("media: cancelamento pedido id=%s por=%s status=%s", downloadID, userID, download.Status)
	return download, true
}

// publicarCancelamento avisa a tela imediatamente. Uma falha aqui não é motivo
// para a requisição falhar: o cancelamento já está gravado, e o próximo
// carregamento da lista mostra o estado certo.
func (s *Server) publicarCancelamento(ctx context.Context, download db.Download) {
	if s.rdStream == nil {
		return
	}
	evento := stream.DownloadEvent{
		ID:            download.ID.String(),
		UserID:        download.UserID.String(),
		Status:        db.CoreDownloadStatusCANCELED,
		Title:         download.Title,
		Platform:      download.Platform,
		PlatformLabel: media.Platform(download.Platform).Label(),
	}
	if err := s.rdStream.Publish(ctx, stream.StreamName, evento); err != nil {
		log.Printf("media: falha ao publicar cancelamento id=%s: %v", download.ID, err)
	}
}

// sessaoUsada descreve o que aconteceu com a sessão gerenciada nesta resolução.
// Existe porque "não foi possível" não diz se a conta que o administrador
// cadastrou chegou a ser usada — e essa é a primeira pergunta quando algo falha
// numa plataforma que exige login.
type sessaoUsada struct {
	Plataforma string
	Necessaria bool
	Conta      string
	Motivo     string
}

// Descricao resume o estado para o painel, em uma linha.
func (s sessaoUsada) Descricao() string {
	switch {
	case !s.Necessaria:
		return "esta plataforma resolve sem conta"
	case s.Conta != "":
		return "usando a conta \"" + s.Conta + "\""
	default:
		return s.Motivo
	}
}

// sessaoPara empresta a sessão gerenciada da plataforma da URL, quando existe.
// A ausência não é erro: a maioria do conteúdo público resolve sem conta, e
// exigir uma quebraria tudo que hoje funciona anônimo.
func (s *Server) sessaoPara(ctx context.Context, bruta string) (*ytaccounts.Lease, sessaoUsada) {
	_, parsed, err := media.NormalizeURL(bruta)
	if err != nil {
		return nil, sessaoUsada{}
	}

	plataforma := string(media.PlatformFor(parsed))
	estado := sessaoUsada{Plataforma: plataforma}

	// Uma plataforma sem perfil de login (Dailymotion, Twitch, SoundCloud) não
	// tem conta para emprestar. PerfilDe cai no padrão quando não conhece a
	// plataforma, então sem esta checagem um link do Dailymotion acabaria
	// pedindo uma sessão e sendo informado de que falta conta "do YouTube".
	if !browser.PlataformaSuportada(plataforma) {
		return nil, estado
	}

	// Só onde resolver anônimo comprovadamente falha. Buscar os cookies pode
	// subir um Chrome headless sobre o perfil, e isso roda dentro da requisição
	// de quem colou o link — por isso o pacote ytaccounts guarda o jar por uma
	// janela curta, e só a primeira resolução da conta paga esse custo.
	if !browser.PerfilDe(plataforma).MetadadosExigemSessao {
		return nil, estado
	}
	estado.Necessaria = true

	if s.accounts == nil {
		estado.Motivo = "gerenciamento de contas indisponível neste ambiente"
		return nil, estado
	}

	lease := ytaccounts.Acquire(ctx, s.accounts, plataforma)
	if lease == nil {
		estado.Motivo = "nenhuma conta autenticada de " +
			browser.PerfilDe(plataforma).Label + " disponível"
		return nil, estado
	}

	estado.Conta = lease.Label
	return lease, estado
}

// arquivoDeSessao evita espalhar checagem de nil por quem monta as opções.
func arquivoDeSessao(lease *ytaccounts.Lease) string {
	if lease == nil {
		return ""
	}
	return lease.CookieFile
}

// tamanhoEstimado soma o que será baixado de verdade.
//
// Acima de 720p o formato de vídeo vem SEM áudio, e o download junta a melhor
// faixa de áudio a ele. Guardar só o tamanho do vídeo dava um denominador curto
// para a barra de progresso, que então travava perto do fim e só destravava com
// a conclusão. Vale como estimativa: o provider corrige com o tamanho real
// conforme baixa.
func tamanhoEstimado(formatos []media.Format, escolhido media.Format, kind media.Kind) int64 {
	total := escolhido.SizeBytes
	if kind != media.KindVideo || escolhido.SizeBytes == 0 {
		return total
	}
	// Um formato que já traz áudio (progressivo) não recebe faixa extra.
	if escolhido.AudioCodec != "" && escolhido.AudioCodec != "none" {
		return total
	}

	for _, formato := range formatos {
		if formato.Kind == media.KindAudio && formato.SizeBytes > 0 {
			// A lista já vem ordenada: o primeiro áudio é o que o "bestaudio"
			// do seletor escolheria.
			return total + formato.SizeBytes
		}
	}
	return total
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

// limiteExcedido aplica o teto de tamanho de arquivo.
//
// São duas origens de limite, e a ordem importa: quando a requisição vem de uma
// INTEGRAÇÃO, vale o teto configurado nela; caso contrário, valem os tetos por
// plano do usuário. Um sistema não tem plano — ele tem contrato —, e cair no
// ramo dos planos daria a ele o limite do plano 'enterprise' da conta de
// serviço, que é nenhum.
func (s *Server) limiteExcedido(ctx *gin.Context, user db.User, tamanho int64) (gin.H, bool) {
	if tamanho <= 0 {
		return nil, false
	}

	if integracao, ehIntegracao := currentIntegration(ctx); ehIntegracao {
		if integracao.MaxFileSizeBytes > 0 && tamanho > integracao.MaxFileSizeBytes {
			registrarCodigoErro(ctx, CodeFileTooLarge)
			return gin.H{
				"error": "o tamanho do arquivo excede o limite desta integração",
				"code":  CodeFileTooLarge,
				"limit": integracao.MaxFileSizeBytes,
				"size":  tamanho,
			}, true
		}
		return nil, false
	}

	if user.Role == db.CoreUserRoleSuperAdmin {
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
			// "error" acompanha "message" para que TODA resposta de erro da API
			// tenha o mesmo par (error, code). A chave antiga fica: a tela já
			// a lê, e removê-la quebraria o aviso de arquivo grande.
			"error":   "O tamanho do arquivo excede o limite do seu plano",
			"code":    "limit_exceeded",
			"message": "O tamanho do arquivo excede o limite do seu plano",
			"limit":   limite,
			"size":    tamanho,
		}, true
	}
	return nil, false
}

// respostaDeErro monta o corpo de erro do download.
//
// O detalhe técnico (o resumo do stderr que o provider capturou) só é anexado
// para o super admin. Para o usuário comum ele não significa nada e expõe
// interno à toa; para quem opera, é a diferença entre "não foi possível
// concluir o download" e saber que a plataforma respondeu 403 ao IP do
// servidor. Sem isso, diagnosticar exige entrar no container e ler log.
func respostaDeErro(ctx *gin.Context, err error) gin.H {
	// O padrão é o mais restrito. Qualquer caminho que esqueça de identificar o
	// usuário cai aqui, e não no ramo que expõe interno.
	corpo := gin.H{
		"error": media.PublicMessage(err),
		"code":  codigoPublico(err),
	}

	user, ok := currentUser(ctx)
	if !ok || user.Role != db.CoreUserRoleSuperAdmin {
		return corpo
	}

	// Só a partir daqui existe informação nossa na resposta.
	corpo["error"] = media.UserMessage(err)
	corpo["code"] = codigoErro(err)
	if detalhe := media.Detail(err); detalhe != "" {
		corpo["detail"] = detalhe
	}
	return corpo
}

// codigoPublico colapsa os códigos que descrevem a NOSSA infraestrutura em um
// só. "blocked" e "provider_unavailable" contam ao cliente final que existe um
// servidor sendo recusado e um mecanismo de download por trás — informação que
// não é dele. Os códigos sobre o conteúdo continuam distintos: são o que
// permite à tela dar uma orientação útil.
func codigoPublico(err error) string {
	switch codigoErro(err) {
	case "blocked", "network", "provider_unavailable":
		return "unavailable"
	default:
		return codigoErro(err)
	}
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
	case errors.Is(err, media.ErrBlocked):
		return http.StatusForbidden
	case errors.Is(err, media.ErrNetwork):
		return http.StatusBadGateway
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
	case errors.Is(err, media.ErrBlocked):
		return "blocked"
	case errors.Is(err, media.ErrNetwork):
		return "network"
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
