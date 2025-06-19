createmigration:
	migrate create -ext=sql -dir=./sql/migrations -seq $(word 2,$(MAKECMDGOALS))
	@echo "Migration created: $(word 2,$(MAKECMDGOALS))"
%:
	@:

migrateup:
	migrate -path=./sql/migrations -database "$(DB_SOURCE)" -verbose up

migrateup1:
	migrate -path=./sql/migrations -database "$(DB_SOURCE)" -verbose up 1

migratedown:
	migrate -path=./sql/migrations -database "$(DB_SOURCE)" -verbose down

migratedown1:
	migrate -path=./sql/migrations -database "$(DB_SOURCE)" -verbose down 1

dev:
	go run main.go

build:
	go build .

sqlc:
	sqlc generate

test:
	go test -v -cover ./...

testnocache:
	go test -count=1 -v -cover ./...

server:
	go run main.go

mock:
	mockgen -source=db/sqlc/store.go -package=mocks -destination=db/mocks/store_mock.go

.PHONY: createmigration migrateup migratedown dev build sqlc test server mock migrateup1 migratedown1 testnocache