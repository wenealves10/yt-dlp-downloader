# Diretórios e variáveis reutilizáveis
MIGRATIONS_DIR=./sql/migrations
DB_URL=$(DB_SOURCE)

# Alvo principal que evita erro ao passar argumentos
%:
	@:

# Criação de migration com nome via argumento
createmigration:
	@if [ -z "$(name)" ]; then \
		echo "Erro: informe o nome da migration com 'make createmigration name=nome_da_migration'"; \
		exit 1; \
	fi
	migrate create -ext=sql -dir=$(MIGRATIONS_DIR) -seq $(name)
	@echo "Migration criada: $(name)"

# Comandos para rodar migrations
migrateup:
	migrate -path=$(MIGRATIONS_DIR) -database "$(DB_URL)" -verbose up

migrateup1:
	migrate -path=$(MIGRATIONS_DIR) -database "$(DB_URL)" -verbose up 1

migratedown:
	migrate -path=$(MIGRATIONS_DIR) -database "$(DB_URL)" -verbose down

migratedown1:
	migrate -path=$(MIGRATIONS_DIR) -database "$(DB_URL)" -verbose down 1

# Geração de código com sqlc
sqlc:
	sqlc generate

# Testes
test:
	go test -v -cover ./...

testnocache:
	go test -count=1 -v -cover ./...

# Mock com mockgen
mock:
	mockgen -source=db/sqlc/store.go -package=mocks -destination=db/mocks/store_mock.go

# Roda Server
run-server:
	go run cmd/server/main.go

build-server:
	go build -o bin/server cmd/server/main.go

run-worker:
	go run cmd/worker/main.go

build-worker:
	go build -o bin/worker cmd/worker/main.go

.PHONY: createmigration migrateup migrateup1 migratedown migratedown1 sqlc test testnocache mock run-server build-server run-worker build-worker
