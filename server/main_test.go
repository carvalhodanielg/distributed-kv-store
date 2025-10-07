package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/carvalhodanielg/kvstore/internal/constants"
	pb "github.com/carvalhodanielg/kvstore/pb/proto"
	"github.com/carvalhodanielg/kvstore/store"
	bolt "go.etcd.io/bbolt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// setupTestServer cria um servidor de teste
func setupTestServer(t *testing.T) (*grpc.Server, *server, string) {
	// Cria um banco de dados temporário
	dbPath := "test_server.db"
	os.Remove(dbPath) // Remove se existir

	db, err := bolt.Open(dbPath, constants.DBFilePermission, nil)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(constants.BucketStore))
		return err
	})

	if err != nil {
		t.Fatalf("failed to create bucket in test db: %v", err)
	}

	// Inicializa o store
	store.Init(db)

	// Cria o servidor
	srv := grpc.NewServer()
	s := &server{
		store: store.NewKVStore(),
	}

	pb.RegisterKvStoreServer(srv, s)

	// Escolhe uma porta disponível
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	// Inicia o servidor em uma goroutine
	go func() {
		if err := srv.Serve(listener); err != nil {
			t.Logf("server error: %v", err)
		}
	}()

	// Aguarda um pouco para o servidor iniciar
	time.Sleep(100 * time.Millisecond)

	return srv, s, listener.Addr().String()
}

// cleanupTestServer limpa o servidor de teste
func cleanupTestServer(t *testing.T, srv *grpc.Server, addr string) {
	srv.Stop()
	os.Remove("test_server.db")
	os.Remove("walog.ndjson")
}

// createTestClient cria um cliente gRPC para testes
func createTestClient(t *testing.T, addr string) pb.KvStoreClient {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	return pb.NewKvStoreClient(conn)
}

func TestServer_Put(t *testing.T) {
	srv, _, addr := setupTestServer(t)
	defer cleanupTestServer(t, srv, addr)

	client := createTestClient(t, addr)

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"normal_put", "key1", "value1"},
		{"empty_key", "", "value"},
		{"empty_value", "key", ""},
		{"special_chars", "key!@#$%", "value!@#$%"},
		{"unicode", "key_中文", "value_中文"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pb.PutRequest{
				Key:   tt.key,
				Value: tt.value,
			}

			resp, err := client.Put(context.Background(), req)
			if err != nil {
				t.Fatalf("Put() failed: %v", err)
			}

			if !resp.Success {
				t.Error("Put() returned success=false")
			}

			// Verifica se o valor foi realmente salvo
			getReq := &pb.GetRequest{Key: tt.key}
			getResp, err := client.Get(context.Background(), getReq)
			if err != nil {
				t.Fatalf("Get() failed: %v", err)
			}

			if getResp.Value != tt.value {
				t.Errorf("Get() returned wrong value. Expected %s, got %s", tt.value, getResp.Value)
			}
		})
	}
}

func TestServer_Get(t *testing.T) {
	srv, _, addr := setupTestServer(t)
	defer cleanupTestServer(t, srv, addr)

	client := createTestClient(t, addr)

	// Testa chave inexistente
	req := &pb.GetRequest{Key: "nonexistent"}
	resp, err := client.Get(context.Background(), req)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if resp.Key != "nonexistent" {
		t.Errorf("Get() returned wrong key. Expected nonexistent, got %s", resp.Key)
	}

	if resp.Value != "" {
		t.Errorf("Get() for nonexistent key should return empty value, got %s", resp.Value)
	}

	// Adiciona dados de teste
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"":     "empty_key",
	}

	for key, value := range testData {
		putReq := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), putReq)
		if err != nil {
			t.Fatalf("Put() failed: %v", err)
		}
	}

	// Testa recuperação dos dados
	for key, expectedValue := range testData {
		req := &pb.GetRequest{Key: key}
		resp, err := client.Get(context.Background(), req)
		if err != nil {
			t.Fatalf("Get() failed: %v", err)
		}

		if resp.Key != key {
			t.Errorf("Get() returned wrong key. Expected %s, got %s", key, resp.Key)
		}

		if resp.Value != expectedValue {
			t.Errorf("Get() returned wrong value. Expected %s, got %s", expectedValue, resp.Value)
		}
	}
}

func TestServer_Delete(t *testing.T) {
	srv, _, addr := setupTestServer(t)
	defer cleanupTestServer(t, srv, addr)

	client := createTestClient(t, addr)

	// Adiciona dados de teste
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, value := range testData {
		putReq := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), putReq)
		if err != nil {
			t.Fatalf("Put() failed: %v", err)
		}
	}

	// Testa deleção de chave existente
	req := &pb.DeleteRequest{Key: "key1"}
	resp, err := client.Delete(context.Background(), req)
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	if resp.Key != "key1" {
		t.Errorf("Delete() returned wrong key. Expected key1, got %s", resp.Key)
	}

	// Verifica se a chave foi realmente deletada
	getReq := &pb.GetRequest{Key: "key1"}
	getResp, err := client.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if getResp.Value != "" {
		t.Error("Delete() failed to remove the key")
	}

	// Verifica se outras chaves ainda existem
	getReq = &pb.GetRequest{Key: "key2"}
	getResp, err = client.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if getResp.Value != "value2" {
		t.Error("Delete() removed wrong key")
	}

	// Testa deleção de chave inexistente (não deve causar erro)
	req = &pb.DeleteRequest{Key: "nonexistent"}
	resp, err = client.Delete(context.Background(), req)
	if err != nil {
		t.Fatalf("Delete() failed for nonexistent key: %v", err)
	}

	if resp.Key != "nonexistent" {
		t.Errorf("Delete() returned wrong key. Expected nonexistent, got %s", resp.Key)
	}
}

