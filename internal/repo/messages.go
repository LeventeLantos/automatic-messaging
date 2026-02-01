package repo

import (
	"context"

	"github.com/LeventeLantos/automatic-messaging/internal/model"
)

type MessageRepository interface {
	ClaimPending(ctx context.Context, limit int) ([]model.Message, error)
	MarkSent(ctx context.Context, id int64, remoteMessageID string) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
	ListSent(ctx context.Context, limit, offset int) ([]model.Message, error)
}
