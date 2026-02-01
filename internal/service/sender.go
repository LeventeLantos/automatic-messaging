package service

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/LeventeLantos/automatic-messaging/internal/model"
)

type SendClient interface {
	Send(ctx context.Context, phoneNumber, message string) (remoteMessageID string, err error)
}

type Sender struct {
	client     SendClient
	contentMax int

	onSent   func(ctx context.Context, internalID int64, remoteMessageID string) error
	onFailed func(ctx context.Context, internalID int64, reason string) error
}

func NewSender(client SendClient, contentMax int) *Sender {
	return &Sender{
		client:     client,
		contentMax: contentMax,
	}
}

func (s *Sender) WithHooks(
	onSent func(ctx context.Context, internalID int64, remoteMessageID string) error,
	onFailed func(ctx context.Context, internalID int64, reason string) error,
) *Sender {
	s.onSent = onSent
	s.onFailed = onFailed
	return s
}

func (s *Sender) ProcessBatch(ctx context.Context, msgs []model.Message) (sent int, failed int) {
	for _, m := range msgs {
		if utf8.RuneCountInString(m.Content) > s.contentMax {
			failed++
			s.fail(ctx, m.ID, fmt.Sprintf("content exceeds %d chars", s.contentMax))
			continue
		}

		remoteID, err := s.client.Send(ctx, m.RecipientPhone, m.Content)
		if err != nil {
			failed++
			s.fail(ctx, m.ID, err.Error())
			continue
		}

		sent++
		if s.onSent != nil {
			_ = s.onSent(ctx, m.ID, remoteID)
		}
	}
	return sent, failed
}

func (s *Sender) fail(ctx context.Context, id int64, reason string) {
	if s.onFailed != nil {
		_ = s.onFailed(ctx, id, reason)
	}
}
