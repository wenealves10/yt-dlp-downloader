package jobs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/browser"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
)

// perAccountCheckTimeout limita o custo de uma conta problemática: o health
// check inteiro não pode travar por causa de um perfil corrompido.
const perAccountCheckTimeout = 2 * time.Minute

// JobYoutubeHealthCheck confere as sessões das contas gerenciadas. É leve por
// construção: para cada conta, o serviço de navegador lê os cookies do perfil e
// faz no máximo uma requisição ao YouTube.
type JobYoutubeHealthCheck struct {
	store   db.Store
	browser *browser.Client
}

func NewJobYoutubeHealthCheck(store db.Store, browserClient *browser.Client) *JobYoutubeHealthCheck {
	return &JobYoutubeHealthCheck{store: store, browser: browserClient}
}

func (p *JobYoutubeHealthCheck) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	if !p.browser.Configured() {
		return nil
	}

	accounts, err := p.store.GetYoutubeAccountsForHealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("failed to list youtube accounts: %v", err)
	}
	if len(accounts) == 0 {
		return nil
	}

	for _, account := range accounts {
		p.checkAccount(ctx, account)
	}

	return nil
}

func (p *JobYoutubeHealthCheck) checkAccount(ctx context.Context, account db.YoutubeAccount) {
	checkCtx, cancel := context.WithTimeout(ctx, perAccountCheckTimeout)
	defer cancel()

	result, err := p.browser.Check(checkCtx, account.ID)
	if err != nil {
		// Falha de infraestrutura não é sessão expirada: o estado vira ERROR e
		// o administrador não recebe um pedido de login desnecessário.
		log.Printf("jobs: health check falhou account_id=%s label=%s erro=%v", account.ID, account.Label, err)
		p.applyStatus(ctx, account, db.CoreYoutubeAccountStatusERROR, browser.UserMessage(err), "")
		return
	}

	if !result.Authenticated {
		log.Printf("jobs: sessão inválida account_id=%s label=%s motivo=%s", account.ID, account.Label, result.Reason)
		p.applyStatus(ctx, account, db.CoreYoutubeAccountStatusREQUIRESAUTH, result.Reason, "")
		return
	}

	log.Printf("jobs: sessão válida account_id=%s label=%s", account.ID, account.Label)
	p.applyStatus(ctx, account, db.CoreYoutubeAccountStatusAUTHENTICATED, "", result.Email)
}

// applyStatus grava o veredito. O e-mail descoberto é apenas identificação
// visual no painel; nenhum dado de sessão é persistido.
func (p *JobYoutubeHealthCheck) applyStatus(ctx context.Context, account db.YoutubeAccount, status db.CoreYoutubeAccountStatus, reason, email string) {
	lastError := pgtype.Text{Valid: false}
	if reason != "" {
		lastError = pgtype.Text{String: truncateReason(reason), Valid: true}
	}

	if _, err := p.store.UpdateYoutubeAccountStatus(ctx, db.UpdateYoutubeAccountStatusParams{
		ID:        account.ID,
		Status:    status,
		LastError: lastError,
	}); err != nil {
		log.Printf("jobs: falha ao gravar status da conta account_id=%s: %v", account.ID, err)
		return
	}

	if email == "" || account.Email.String == email {
		return
	}
	if _, err := p.store.UpdateYoutubeAccount(ctx, db.UpdateYoutubeAccountParams{
		ID:    account.ID,
		Email: pgtype.Text{String: email, Valid: true},
	}); err != nil {
		log.Printf("jobs: falha ao gravar e-mail da conta account_id=%s: %v", account.ID, err)
	}
}

func truncateReason(reason string) string {
	const limit = 500
	if len(reason) <= limit {
		return reason
	}
	return reason[:limit] + "…"
}
