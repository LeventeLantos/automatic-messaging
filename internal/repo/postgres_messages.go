package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/LeventeLantos/automatic-messaging/internal/model"
)

type PostgresMessageRepo struct {
	db *sql.DB
}

func NewPostgresMessageRepo(db *sql.DB) *PostgresMessageRepo {
	return &PostgresMessageRepo{db: db}
}

func (r *PostgresMessageRepo) ClaimPending(ctx context.Context, limit int) ([]model.Message, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be > 0")
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, recipient_phone, content, status, attempt_count, created_at, updated_at
		FROM messages
		WHERE status = 'pending'
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []model.Message
	for rows.Next() {
		var m model.Message
		var status string
		if err := rows.Scan(
			&m.ID,
			&m.RecipientPhone,
			&m.Content,
			&status,
			&m.AttemptCount,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		m.Status = model.Status(status)
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(msgs) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}

	now := time.Now().UTC()
	for _, m := range msgs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE messages
			SET status = 'processing', updated_at = $2
			WHERE id = $1
		`, m.ID, now); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	for i := range msgs {
		msgs[i].Status = model.Processing
		msgs[i].UpdatedAt = now
	}
	return msgs, nil
}

func (r *PostgresMessageRepo) MarkSent(ctx context.Context, id int64, remoteMessageID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE messages
		SET status = 'sent',
		    sent_at = now(),
		    remote_message_id = $2,
		    updated_at = now()
		WHERE id = $1
	`, id, remoteMessageID)
	return err
}

func (r *PostgresMessageRepo) MarkFailed(ctx context.Context, id int64, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE messages
		SET status = 'failed',
		    attempt_count = attempt_count + 1,
		    last_error = $2,
		    updated_at = now()
		WHERE id = $1
	`, id, reason)
	return err
}

func (r *PostgresMessageRepo) ListSent(ctx context.Context, limit, offset int) ([]model.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, recipient_phone, content, status, attempt_count,
		       last_error, sent_at, remote_message_id, created_at, updated_at
		FROM messages
		WHERE status = 'sent'
		ORDER BY sent_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Message
	for rows.Next() {
		var m model.Message
		var status string
		var lastErr sql.NullString
		var sentAt sql.NullTime
		var remoteID sql.NullString

		if err := rows.Scan(
			&m.ID,
			&m.RecipientPhone,
			&m.Content,
			&status,
			&m.AttemptCount,
			&lastErr,
			&sentAt,
			&remoteID,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, err
		}

		m.Status = model.Status(status)

		if lastErr.Valid {
			s := lastErr.String
			m.LastError = &s
		}
		if sentAt.Valid {
			t := sentAt.Time
			m.SentAt = &t
		}
		if remoteID.Valid {
			s := remoteID.String
			m.RemoteMessageID = &s
		}

		out = append(out, m)
	}
	return out, rows.Err()
}
