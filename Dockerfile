# Multi-stage build para otimizar o tamanho da imagem final
FROM golang:1.25.1-alpine AS builder

# Instalar dependências necessárias
RUN apk add --no-cache protobuf-dev protoc

# Instalar plugins do protoc
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Definir diretório de trabalho
WORKDIR /app

# Copiar arquivos de dependências
COPY go.mod go.sum ./

# Baixar dependências
RUN go mod download

# Copiar código fonte
COPY . .

# Gerar código protobuf
RUN make proto_generate

# Build do servidor
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./server/main.go

# Build do cliente
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o client ./client/main.go

# Imagem final minimalista
FROM alpine:latest

# Instalar ca-certificates para HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copiar binários do builder
COPY --from=builder /app/server .
COPY --from=builder /app/client .

# Expor porta do servidor
EXPOSE 50051

# Comando padrão: executar servidor
CMD ["./server"]
