DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'message_status') THEN
CREATE TYPE message_status AS ENUM ('pending', 'processing', 'sent', 'failed');
END IF;
END
$$;

CREATE TABLE IF NOT EXISTS messages (
                                        id                BIGSERIAL PRIMARY KEY,
                                        recipient_phone   TEXT NOT NULL,
                                        content           TEXT NOT NULL,
                                        status            message_status NOT NULL DEFAULT 'pending',
                                        attempt_count     INT NOT NULL DEFAULT 0,

                                        last_error        TEXT,
                                        sent_at           TIMESTAMPTZ,
                                        remote_message_id UUID,

                                        created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT content_len_chk CHECK (char_length(content) <= 160)
    );

CREATE INDEX IF NOT EXISTS idx_messages_status_created
    ON messages(status, created_at);

CREATE INDEX IF NOT EXISTS idx_messages_sent_at
    ON messages(sent_at DESC);
