package model

import "time"

type Status string

const (
	Pending    Status = "pending"
	Processing Status = "processing"
	Sent       Status = "sent"
	Failed     Status = "failed"
)

type Message struct {
	ID             int64
	RecipientPhone string
	Content        string
	Status         Status

	AttemptCount    int
	LastError       *string
	SentAt          *time.Time
	RemoteMessageID *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
