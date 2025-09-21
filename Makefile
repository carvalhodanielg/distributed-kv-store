proto_generate:
	protoc 	--go_out=pb --go_opt=paths=source_relative \
       		--go-grpc_out=pb --go-grpc_opt=paths=source_relative \
       		proto/kvstore.proto

run:
	go run server/main.go

populate:
	go run client/main.go --flag="populate"

# Comandos Docker
docker-build:
	docker build -t kvstore:latest .

docker-build-debug:
	docker build -f Dockerfile.debug -t kvstore:debug .

docker-build-client:
	docker build -f Dockerfile.client -t kvstore-client:latest .

docker-run:
	docker run -p 50051:50051 --name kvstore-server kvstore:latest

docker-run-debug:
	docker run -p 50051:50051 --name kvstore-server-debug kvstore:debug

docker-run-client:
	docker run --rm --network host kvstore-client:latest --addr=localhost:50051 --flag="get" --key="test"

docker-compose-up:
	docker-compose up -d

docker-compose-down:
	docker-compose down

docker-compose-logs:
	docker-compose logs -f

# Comandos de desenvolvimento
dev-setup:
	go mod tidy
	make proto_generate

# Comandos de teste
test:
	go test ./...

test-coverage:
	go test -cover ./...

# Limpeza
clean:
	docker-compose down
	docker rmi kvstore:latest kvstore-client:latest kvstore:debug 2>/dev/null || true
	go clean

# Debug
debug:
	chmod +x debug.sh && ./debug.sh