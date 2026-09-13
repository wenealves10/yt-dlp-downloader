package server

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/integrations"
)

const (
	// chaveIntegracao guarda a integração autenticada. É definida assim que a
	// chave é reconhecida — ANTES das checagens de acesso — porque a auditoria
	// precisa poder atribuir a alguém uma tentativa que foi recusada por IP ou
	// por chave revogada. Sem isso, justamente as requisições suspeitas seriam
	// as que não apareceriam no painel.
	chaveIntegracao = "integration"

	// chaveIntegracaoAPIKey guarda qual chave da integração foi usada.
	chaveIntegracaoAPIKey = "integration_api_key_id"

	// Headers de autenticação aceitos.
	headerAPIKey        = "X-API-Key"
	headerIntegrationID = "X-Integration-Id"
)

// currentIntegration devolve a integração autenticada na requisição.
func currentIntegration(ctx *gin.Context) (db.Integration, bool) {
	valor, existe := ctx.Get(chaveIntegracao)
	if !existe {
		return db.Integration{}, false
	}
	integracao, ok := valor.(db.Integration)
	return integracao, ok
}

// currentIntegrationKeyID devolve a chave usada na requisição.
func currentIntegrationKeyID(ctx *gin.Context) (uuid.UUID, bool) {
	valor, existe := ctx.Get(chaveIntegracaoAPIKey)
	if !existe {
		return uuid.Nil, false
	}
	id, ok := valor.(uuid.UUID)
	return id, ok
}

// extrairChaveAPI aceita dois formatos, e os dois de propósito.
//
// `Authorization: Bearer` é o que todo cliente HTTP já sabe fazer e o que
// qualquer biblioteca de API espera. `X-API-Key` existe porque alguns
// ambientes (proxy corporativo, gateway, ferramenta low-code) removem ou
// reescrevem o header Authorization, e nesses casos a integração ficaria
// impossível sem uma segunda porta.
//
// A ambiguidade que isso poderia criar não existe: estas rotas vivem sob
// /v1/integration e lá um Bearer NUNCA é token de usuário.
func extrairChaveAPI(ctx *gin.Context) string {
	if chave := strings.TrimSpace(ctx.GetHeader(headerAPIKey)); chave != "" {
		return chave
	}

	cabecalho := strings.TrimSpace(ctx.GetHeader(authorizationHeaderKey))
	if cabecalho == "" {
		return ""
	}
	campos := strings.Fields(cabecalho)
	if len(campos) == 2 && strings.EqualFold(campos[0], authorizationTypeBearer) {
		return campos[1]
	}
	// Uma chave colada sem o "Bearer" é erro comum de integração e não custa
	// nada aceitar: ela é reconhecível pelo prefixo.
	if len(campos) == 1 && integrations.LooksLikeKey(campos[0]) {
		return campos[0]
	}
	return ""
}

