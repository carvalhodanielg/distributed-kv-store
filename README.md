# 🗄️ Distributed Key-Value Store

Um sistema de armazenamento chave-valor distribuído construído com Go e gRPC, oferecendo operações CRUD básicas e funcionalidade de watch em tempo real. Totalmente containerizado para fácil reprodução e deploy.

## 📋 Índice

- [Sobre o Projeto](#sobre-o-projeto)
- [Stack Tecnológica](#stack-tecnológica)
- [Arquitetura](#arquitetura)
- [Funcionalidades](#funcionalidades)
- [Pré-requisitos](#pré-requisitos)
- [🚀 Quick Start](#-quick-start)
- [🐳 Docker](#-docker)
- [💻 Desenvolvimento Local](#-desenvolvimento-local)
- [📚 API Reference](#-api-reference)
- [🧪 Testes](#-testes)
- [📁 Estrutura do Projeto](#-estrutura-do-projeto)
- [🗺️ Roadmap](#️-roadmap)
- [🤝 Contribuição](#-contribuição)

## 🎯 Sobre o Projeto

Este projeto implementa um sistema de armazenamento chave-valor distribuído que permite operações básicas de persistência de dados através de uma API gRPC. O sistema foi desenvolvido com foco em simplicidade, performance e extensibilidade.

### Principais Características

- **🔒 Thread-safe**: Implementação com mutex para operações concorrentes
- **⚡ Real-time notifications**: Sistema de watch para monitorar mudanças em chaves específicas
- **🚀 gRPC**: Comunicação eficiente entre cliente e servidor
- **📦 Protocol Buffers**: Serialização otimizada de dados
- **🖥️ CLI Client**: Interface de linha de comando para interação
- **🐳 Containerizado**: Fácil deploy e reprodução com Docker
- **🛠️ Multi-stage builds**: Imagens Docker otimizadas

## 🛠️ Stack Tecnológica

- **Linguagem**: Go 1.25.1
- **Comunicação**: gRPC
- **Serialização**: Protocol Buffers
- **Concorrência**: Goroutines e Channels
- **Sincronização**: sync.RWMutex
- **Build**: Makefile
- **Containerização**: Docker & Docker Compose
- **Base**: Alpine Linux (imagens finais)

### Dependências Principais

```go
google.golang.org/grpc v1.75.1
google.golang.org/protobuf v1.36.9
```

## 🏗️ Arquitetura

```
┌─────────────────┐    gRPC     ┌─────────────────┐
│   CLI Client    │◄──────────►│   gRPC Server    │
│                 │             │                 │
│  - Put/Get      │             │  - KvStore      │
│  - Delete       │             │  - Watchers     │
│  - Watch        │             │  - Thread-safe  │
└─────────────────┘             └─────────────────┘
```

### Componentes Principais

1. **Server**: Servidor gRPC que expõe a API do KV Store
2. **Client**: Cliente CLI para interação com o servidor
3. **Store**: Implementação thread-safe do armazenamento em memória
4. **Proto**: Definições dos contratos gRPC

## ⚡ Funcionalidades

### Operações CRUD
- **PUT**: Armazenar pares chave-valor
- **GET**: Recuperar valor por chave
- **DELETE**: Remover chave do armazenamento
- **GET_ALL**: Recuperar todos os pares chave-valor

### Sistema de Watch
- **Watch**: Monitorar mudanças em chaves específicas em tempo real
- **Streaming**: Notificações via gRPC streaming
- **Auto-cleanup**: Limpeza automática de watchers desconectados

## 📦 Pré-requisitos

### Opção 1: Docker (Recomendado)
- Docker 20.10+
- Docker Compose 2.0+

### Opção 2: Desenvolvimento Local
- Go 1.25.1 ou superior
- Protocol Buffers compiler (`protoc`)
- Go plugins para protoc:
  - `protoc-gen-go`
  - `protoc-gen-go-grpc`

## 🚀 Quick Start

### Com Docker (Mais Fácil)

```bash
# 1. Clone o repositório
git clone <repository-url>
cd kvstore

# 2. Inicie o servidor
make docker-compose-up

# 3. Teste em outro terminal
docker run --rm --network host kvstore:latest ./client --addr=localhost:50051 --flag="put" --key="test" --value="hello"
```

### Desenvolvimento Local

```bash
# 1. Clone e configure
git clone <repository-url>
cd kvstore
make dev-setup

# 2. Execute o servidor
make run

# 3. Teste em outro terminal
go run client/main.go --flag="put" --key="test" --value="hello"
```

## 🐳 Docker

### Comandos Disponíveis

```bash
# Build das imagens
make docker-build              # Build servidor
make docker-build-client       # Build cliente

# Execução individual
make docker-run               # Executar servidor
make docker-run-client        # Executar cliente de teste

# Docker Compose (Recomendado)
make docker-compose-up        # Iniciar stack completa
make docker-compose-down      # Parar stack
make docker-compose-logs      # Ver logs

# Limpeza
make clean                    # Limpar imagens e containers
```

### Estrutura das Imagens

- **kvstore:latest**: Servidor gRPC otimizado (Alpine Linux)
- **kvstore-client:latest**: Cliente CLI para testes

### Docker Compose

O `docker-compose.yml` inclui:
- **kvstore-server**: Servidor principal com health check
- **kvstore-client**: Cliente para testes (profile: client)
- **kvstore-network**: Rede isolada para comunicação

### Exemplos de Uso

#### Teste Rápido
```bash
# Terminal 1: Iniciar servidor
docker run -p 50051:50051 kvstore:latest

# Terminal 2: Testar operações
docker run --rm --network host kvstore:latest ./client \
  --addr=localhost:50051 --flag="put" --key="docker" --value="test"

docker run --rm --network host kvstore:latest ./client \
  --addr=localhost:50051 --flag="get" --key="docker"
```

#### Desenvolvimento com Hot Reload
```bash
# Usar docker-compose para desenvolvimento
make docker-compose-up

# Testar com cliente local
go run client/main.go --flag="put" --key="local" --value="test"
```

## 💻 Desenvolvimento Local

### Configuração Inicial

```bash
# Setup completo do ambiente
make dev-setup
```

Este comando executa:
- `go mod tidy`: Limpa e organiza dependências
- `make proto_generate`: Gera código gRPC

### Comandos de Desenvolvimento

```bash
# Executar servidor
make run                    # Servidor na porta 50051
go run server/main.go --port=8080  # Porta customizada

# Testar cliente
go run client/main.go --flag="put" --key="nome" --value="Daniel"
go run client/main.go --flag="get" --key="nome"
go run client/main.go --flag="delete" --key="nome"
go run client/main.go --flag="all"

# Popular com dados de teste
make populate

# Monitorar mudanças
go run client/main.go --flag="watch" --key="nome"
```

### Exemplos Práticos

```bash
# Armazenar informações de usuário
go run client/main.go --flag="put" --key="user:1" --value='{"name":"João","email":"joao@email.com"}'

# Recuperar usuário
go run client/main.go --flag="get" --key="user:1"

# Monitorar mudanças no usuário
go run client/main.go --flag="watch" --key="user:1"
```

## 📚 API Reference

### Serviço KvStore

```protobuf
service KvStore {
    rpc Put(PutRequest) returns (PutResponse);
    rpc Get(GetRequest) returns (GetResponse);
    rpc Delete(DeleteRequest) returns (DeleteResponse);
    rpc GetAll(GetAllRequest) returns (GetAllResponse);
    rpc Watch(WatchRequest) returns (stream WatchResponse);
}
```

### Mensagens

#### PutRequest/PutResponse
```protobuf
message PutRequest {
    string key = 1;
    string value = 2;
}

message PutResponse {
    bool success = 1;
}
```

#### GetRequest/GetResponse
```protobuf
message GetRequest {
    string key = 1;
}

message GetResponse {
    string key = 1;
    string value = 2;
}
```

#### WatchRequest/WatchResponse
```protobuf
message WatchRequest {
    string key = 1;
}

message WatchResponse {
    string message = 1;
}
```

## 🧪 Testes

### Testes Locais
```bash
# Executar todos os testes
make test

# Testes com cobertura
make test-coverage

# Teste específico
go test ./store -v
```

### Testes com Docker
```bash
# Testar servidor containerizado
docker run --rm kvstore:latest ./server --help

# Testar cliente containerizado
docker run --rm kvstore:latest ./client --help
```

## 📁 Estrutura do Projeto

```
kvstore/
├── client/                 # Cliente CLI
│   └── main.go
├── server/                 # Servidor gRPC
│   └── main.go
├── store/                  # Implementação do KV Store
│   └── kv.go
├── proto/                  # Definições Protocol Buffers
│   └── kvstore.proto
├── pb/                     # Código gerado do protobuf
│   └── proto/
├── go.mod                  # Dependências Go
├── go.sum                  # Checksums das dependências
├── Makefile               # Comandos de build e Docker
├── Dockerfile             # Imagem do servidor
├── Dockerfile.client      # Imagem do cliente
├── docker-compose.yml     # Orquestração de containers
├── .dockerignore          # Arquivos ignorados no build
├── kvstore_test.go        # Testes unitários
└── README.md              # Este arquivo
```

## 🗺️ Roadmap

### Versão Atual (v1.0)
- ✅ Operações CRUD básicas
- ✅ Sistema de watch em tempo real
- ✅ Cliente CLI funcional
- ✅ Thread-safety
- ✅ Containerização com Docker
- ✅ Multi-stage builds otimizados

### Próximas Versões

#### v1.1 - Persistência
- [ ] Persistência em disco (WAL, Snapshots)
- [ ] Recuperação de dados após restart
- [ ] Configuração de diretório de dados
- [ ] Volumes Docker para persistência

#### v1.2 - Distribuição
- [ ] Cluster de múltiplos nós
- [ ] Replicação de dados
- [ ] Consenso (Raft)
- [ ] Load balancing
- [ ] Docker Swarm/Kubernetes support

#### v1.3 - Performance
- [ ] Cache em memória otimizado
- [ ] Compressão de dados
- [ ] Métricas e monitoramento
- [ ] Benchmarks
- [ ] Docker multi-stage builds otimizados

#### v1.4 - Segurança
- [ ] Autenticação e autorização
- [ ] TLS/SSL
- [ ] Rate limiting
- [ ] Auditoria de operações
- [ ] Docker secrets integration

## 🤝 Contribuição

Contribuições são bem-vindas! Para contribuir:

1. Fork o projeto
2. Crie uma branch para sua feature (`git checkout -b feature/nova-feature`)
3. Commit suas mudanças (`git commit -am 'Adiciona nova feature'`)
4. Push para a branch (`git push origin feature/nova-feature`)
5. Abra um Pull Request

### Padrões de Código

- Siga as convenções do Go
- Escreva testes para novas funcionalidades
- Documente funções públicas
- Use `gofmt` para formatação
- Execute `go vet` antes de commitar
- Teste com Docker antes de fazer PR

### Testando Contribuições

```bash
# Teste local
make test

# Teste com Docker
make docker-build
make docker-run-client

# Limpeza
make clean
```

