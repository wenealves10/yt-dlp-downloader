package db

import (
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgconn"
)

type Store interface {
	Querier
}

type SQLStore struct {
	*Queries
}

func NewStore(db DBTX) Store {
	return &SQLStore{
		Queries: New(db),
	}
}

func HandleDBError(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			// unique_violation
			return "Esse e-mail já está em uso"
		case "23503":
			// foreign_key_violation
			return "Registro relacionado não encontrado"
		case "23502":
			// not_null_violation
			return fmt.Sprintf("Campo obrigatório: %s", pgErr.ColumnName)
		default:
			log.Printf("Erro de banco de dados: %s, Código: %s", pgErr.Message, pgErr.Code)
			return "Ocorreu um erro ao processar sua solicitação"
		}
	}
	return "Ocorreu um erro inesperado ao processar sua solicitação"
}