// integrationAuthMiddleware autentica o sistema e aplica o controle de acesso.
//
// A ordem das checagens é deliberada, da mais barata para a mais cara: forma da
// chave (sem I/O), consulta indexada pelo hash, estado da chave, estado da
// integração, IP de origem e só então o limitador. Inverter isso faria uma
// rajada de chaves inválidas custar um INCR no Redis por tentativa.
func (s *Server) integrationAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		chave := extrairChaveAPI(ctx)
		if chave == "" {
			erroIntegracao(ctx, http.StatusUnauthorized, CodeMissingCredentials,
				"informe a chave de API no header Authorization: Bearer <chave> ou X-API-Key")
			return
		}

		// Filtro de forma antes de tocar o banco. Sem ele, qualquer token de
		// outro sistema jogado neste endpoint viraria uma consulta ao Postgres.
		if !integrations.LooksLikeKey(chave) {
			erroIntegracao(ctx, http.StatusUnauthorized, CodeInvalidAPIKey,
				"chave de API inválida")
			return
		}

		linha, err := s.store.GetIntegrationKeyByHash(ctx.Request.Context(),
			integrations.HashKey(chave))
		if err != nil {
			// Mensagem idêntica para chave inexistente e para erro de banco:
			// distinguir os dois contaria a quem está tentando adivinhar se o
			// valor testado chegou perto de existir.
			erroIntegracao(ctx, http.StatusUnauthorized, CodeInvalidAPIKey,
				"chave de API inválida")
			return
		}

		// A partir daqui a requisição tem dono, e toda recusa fica registrada
		// na conta dele.
		ctx.Set(chaveIntegracao, linha.Integration)
		ctx.Set(chaveIntegracaoAPIKey, linha.KeyID)

		if linha.KeyRevokedAt.Valid {
			erroIntegracao(ctx, http.StatusUnauthorized, CodeKeyRevoked,
				"esta chave de API foi revogada")
			return
		}
		if linha.KeyExpiresAt.Valid && !linha.KeyExpiresAt.Time.After(time.Now()) {
			erroIntegracao(ctx, http.StatusUnauthorized, CodeKeyExpired,
				"esta chave de API expirou")
			return
		}

		// O identificador público é opcional, mas se vier tem de bater. É uma
		// trava contra o acidente clássico de configuração: a chave de
		// homologação apontada para a integração de produção.
		if informado := strings.TrimSpace(ctx.GetHeader(headerIntegrationID)); informado != "" {
			if !strings.EqualFold(informado, linha.Integration.ID.String()) {
				erroIntegracao(ctx, http.StatusUnauthorized, CodeIntegrationMismatch,
					"a chave de API não pertence à integração informada em "+headerIntegrationID)
				return
			}
		}

		if linha.Integration.DeletedAt.Valid {
			erroIntegracao(ctx, http.StatusForbidden, CodeIntegrationDisabled,
				"esta integração foi removida")
			return
		}
		if !linha.Integration.Active {
			erroIntegracao(ctx, http.StatusForbidden, CodeIntegrationDisabled,
				"esta integração está desativada")
			return
		}
		// A conta de serviço desativada também barra: é o mesmo interruptor que
		// vale para uma conta de pessoa, e deixá-lo de fora criaria um estado em
		// que a conta consta bloqueada e continua baixando.
		if !linha.User.Active || linha.User.DeletedAt.Valid {
			erroIntegracao(ctx, http.StatusForbidden, CodeIntegrationDisabled,
				"a conta desta integração está desativada")
			return
		}

		ipCliente := ctx.ClientIP()
		if !integrations.IPAutorizado(linha.Integration.AllowedIps, ipCliente) {
			// O IP recusado vai para o log do processo E para a auditoria: é o
			// sinal mais direto de chave vazada, e quem opera precisa ver de
			// onde veio sem depender de o cliente reclamar primeiro.
			erroIntegracao(ctx, http.StatusForbidden, CodeIPNotAllowed,
				"este endereço IP não está autorizado para esta integração")
			return
		}

		// O limite é por CHAVE, e não por integração: quando uma integração tem
		// várias chaves (um serviço por ambiente, por exemplo), o consumo
		// descontrolado de uma não deve derrubar as outras.
		decisao := s.limiter.Allow(ctx.Request.Context(),
			linha.KeyID.String(), int(linha.Integration.RateLimitPerMinute))

		ctx.Header("X-RateLimit-Limit", strconv.Itoa(decisao.Limite))
		ctx.Header("X-RateLimit-Remaining", strconv.Itoa(decisao.Restante))

		if !decisao.Permitido {
			segundos := int(decisao.Espera.Seconds())
			if segundos < 1 {
				segundos = 1
			}
			// Retry-After transforma um 429 em instrução. Sem ele, a reação
			// natural do cliente é tentar de novo na hora, o que só agrava.
			ctx.Header("Retry-After", strconv.Itoa(segundos))
			erroIntegracao(ctx, http.StatusTooManyRequests, CodeRateLimited,
				"limite de chamadas por minuto excedido; aguarde "+strconv.Itoa(segundos)+"s")
			return
		}

		// A conta de serviço entra no contexto pela MESMA chave usada pelo
		// login de usuário. É o que faz os handlers de download funcionarem
		// aqui sem nenhuma adaptação.
		ctx.Set(authorizationUserKey, linha.User)

		// O carimbo de uso é escrito no máximo uma vez por minuto por chave.
		if s.limiter.DeveRegistrarUso(ctx.Request.Context(), linha.KeyID.String()) {
			if err := s.store.TouchIntegrationAPIKey(ctx.Request.Context(), db.TouchIntegrationAPIKeyParams{
				ID:         linha.KeyID,
				LastUsedIp: pgtype.Text{String: ipCliente, Valid: ipCliente != ""},
			}); err != nil {
				// Perder o carimbo não justifica recusar a requisição.
				log.Printf("integracoes: falha ao registrar uso da chave %s: %v", linha.KeyID, err)
			}
		}

		ctx.Next()
	}
}

