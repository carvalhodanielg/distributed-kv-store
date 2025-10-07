# Testes do KVStore

Este diretório contém uma suíte completa de testes para o sistema KVStore, incluindo testes unitários, de integração e benchmarks.

## Estrutura dos Testes

### Testes Unitários

- **`store/kv_test.go`**: Testes unitários para o store.KVStore
- **`store/wal_test.go`**: Testes unitários para o sistema WAL
- **`server/main_test.go`**: Testes unitários para o servidor gRPC

### Testes de Integração

- **`integration_test.go`**: Testes end-to-end do sistema completo

### Benchmarks

- **`benchmark_test.go`**: Benchmarks de performance

### Utilitários de Teste

- **`testutils/testutils.go`**: Helpers e utilitários para testes

## Como Executar os Testes

### Usando Make

```bash
# Executar todos os testes
make -f Makefile.test test

# Executar apenas testes unitários
make -f Makefile.test test-unit

# Executar apenas testes de integração
make -f Makefile.test test-integration

# Executar benchmarks
make -f Makefile.test test-benchmark

# Executar testes com cobertura
make -f Makefile.test test-coverage
```

### Usando Go diretamente

```bash
# Todos os testes
go test -v ./...

# Apenas testes unitários
go test -v -short ./...

# Testes de integração
go test -v -run TestIntegration ./...

# Benchmarks
go test -v -bench=. -benchmem ./...

# Testes com cobertura
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Cobertura de Testes

Os testes cobrem todas as funcionalidades principais do sistema KVStore.
