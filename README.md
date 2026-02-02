# Automatic Messaging Service

This project implements an automatic message sending system written in **Go**.
The service periodically sends unsent messages from a PostgreSQL database to a webhook endpoint, tracks delivery status, and exposes a small HTTP API to control and inspect the system.

---

## Features

* Automatic message sending every N seconds (default: 2 minutes)
* Sends 2 unsent messages per interval
* Messages are never resent once sent
* New records are picked up automatically
* Scheduler start/stop via API
* List sent messages via API
* Redis cache for sent message IDs
* OpenAPI documentation
* Docker-first local setup

---

## Architecture Overview

* Scheduler: In-memory ticker (no cron, no OS dependencies)
* Service layer: Message validation + webhook sending
* Repository: PostgreSQL with row-level locking
* Cache: Redis stores `{messageId, sentAt}`
* API: net/http (stdlib only)
* Logging: Go `log/slog`

---

## Requirements

> Docker-first setup

* Docker + Docker Compose
* Go 1.25 (only needed if running without Docker)

PostgreSQL and Redis are provided via Docker Compose.

---

## Configuration

Copy the example environment file and set the webhook URL:

```bash
cp .env.example .env
```

Edit `.env` and set:

* `WEBHOOK_URL`

---

## Running the application

Start all services:

```bash
make up
```

The scheduler starts automatically when the application starts.

---

## Testing / Validation

All validation is handled via the Makefile.

### Full end-to-end testing

```bash
make reset-db
```

This will:

1. Drop and recreate the database schema
2. Run the initial migration
3. Insert test messages

Wait for one or two scheduler intervals (default: 2 minutes), then run:

```bash
make validate
```

This command verifies:

* Sent messages in PostgreSQL
* Cached message IDs in Redis

---

### Health check

You can also verify the API is up:

```bash
make health
```

Expected output:

```
{"ok":true}
```
