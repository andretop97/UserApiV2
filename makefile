include .env

migrate_up:
	migrate -path=src/migrations -database "postgresql://${POSTGRESQL_USERNAME}:${POSTGRESQL_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRESQL_DATABASE}?sslmode=disable" -verbose up

migrate_down:
	migrate -path=src/migrations -database "postgresql://${POSTGRESQL_USERNAME}:${POSTGRESQL_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRESQL_DATABASE}?sslmode=disable" -verbose down

migrate_rollback:
	migrate -path=src/migrations -database "postgresql://${POSTGRESQL_USERNAME}:${POSTGRESQL_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRESQL_DATABASE}?sslmode=disable" -verbose down 1

.PHONY: migrate_up migrate_down migrate_rollback
