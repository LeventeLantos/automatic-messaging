SHELL := /bin/bash

COMPOSE ?= docker compose
POSTGRES_SVC ?= postgres
REDIS_SVC ?= redis
API_SVC ?= api
API_PORT ?= 8080

DB_NAME ?= messaging
DB_USER ?= postgres

MIGRATION_FILE_IN_CONTAINER ?= /migrations/001_init.sql

PHONE1 ?= +361111111
PHONE2 ?= +362222222
PHONE3 ?= +363333333

MSG1 ?= Hello test message 1
MSG2 ?= Hello test message 2
MSG3 ?= Hello test message 3

ID ?= 1

.PHONY: health up down restart test logs ps psql redis-cli \
        migrate seed drop-schema reset-db nuke-db \
        validate validate-db validate-redis-keys validate-redis-get

health:
	curl -s http://localhost:$(API_PORT)/v1/health || true

up:
	$(COMPOSE) up --build -d

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) restart

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps

psql:
	$(COMPOSE) exec -it $(POSTGRES_SVC) psql -U $(DB_USER) -d $(DB_NAME)

redis-cli:
	$(COMPOSE) exec -it $(REDIS_SVC) redis-cli

migrate:
	$(COMPOSE) exec -T $(POSTGRES_SVC) psql -U $(DB_USER) -d $(DB_NAME) -f $(MIGRATION_FILE_IN_CONTAINER)

seed:
	$(COMPOSE) exec -T $(POSTGRES_SVC) psql -U $(DB_USER) -d $(DB_NAME) -c "\
		INSERT INTO messages (recipient_phone, content) VALUES \
		('$(PHONE1)', '$(MSG1)'), \
		('$(PHONE2)', '$(MSG2)'), \
		('$(PHONE3)', '$(MSG3)'); \
	"

drop-schema:
	$(COMPOSE) exec -T $(POSTGRES_SVC) psql -U $(DB_USER) -d $(DB_NAME) -c "\
		DROP SCHEMA public CASCADE; \
		CREATE SCHEMA public; \
	"

reset-db: drop-schema migrate seed

nuke-db:
	$(COMPOSE) down -v

validate: validate-db validate-redis-keys validate-redis-get

validate-db:
	$(COMPOSE) exec -T $(POSTGRES_SVC) psql -U $(DB_USER) -d $(DB_NAME) -c \
	"SELECT id, status, sent_at, remote_message_id FROM messages ORDER BY id;"

validate-redis-keys:
	$(COMPOSE) exec -T $(REDIS_SVC) redis-cli KEYS 'msg:*'

validate-redis-get:
	$(COMPOSE) exec -T $(REDIS_SVC) redis-cli GET msg:$(ID)

test:
	go test -v ./...
