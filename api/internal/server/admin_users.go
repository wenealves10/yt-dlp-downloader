package server

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wenealves10/yt-dlp-downloader/internal/db"
	"github.com/wenealves10/yt-dlp-downloader/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

// adminUserResponse é a visão do painel. Nunca carrega o hash da senha: ele não
// tem utilidade nenhuma na tela e vazá-lo permitiria ataque offline.
type adminUserResponse struct {
	ID           uuid.UUID  `json:"id"`
	FullName     string     `json:"full_name"`
	Email        string     `json:"email"`
	PhotoUrl     string     `json:"photo_url,omitempty"`
	Plan         string     `json:"plan"`
	Role         string     `json:"role"`
	DailyLimit   int32      `json:"daily_limit"`
	Active       bool       `json:"active"`
	IsVerified   bool       `json:"is_verified"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
	CreatedAt    *time.Time `json:"created_at"`
	DownloadsAll int64      `json:"downloads_total"`
	StorageBytes int64      `json:"storage_bytes"`
}

func adminUserFromRow(row db.AdminListUsersRow) adminUserResponse {
	response := adminUserResponse{
		ID:           row.ID,
		FullName:     row.FullName,
		Email:        row.Email,
		PhotoUrl:     row.PhotoUrl.String,
		Plan:         string(row.Plan),
		Role:         string(row.Role),
		DailyLimit:   row.DailyLimit,
		Active:       row.Active,
		IsVerified:   row.IsVerified,
		CreatedAt:    row.CreatedAt,
		DownloadsAll: row.DownloadsTotal,
		StorageBytes: row.StorageBytes,
	}
	if row.LastLogin.Valid {
		lastLogin := row.LastLogin.Time
		response.LastLogin = &lastLogin
	}
	return response
}

func adminUserFromModel(user db.User) adminUserResponse {
	response := adminUserResponse{
		ID:         user.ID,
		FullName:   user.FullName,
		Email:      user.Email,
		PhotoUrl:   user.PhotoUrl.String,
		Plan:       string(user.Plan),
		Role:       string(user.Role),
		DailyLimit: user.DailyLimit,
		Active:     user.Active,
		IsVerified: user.IsVerified,
		CreatedAt:  user.CreatedAt,
	}
	if user.LastLogin.Valid {
		lastLogin := user.LastLogin.Time
		response.LastLogin = &lastLogin
	}
	return response
}

// paginacao normaliza os parâmetros de página. Um perPage sem teto deixaria a
// tela derrubar o banco com um único parâmetro na URL.
func paginacao(ctx *gin.Context) (limit, offset, page, perPage int) {
	page, err := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	perPage, err = strconv.Atoi(ctx.DefaultQuery("perPage", "20"))
	if err != nil || perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return perPage, (page - 1) * perPage, page, perPage
}

// filtroTexto devolve o termo de busca; vazio desliga o filtro na consulta.
func filtroTexto(ctx *gin.Context, chave string) pgtype.Text {
	valor := strings.TrimSpace(ctx.Query(chave))
	if valor == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: valor, Valid: true}
}

func filtroPlano(ctx *gin.Context) db.NullCorePlanType {
	switch db.CorePlanType(ctx.Query("plan")) {
	case db.CorePlanTypeFree:
		return db.NullCorePlanType{CorePlanType: db.CorePlanTypeFree, Valid: true}
	case db.CorePlanTypePremium:
		return db.NullCorePlanType{CorePlanType: db.CorePlanTypePremium, Valid: true}
	case db.CorePlanTypeEnterprise:
		return db.NullCorePlanType{CorePlanType: db.CorePlanTypeEnterprise, Valid: true}
	}
	return db.NullCorePlanType{Valid: false}
}

func filtroPapel(ctx *gin.Context) db.NullCoreUserRole {
	switch db.CoreUserRole(ctx.Query("role")) {
	case db.CoreUserRoleUser:
		return db.NullCoreUserRole{CoreUserRole: db.CoreUserRoleUser, Valid: true}
	case db.CoreUserRoleAdmin:
		return db.NullCoreUserRole{CoreUserRole: db.CoreUserRoleAdmin, Valid: true}
	case db.CoreUserRoleSuperAdmin:
		return db.NullCoreUserRole{CoreUserRole: db.CoreUserRoleSuperAdmin, Valid: true}
	}
	return db.NullCoreUserRole{Valid: false}
}

func filtroAtivo(ctx *gin.Context) pgtype.Bool {
	switch ctx.Query("status") {
	case "active":
		return pgtype.Bool{Bool: true, Valid: true}
	case "blocked":
		return pgtype.Bool{Bool: false, Valid: true}
	}
	return pgtype.Bool{Valid: false}
}

func (s *Server) listUsers(ctx *gin.Context) {
	limit, offset, page, perPage := paginacao(ctx)
	search, plan, role, active := filtroTexto(ctx, "search"), filtroPlano(ctx), filtroPapel(ctx), filtroAtivo(ctx)

	total, err := s.store.AdminCountUsers(ctx.Request.Context(), db.AdminCountUsersParams{
		Search: search, Plan: plan, Role: role, Active: active,
	})
	if err != nil {
		log.Printf("admin: falha ao contar usuários: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar usuários")))
		return
	}

	rows, err := s.store.AdminListUsers(ctx.Request.Context(), db.AdminListUsersParams{
		Limit: int32(limit), Offset: int32(offset),
		Search: search, Plan: plan, Role: role, Active: active,
	})
	if err != nil {
		log.Printf("admin: falha ao listar usuários: %v", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao listar usuários")))
		return
	}

	users := make([]adminUserResponse, 0, len(rows))
	for _, row := range rows {
		users = append(users, adminUserFromRow(row))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"users":     users,
		"total":     total,
		"page":      page,
		"per_page":  perPage,
		"next_page": int64(offset+len(rows)) < total,
		"prev_page": page > 1,
	})
}

func (s *Server) getUser(ctx *gin.Context) {
	userID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador inválido")))
		return
	}

	row, err := s.store.AdminGetUserDetail(ctx.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("usuário não encontrado")))
			return
		}
		log.Printf("admin: falha ao carregar usuário %s: %v", userID, err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar usuário")))
		return
	}

	response := adminUserResponse{
		ID: row.ID, FullName: row.FullName, Email: row.Email,
		PhotoUrl: row.PhotoUrl.String, Plan: string(row.Plan), Role: string(row.Role),
		DailyLimit: row.DailyLimit, Active: row.Active, IsVerified: row.IsVerified,
		CreatedAt: row.CreatedAt, DownloadsAll: row.DownloadsTotal, StorageBytes: row.StorageBytes,
	}
	if row.LastLogin.Valid {
		lastLogin := row.LastLogin.Time
		response.LastLogin = &lastLogin
	}

	ctx.JSON(http.StatusOK, gin.H{
		"user":                response,
		"downloads_completed": row.DownloadsCompleted,
		"downloads_failed":    row.DownloadsFailed,
		"transferred_bytes":   row.TransferredBytes,
	})
}

type createUserAdminRequest struct {
	FullName   string `json:"full_name" binding:"required,min=2,max=120"`
	Email      string `json:"email" binding:"required,email"`
	Password   string `json:"password" binding:"omitempty,min=8,max=72"`
	Plan       string `json:"plan" binding:"omitempty,oneof=free premium enterprise"`
	Role       string `json:"role" binding:"omitempty,oneof=user admin super_admin"`
	DailyLimit int32  `json:"daily_limit" binding:"omitempty,min=0,max=100000"`
}

// createUser cadastra pelo painel. Sem senha informada, uma é gerada e devolvida
// UMA ÚNICA VEZ nesta resposta — depois disso só resta redefinir, porque o banco
// guarda apenas o hash.
func (s *Server) createUser(ctx *gin.Context) {
	var req createUserAdminRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	senha := req.Password
	gerada := false
	if senha == "" {
		var err error
		senha, err = utils.GenerateReadablePassword()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao gerar senha")))
			return
		}
		gerada = true
	}

	hashed, err := utils.HashPassword(senha)
	if err != nil {
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("senha muito longa")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao processar a senha")))
		return
	}

	plano := db.CorePlanTypeFree
	if req.Plan != "" {
		plano = db.CorePlanType(req.Plan)
	}
	limite := req.DailyLimit
	if limite == 0 {
		limite = 2
	}

	user, err := s.store.CreateUser(ctx.Request.Context(), db.CreateUserParams{
		ID:                utils.GenerateUUID(),
		FullName:          req.FullName,
		Email:             strings.ToLower(strings.TrimSpace(req.Email)),
		HashedPassword:    hashed,
		Plan:              plano,
		DailyLimit:        limite,
		Active:            true,
		IsVerified:        true,
		PasswordChangedAt: utils.GetCurrentTime(),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New(db.HandleDBError(err))))
		return
	}

	// O papel não entra no CreateUser: a coluna tem default 'user' e promover é
	// um segundo passo, explícito.
	if req.Role != "" && db.CoreUserRole(req.Role) != db.CoreUserRoleUser {
		if updated, err := s.store.AdminUpdateUser(ctx.Request.Context(), db.AdminUpdateUserParams{
			ID:   user.ID,
			Role: db.NullCoreUserRole{CoreUserRole: db.CoreUserRole(req.Role), Valid: true},
		}); err == nil {
			user = updated
		} else {
			log.Printf("admin: usuário criado mas falhou ao definir o papel %s: %v", user.ID, err)
		}
	}

	admin, _ := currentUser(ctx)
	log.Printf("admin: usuário criado user_id=%s por=%s senha_gerada=%t", user.ID, admin.ID, gerada)

	resposta := gin.H{"user": adminUserFromModel(user)}
	if gerada {
		// Só quando o painel gerou: uma senha escolhida pelo administrador não
		// precisa voltar pela rede.
		resposta["generated_password"] = senha
	}
	ctx.JSON(http.StatusCreated, resposta)
}

type updateUserAdminRequest struct {
	FullName   *string `json:"full_name" binding:"omitempty,min=2,max=120"`
	Email      *string `json:"email" binding:"omitempty,email"`
	Plan       *string `json:"plan" binding:"omitempty,oneof=free premium enterprise"`
	Role       *string `json:"role" binding:"omitempty,oneof=user admin super_admin"`
	DailyLimit *int32  `json:"daily_limit" binding:"omitempty,min=0,max=100000"`
	Active     *bool   `json:"active"`
	IsVerified *bool   `json:"is_verified"`
}

func (s *Server) updateUser(ctx *gin.Context) {
	userID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador inválido")))
		return
	}

	var req updateUserAdminRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	alvo, err := s.store.GetUserByID(ctx.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("usuário não encontrado")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar usuário")))
		return
	}

	// Conta de serviço é gerenciada pela seção de integrações, e só por ela.
	// Aqui ela nem aparece na listagem; editá-la por identificador direto
	// permitiria, por exemplo, promovê-la a super admin — uma chave de API com
	// acesso ao painel.
	if ehContaDeServico(ctx, alvo) {
		return
	}

	// Rebaixar ou bloquear o último super admin ativo tranca todo mundo para
	// fora do painel, e não há tela para desfazer isso.
	perdePoder := alvo.Role == db.CoreUserRoleSuperAdmin &&
		((req.Role != nil && db.CoreUserRole(*req.Role) != db.CoreUserRoleSuperAdmin) ||
			(req.Active != nil && !*req.Active))
	if perdePoder {
		if erro := s.garantirOutroSuperAdmin(ctx, alvo); erro != nil {
			ctx.JSON(http.StatusConflict, errorResponse(erro))
			return
		}
	}

	var plano db.NullCorePlanType
	if req.Plan != nil {
		plano = db.NullCorePlanType{CorePlanType: db.CorePlanType(*req.Plan), Valid: true}
	}
	var papel db.NullCoreUserRole
	if req.Role != nil {
		papel = db.NullCoreUserRole{CoreUserRole: db.CoreUserRole(*req.Role), Valid: true}
	}
	var email *string
	if req.Email != nil {
		normalizado := strings.ToLower(strings.TrimSpace(*req.Email))
		email = &normalizado
	}

	user, err := s.store.AdminUpdateUser(ctx.Request.Context(), db.AdminUpdateUserParams{
		ID:         userID,
		FullName:   db.ToPgText(req.FullName),
		Email:      db.ToPgText(email),
		Plan:       plano,
		Role:       papel,
		DailyLimit: db.ToPgInt4(req.DailyLimit),
		Active:     db.ToPgBool(req.Active),
		IsVerified: db.ToPgBool(req.IsVerified),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New(db.HandleDBError(err))))
		return
	}

	admin, _ := currentUser(ctx)
	log.Printf("admin: usuário atualizado user_id=%s por=%s", user.ID, admin.ID)
	ctx.JSON(http.StatusOK, gin.H{"user": adminUserFromModel(user)})
}

type resetPasswordRequest struct {
	Password string `json:"password" binding:"omitempty,min=8,max=72"`
}

// resetUserPassword troca a senha sem exigir a atual: é o administrador agindo
// sobre a conta de outro. A senha só volta na resposta quando foi o painel que a
// gerou.
func (s *Server) resetUserPassword(ctx *gin.Context) {
	userID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador inválido")))
		return
	}

	var req resetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	alvo, err := s.store.GetUserByID(ctx.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("usuário não encontrado")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar usuário")))
		return
	}

	// Uma conta de integração não tem senha para redefinir: quem a autentica é
	// a chave de API. Definir uma aqui criaria justamente o caminho de login
	// que ela não deve ter.
	if ehContaDeServico(ctx, alvo) {
		return
	}

	senha := req.Password
	gerada := false
	if senha == "" {
		if senha, err = utils.GenerateReadablePassword(); err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao gerar senha")))
			return
		}
		gerada = true
	}

	hashed, err := utils.HashPassword(senha)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao processar a senha")))
		return
	}

	if _, err := s.store.AdminUpdateUser(ctx.Request.Context(), db.AdminUpdateUserParams{
		ID:                userID,
		HashedPassword:    db.ToPgText(&hashed),
		PasswordChangedAt: db.ToPgTimestamptz(utils.GetCurrentTime()),
	}); err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao redefinir a senha")))
		return
	}

	admin, _ := currentUser(ctx)
	// A senha em si nunca entra no log.
	log.Printf("admin: senha redefinida user_id=%s por=%s gerada=%t", userID, admin.ID, gerada)

	resposta := gin.H{"message": "senha redefinida"}
	if gerada {
		resposta["generated_password"] = senha
	}
	ctx.JSON(http.StatusOK, resposta)
}

func (s *Server) deleteUser(ctx *gin.Context) {
	userID, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("identificador inválido")))
		return
	}

	admin, _ := currentUser(ctx)
	if admin.ID == userID {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("você não pode remover a própria conta")))
		return
	}

	alvo, err := s.store.GetUserByID(ctx.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("usuário não encontrado")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao carregar usuário")))
		return
	}

	// Remover a conta pela tela de usuários deixaria a integração apontando
	// para uma conta inexistente. A remoção correta é na seção de integrações,
	// que revoga as chaves junto.
	if ehContaDeServico(ctx, alvo) {
		return
	}

	if alvo.Role == db.CoreUserRoleSuperAdmin {
		if erro := s.garantirOutroSuperAdmin(ctx, alvo); erro != nil {
			ctx.JSON(http.StatusConflict, errorResponse(erro))
			return
		}
	}

	if err := s.store.AdminSoftDeleteUser(ctx.Request.Context(), userID); err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(errors.New("falha ao remover usuário")))
		return
	}

	log.Printf("admin: usuário removido user_id=%s por=%s", userID, admin.ID)
	ctx.JSON(http.StatusOK, gin.H{"message": "usuário removido"})
}

// garantirOutroSuperAdmin impede que a plataforma fique sem ninguém capaz de
// entrar no painel.
func (s *Server) garantirOutroSuperAdmin(ctx *gin.Context, alvo db.User) error {
	total, err := s.store.AdminCountSuperAdmins(ctx.Request.Context())
	if err != nil {
		return errors.New("não foi possível conferir os administradores")
	}
	if total <= 1 && alvo.Active {
		return errors.New("este é o único super admin ativo; promova outro antes de alterar este")
	}
	return nil
}

// ehContaDeServico recusa a operação quando o alvo é uma conta de integração, e
// já responde por quem chama.
//
// Está em uma função porque a regra tem de valer nas TRÊS rotas de mutação
// (atualizar, redefinir senha, remover): esquecer uma delas abriria exatamente
// o caminho que a separação entre pessoa e sistema existe para fechar.
func ehContaDeServico(ctx *gin.Context, alvo db.User) bool {
	if alvo.Kind != db.CoreUserKindService {
		return false
	}
	ctx.JSON(http.StatusConflict, errorResponse(errors.New(
		"esta é a conta de uma integração; gerencie-a na seção de integrações")))
	return true
}
