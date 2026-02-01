package model

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
}
