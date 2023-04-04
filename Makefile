.PHONY: gen-proto
gen-proto:
	protoc --go_out=. --go_opt=paths=source_relative \
        --go-grpc_out=. --go-grpc_opt=paths=source_relative \
        api/chat/chat.proto

.PHONY: run
run:
	go run cmd/chat/main.go

.PHONY: run-client
run-client:
	go run clients/app/cmd/main.go

.PHONY: up
up:
	docker-compose up --build