// integrationQuotaMiddleware aplica os dois tetos que protegem a plataforma de
// um cliente entusiasmado: quantos downloads por dia e quantos ao mesmo tempo.
//
// Só entra nas rotas que CRIAM download. Aplicá-lo na listagem faria um cliente
// que já esgotou a cota perder também a capacidade de consultar o que pediu
// antes — e aí ele passaria a repetir os pedidos, que é o oposto do objetivo.
func (s *Server) integrationQuotaMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		integracao, ok := currentIntegration(ctx)
		if !ok {
			erroIntegracao(ctx, http.StatusUnauthorized, CodeMissingCredentials,
				"requisição não autenticada")
			return
		}

		if integracao.DailyLimit > 0 {
			usados, err := s.store.CountIntegrationDownloadsToday(ctx.Request.Context(), integracao.UserID)
			if err != nil {
				log.Printf("integracoes: falha ao contar cota de %s: %v", integracao.ID, err)
				erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError,
					"não foi possível verificar a cota")
				return
			}
			if usados >= int64(integracao.DailyLimit) {
				// O reset é à meia-noite de São Paulo, o mesmo corte da
				// contagem. Dizer QUANDO libera evita o cliente ficar tentando.
				ctx.Header("X-Quota-Limit", strconv.Itoa(int(integracao.DailyLimit)))
				ctx.Header("X-Quota-Remaining", "0")
				ctx.Header("X-Quota-Reset", proximoResetCota().Format(time.RFC3339))
				erroIntegracao(ctx, http.StatusTooManyRequests, CodeQuotaExceeded,
					"cota diária de downloads esgotada")
				return
			}
			ctx.Header("X-Quota-Limit", strconv.Itoa(int(integracao.DailyLimit)))
			ctx.Header("X-Quota-Remaining", strconv.FormatInt(int64(integracao.DailyLimit)-usados, 10))
			ctx.Header("X-Quota-Reset", proximoResetCota().Format(time.RFC3339))
		}

		if integracao.MaxConcurrentDownloads > 0 {
			emAndamento, err := s.store.CountIntegrationActiveDownloads(ctx.Request.Context(), integracao.UserID)
			if err != nil {
				log.Printf("integracoes: falha ao contar simultâneos de %s: %v", integracao.ID, err)
				erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError,
					"não foi possível verificar os downloads em andamento")
				return
			}
			if emAndamento >= int64(integracao.MaxConcurrentDownloads) {
				// Diferente da cota, isto se resolve sozinho em minutos: o
				// Retry-After diz para o cliente voltar em vez de desistir.
				ctx.Header("Retry-After", "30")
				erroIntegracao(ctx, http.StatusTooManyRequests, CodeTooManyConcurrent,
					"limite de downloads simultâneos atingido; aguarde a conclusão dos atuais")
				return
			}
		}

		ctx.Next()
	}
}

