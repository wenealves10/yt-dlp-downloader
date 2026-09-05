package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func ToPgText(ptr *string) pgtype.Text {
	if ptr != nil {
		return pgtype.Text{String: *ptr, Valid: true}
	}
	return pgtype.Text{Valid: false}
}

func ToPgBool(ptr *bool) pgtype.Bool {
	if ptr != nil {
		return pgtype.Bool{Bool: *ptr, Valid: true}
	}
	return pgtype.Bool{Valid: false}
}

func ToPgInt4(ptr *int32) pgtype.Int4 {
	if ptr != nil {
		return pgtype.Int4{Int32: *ptr, Valid: true}
	}
	return pgtype.Int4{Valid: false}
}

func ToPgTimestamptz(ptr *time.Time) pgtype.Timestamptz {
	if ptr != nil {
		return pgtype.Timestamptz{Time: *ptr, Valid: true}
	}
	return pgtype.Timestamptz{Valid: false}
}

func ToCorePlanType(plan *CorePlanType) NullCorePlanType {
	if plan != nil {
		return NullCorePlanType{CorePlanType: *plan, Valid: true}
	}
	return NullCorePlanType{Valid: false}
}

func ToPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil}
}

func ToCoreYoutubeAccountStatus(status *CoreYoutubeAccountStatus) NullCoreYoutubeAccountStatus {
	if status != nil {
		return NullCoreYoutubeAccountStatus{CoreYoutubeAccountStatus: *status, Valid: true}
	}
	return NullCoreYoutubeAccountStatus{Valid: false}
}
