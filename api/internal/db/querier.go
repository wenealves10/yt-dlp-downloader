// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package db

import (
	"context"

	"github.com/google/uuid"
)

type Querier interface {
	CountDownloadsByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountDownloadsToday(ctx context.Context, userID uuid.UUID) (int64, error)
	CreateDownload(ctx context.Context, arg CreateDownloadParams) (Download, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	DeleteDownload(ctx context.Context, id uuid.UUID) error
	GetDownloadByID(ctx context.Context, id uuid.UUID) (Download, error)
	GetDownloadsByUser(ctx context.Context, arg GetDownloadsByUserParams) ([]Download, error)
	GetDownloadsExpired(ctx context.Context) ([]Download, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (User, error)
	GetUsers(ctx context.Context) ([]User, error)
	UpdateDownload(ctx context.Context, arg UpdateDownloadParams) error
	UpdateDownloadStatus(ctx context.Context, arg UpdateDownloadStatusParams) error
	UpdateUser(ctx context.Context, arg UpdateUserParams) error
	UpdateUserLoginInfo(ctx context.Context, id uuid.UUID) error
}

var _ Querier = (*Queries)(nil)