// proximoResetCota devolve a próxima meia-noite no fuso usado pela contagem.
//
// O fuso é o de São Paulo, e não UTC, pelo mesmo motivo do contador do usuário
// comum: o "dia" da cota tem de ser o dia de quem usa. Em UTC, a cota viraria
// às 21h e o cliente veria o limite resetar no meio da tarde.
func proximoResetCota() time.Time {
	local, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		// Imagem sem tzdata: UTC é uma resposta imperfeita, mas melhor do que
		// nenhuma — e o erro não deve derrubar a requisição.
		local = time.UTC
	}
	agora := time.Now().In(local)
	return time.Date(agora.Year(), agora.Month(), agora.Day()+1, 0, 0, 0, 0, local)
}

// integrationAuditMiddleware registra a requisição depois de respondida.
//
// É o middleware MAIS EXTERNO do grupo, e tem de ser: assim ele também enxerga
// as requisições recusadas pela autenticação, que são exatamente as que
// interessam quando se investiga uso indevido de uma chave.
func (s *Server) integrationAuditMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		inicio := time.Now()

		ctx.Next()

		integracao, identificada := currentIntegration(ctx)
		if !identificada {
			// Chave desconhecida: não há a quem atribuir a linha, e inventar um
			// dono violaria a chave estrangeira. Fica no log do processo.
			if ctx.Writer.Status() == http.StatusUnauthorized {
				log.Printf("integracoes: tentativa com credencial não reconhecida ip=%s rota=%s",
					ctx.ClientIP(), ctx.Request.URL.Path)
			}
			return
		}

		status := ctx.Writer.Status()
		codigo := codigoRespondido(ctx)
		if codigo == "" && status >= 400 {
			codigo = codigoPadraoDoStatus(status)
		}

		var chaveID pgtype.UUID
		if id, ok := currentIntegrationKeyID(ctx); ok {
			chaveID = pgtype.UUID{Bytes: id, Valid: true}
		}

		// O download é gravado em coluna própria, e não só no caminho: é o que
		// permite ao painel mostrar "todas as chamadas sobre este download".
		var downloadID pgtype.UUID
		if bruto := ctx.Param("id"); bruto != "" {
			if id, err := uuid.Parse(bruto); err == nil {
				downloadID = pgtype.UUID{Bytes: id, Valid: true}
			}
		}

		var codigoTexto pgtype.Text
		if codigo != "" {
			codigoTexto = pgtype.Text{String: codigo, Valid: true}
		}

		s.auditor.registrar(db.CreateIntegrationRequestParams{
			IntegrationID: integracao.ID,
			ApiKeyID:      chaveID,
			Method:        ctx.Request.Method,
			Path:          truncar(ctx.Request.URL.Path, 300),
			StatusCode:    int32(status),
			Ip:            ctx.ClientIP(),
			UserAgent:     truncar(ctx.Request.UserAgent(), 300),
			DurationMs:    int32(time.Since(inicio).Milliseconds()),
			ErrorCode:     codigoTexto,
			DownloadID:    downloadID,
		})
	}
}

// truncar limita o texto gravado. Path e user agent vêm do cliente, e sem teto
// uma requisição com uma URL de 8 KB viraria uma linha de 8 KB no banco.
func truncar(valor string, limite int) string {
	if len(valor) <= limite {
		return valor
	}
	return valor[:limite]
}

// integrationRecovery responde 500 em JSON quando um handler entra em pânico.
//
// O gin.Recovery global responde um corpo vazio, e nesta superfície isso
// deixaria o cliente sem `code` — a única resposta da API que ele não
// conseguiria tratar de forma programática.
func integrationRecovery() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(ctx *gin.Context, recuperado any) {
		log.Printf("integracoes: pânico em %s %s: %v",
			ctx.Request.Method, ctx.Request.URL.Path, recuperado)
		erroIntegracao(ctx, http.StatusInternalServerError, CodeInternalError,
			"erro interno ao processar a requisição")
	})
}

// erroNaoEncontrado é a resposta de rota inexistente sob /v1/integration.
func erroNaoEncontrado(ctx *gin.Context) {
	erroIntegracao(ctx, http.StatusNotFound, CodeNotFound,
		"rota não encontrada; consulte a documentação em /docs")
}
