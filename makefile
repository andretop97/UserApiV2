include .env

migrate_up:
	migrate -path=src/migrations -database "postgresql://${POSTGRESQL_USERNAME}:${POSTGRESQL_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRESQL_DATABASE}?sslmode=disable" -verbose up

migrate_down:
	migrate -path=src/migrations -database "postgresql://${POSTGRESQL_USERNAME}:${POSTGRESQL_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRESQL_DATABASE}?sslmode=disable" -verbose down

migrate_rollback:
	migrate -path=src/migrations -database "postgresql://${POSTGRESQL_USERNAME}:${POSTGRESQL_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRESQL_DATABASE}?sslmode=disable" -verbose down 1

infra_up:
	docker compose up -d --wait postgres redis

infra_down:
	docker compose down

test:
	go test ./...

# No Docker Desktop do Windows o testcontainers resolve o socket como named pipe e o Ryuk não sobe.
# /var/run/docker.sock é o caminho dentro da VM do Docker (e o padrão no Linux/Mac/CI).
# MSYS_NO_PATHCONV impede que o sh do Git converta o caminho para C:/Program Files/Git/...
test_integration: export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE ?= /var/run/docker.sock
test_integration: export MSYS_NO_PATHCONV := 1
test_integration:
	go test -tags integration -count=1 ./...

.PHONY: migrate_up migrate_down migrate_rollback infra_up infra_down test test_integration
