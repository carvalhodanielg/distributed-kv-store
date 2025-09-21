#!/bin/bash

echo "=== Testando build local ==="
echo "1. Verificando arquivos protobuf..."
ls -la pb/proto/

echo "2. Testando build do servidor..."
go build -o server ./server/main.go
if [ $? -eq 0 ]; then
    echo "✅ Build do servidor OK"
    ls -la server
else
    echo "❌ Erro no build do servidor"
fi

echo "3. Testando build do cliente..."
go build -o client ./client/main.go
if [ $? -eq 0 ]; then
    echo "✅ Build do cliente OK"
    ls -la client
else
    echo "❌ Erro no build do cliente"
fi

echo "4. Testando Docker build..."
docker build -t kvstore:test .
if [ $? -eq 0 ]; then
    echo "✅ Docker build OK"
else
    echo "❌ Erro no Docker build"
fi
