package cache

import (
	"context"
	"time"
)

type MessageCache interface {
	StoreSent(ctx context.Context, internalID int64, remoteMessageID string, sentAt time.Time) error
}