func TestServer_GetAll(t *testing.T) {
	srv, _, addr := setupTestServer(t)
	defer cleanupTestServer(t, srv, addr)

	client := createTestClient(t, addr)

	// Testa GetAll em store vazio
	req := &pb.GetAllRequest{}
	resp, err := client.GetAll(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAll() failed: %v", err)
	}

	if len(resp.Values) != 0 {
		t.Errorf("GetAll() on empty store should return empty map, got %v", resp.Values)
	}

	// Adiciona dados de teste
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, value := range testData {
		putReq := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), putReq)
		if err != nil {
			t.Fatalf("Put() failed: %v", err)
		}
	}

	// Testa GetAll com dados
	resp, err = client.GetAll(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAll() failed: %v", err)
	}

	if len(resp.Values) != len(testData) {
		t.Errorf("GetAll() returned wrong number of items. Expected %d, got %d", len(testData), len(resp.Values))
	}

	for key, expectedValue := range testData {
		if resp.Values[key] != expectedValue {
			t.Errorf("GetAll() returned wrong value for key %s. Expected %s, got %s", key, expectedValue, resp.Values[key])
		}
	}
}

func TestServer_Watch(t *testing.T) {
	srv, _, addr := setupTestServer(t)
	defer cleanupTestServer(t, srv, addr)

	client := createTestClient(t, addr)

	// Cria um stream de watch
	req := &pb.WatchRequest{Key: "test_key"}
	stream, err := client.Watch(context.Background(), req)
	if err != nil {
		t.Fatalf("Watch() failed: %v", err)
	}

	// Canal para receber notificações
	notifications := make([]string, 0)
	done := make(chan bool)

	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				// Stream foi fechado ou erro
				break
			}
			notifications = append(notifications, resp.Message)
		}
		done <- true
	}()

	// Aguarda um pouco para o stream ser estabelecido
	time.Sleep(100 * time.Millisecond)

	// Faz algumas operações PUT
	putReq := &pb.PutRequest{Key: "test_key", Value: "value1"}
	_, err = client.Put(context.Background(), putReq)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	putReq = &pb.PutRequest{Key: "test_key", Value: "value2"}
	_, err = client.Put(context.Background(), putReq)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Operação em chave diferente (não deve gerar notificação)
	putReq = &pb.PutRequest{Key: "other_key", Value: "value3"}
	_, err = client.Put(context.Background(), putReq)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Aguarda um pouco para as notificações chegarem
	time.Sleep(200 * time.Millisecond)

	// Fecha o stream
	stream.CloseSend()

	// Aguarda o canal ser fechado
	<-done

	// Verifica se recebeu as notificações corretas
	expectedNotifications := 2
	if len(notifications) != expectedNotifications {
		t.Errorf("Expected %d notifications, got %d", expectedNotifications, len(notifications))
	}

	// Verifica conteúdo das notificações
	for i, notification := range notifications {
		expectedValue := fmt.Sprintf("value%d", i+1)
		expectedMessage := fmt.Sprintf("Key test_key updated to %s", expectedValue)
		if notification != expectedMessage {
			t.Errorf("Notification %d: expected %s, got %s", i, expectedMessage, notification)
		}
	}
}

func TestServer_Concurrency(t *testing.T) {
	srv, _, addr := setupTestServer(t)
	defer cleanupTestServer(t, srv, addr)

	client := createTestClient(t, addr)

	// Testa concorrência com múltiplas goroutines
	numGoroutines := 5
	numOperations := 20

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)

				// Put
				putReq := &pb.PutRequest{Key: key, Value: value}
				_, err := client.Put(context.Background(), putReq)
				if err != nil {
					t.Errorf("Put() failed: %v", err)
					continue
				}

				// Get
				getReq := &pb.GetRequest{Key: key}
				getResp, err := client.Get(context.Background(), getReq)
				if err != nil {
					t.Errorf("Get() failed: %v", err)
					continue
				}

				if getResp.Value != value {
					t.Errorf("Concurrency test failed: expected %s, got %s", value, getResp.Value)
				}
			}
			done <- true
		}(i)
	}

	// Aguarda todas as goroutines terminarem
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verifica se todos os dados foram salvos corretamente
	getAllReq := &pb.GetAllRequest{}
	getAllResp, err := client.GetAll(context.Background(), getAllReq)
	if err != nil {
		t.Fatalf("GetAll() failed: %v", err)
	}

	expectedCount := numGoroutines * numOperations
	if len(getAllResp.Values) != expectedCount {
		t.Errorf("Concurrency test: expected %d items, got %d", expectedCount, len(getAllResp.Values))
	}
}

func TestInitDb(t *testing.T) {
	dbPath := "test_init.db"
	os.Remove(dbPath) // Remove se existir

	// Testa criação do banco
	db := InitDb(dbPath)
	if db == nil {
		t.Fatal("InitDb() returned nil")
	}

	// Verifica se o bucket foi criado
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(constants.BucketStore))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Bucket not created: %v", err)
	}

	// Limpa
	db.Close()
	os.Remove(dbPath)
}

func TestMain(m *testing.M) {
	// Configura flags para testes
	flag.Set("port", "0") // Usa porta aleatória

	// Executa os testes
	code := m.Run()

	// Limpa arquivos de teste que possam ter sido criados
	os.Remove("test_server.db")
	os.Remove("test_init.db")
	os.Remove("walog.ndjson")

	os.Exit(code)
}
