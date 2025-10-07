package main

import (
	"bufio"
	"context"
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

// server representa o servidor gRPC para testes de integração
type server struct {
	pb.UnimplementedKvStoreServer
	store *store.KVStore
}

func (s *server) GetAll(_ context.Context, in *pb.GetAllRequest) (*pb.GetAllResponse, error) {
	res := s.store.GetAll()
	return &pb.GetAllResponse{Values: res}, nil
}

func (s *server) Delete(_ context.Context, in *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	s.store.Delete(in.GetKey())
	return &pb.DeleteResponse{Key: in.GetKey()}, nil
}

func (s *server) Get(_ context.Context, in *pb.GetRequest) (*pb.GetResponse, error) {
	return &pb.GetResponse{Key: in.GetKey(), Value: s.store.Get(in.GetKey())}, nil
}

func (s *server) Put(_ context.Context, in *pb.PutRequest) (*pb.PutResponse, error) {
	s.store.Put(in.GetKey(), in.GetValue())
	return &pb.PutResponse{Success: true}, nil
}

func (s *server) Watch(in *pb.WatchRequest, stream pb.KvStore_WatchServer) error {
	w := s.store.Watch(in.Key)
	defer s.store.Unwatch(w)

	for event := range w.Events {
		if err := stream.Send(&pb.WatchResponse{Message: event}); err != nil {
			return err
		}
	}
	return nil
}

// IntegrationTestServer representa um servidor completo para testes de integração
type IntegrationTestServer struct {
	server   *grpc.Server
	store    *store.KVStore
	db       *bolt.DB
	listener net.Listener
	addr     string
}

// setupIntegrationTestServer cria um servidor completo para testes de integração
func setupIntegrationTestServer(t *testing.T) *IntegrationTestServer {
	// Cria um banco de dados temporário
	dbPath := "integration_test.db"
	os.Remove(dbPath) // Remove se existir

	db, err := bolt.Open(dbPath, constants.DBFilePermission, nil)
	if err != nil {
		t.Fatalf("failed to open integration test db: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(constants.BucketStore))
		return err
	})

	if err != nil {
		t.Fatalf("failed to create bucket in integration test db: %v", err)
	}

	// Inicializa o store
	store.Init(db)

	// Cria o servidor
	srv := grpc.NewServer()
	kvStore := store.NewKVStore()
	s := &server{
		store: kvStore,
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
	time.Sleep(200 * time.Millisecond)

	return &IntegrationTestServer{
		server:   srv,
		store:    kvStore,
		db:       db,
		listener: listener,
		addr:     listener.Addr().String(),
	}
}

// cleanupIntegrationTestServer limpa o servidor de integração
func cleanupIntegrationTestServer(t *testing.T, its *IntegrationTestServer) {
	if its.server != nil {
		its.server.Stop()
	}
	if its.db != nil {
		its.db.Close()
	}
	if its.listener != nil {
		its.listener.Close()
	}
	os.Remove("integration_test.db")
	os.Remove("walog.ndjson")
}

// createIntegrationTestClient cria um cliente gRPC para testes de integração
func createIntegrationTestClient(t *testing.T, addr string) pb.KvStoreClient {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	return pb.NewKvStoreClient(conn)
}

func TestIntegration_CompleteWorkflow(t *testing.T) {
	its := setupIntegrationTestServer(t)
	defer cleanupIntegrationTestServer(t, its)

	client := createIntegrationTestClient(t, its.addr)

	// Teste completo: Put -> Get -> GetAll -> Delete -> Get (deve retornar vazio)

	// 1. Put múltiplos valores
	testData := map[string]string{
		"user:1":    "John Doe",
		"user:2":    "Jane Smith",
		"config:db": "postgresql://localhost:5432/mydb",
		"cache:key": "cached_value",
	}

	for key, value := range testData {
		putReq := &pb.PutRequest{Key: key, Value: value}
		putResp, err := client.Put(context.Background(), putReq)
		if err != nil {
			t.Fatalf("Put() failed for key %s: %v", key, err)
		}
		if !putResp.Success {
			t.Errorf("Put() returned success=false for key %s", key)
		}
	}

	// 2. Get individual
	for key, expectedValue := range testData {
		getReq := &pb.GetRequest{Key: key}
		getResp, err := client.Get(context.Background(), getReq)
		if err != nil {
			t.Fatalf("Get() failed for key %s: %v", key, err)
		}
		if getResp.Value != expectedValue {
			t.Errorf("Get() returned wrong value for key %s. Expected %s, got %s", key, expectedValue, getResp.Value)
		}
	}

	// 3. GetAll
	getAllReq := &pb.GetAllRequest{}
	getAllResp, err := client.GetAll(context.Background(), getAllReq)
	if err != nil {
		t.Fatalf("GetAll() failed: %v", err)
	}

	if len(getAllResp.Values) != len(testData) {
		t.Errorf("GetAll() returned wrong number of items. Expected %d, got %d", len(testData), len(getAllResp.Values))
	}

	for key, expectedValue := range testData {
		if getAllResp.Values[key] != expectedValue {
			t.Errorf("GetAll() returned wrong value for key %s. Expected %s, got %s", key, expectedValue, getAllResp.Values[key])
		}
	}

	// 4. Delete um item
	deleteReq := &pb.DeleteRequest{Key: "user:1"}
	deleteResp, err := client.Delete(context.Background(), deleteReq)
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	if deleteResp.Key != "user:1" {
		t.Errorf("Delete() returned wrong key. Expected user:1, got %s", deleteResp.Key)
	}

	// 5. Verifica se foi deletado
	getReq := &pb.GetRequest{Key: "user:1"}
	getResp, err := client.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if getResp.Value != "" {
		t.Error("Delete() failed to remove the key")
	}

	// 6. Verifica se outros itens ainda existem
	getReq = &pb.GetRequest{Key: "user:2"}
	getResp, err = client.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if getResp.Value != "Jane Smith" {
		t.Error("Delete() removed wrong key")
	}
}

func TestIntegration_Persistence(t *testing.T) {
	// Testa persistência: salva dados, reinicia servidor, verifica se dados persistem

	// Primeira sessão: salva dados
	its1 := setupIntegrationTestServer(t)
	client1 := createIntegrationTestClient(t, its1.addr)

	testData := map[string]string{
		"persistent:key1": "value1",
		"persistent:key2": "value2",
		"persistent:key3": "value3",
	}

	for key, value := range testData {
		putReq := &pb.PutRequest{Key: key, Value: value}
		_, err := client1.Put(context.Background(), putReq)
		if err != nil {
			t.Fatalf("Put() failed for key %s: %v", key, err)
		}
	}

	// Fecha primeira sessão
	cleanupIntegrationTestServer(t, its1)

	// Segunda sessão: verifica se dados persistem
	its2 := setupIntegrationTestServer(t)
	defer cleanupIntegrationTestServer(t, its2)

	client2 := createIntegrationTestClient(t, its2.addr)

	// Verifica se todos os dados foram persistidos
	for key, expectedValue := range testData {
		getReq := &pb.GetRequest{Key: key}
		getResp, err := client2.Get(context.Background(), getReq)
		if err != nil {
			t.Fatalf("Get() failed for key %s: %v", key, err)
		}
		if getResp.Value != expectedValue {
			t.Errorf("Persistence test failed for key %s. Expected %s, got %s", key, expectedValue, getResp.Value)
		}
	}
}

func TestIntegration_WatchMultipleClients(t *testing.T) {
	its := setupIntegrationTestServer(t)
	defer cleanupIntegrationTestServer(t, its)

	// Cria múltiplos clientes
	client1 := createIntegrationTestClient(t, its.addr)
	client2 := createIntegrationTestClient(t, its.addr)

	// Cria streams de watch para ambos os clientes
	watchReq1 := &pb.WatchRequest{Key: "shared_key"}
	stream1, err := client1.Watch(context.Background(), watchReq1)
	if err != nil {
		t.Fatalf("Watch() failed for client1: %v", err)
	}

	watchReq2 := &pb.WatchRequest{Key: "shared_key"}
	stream2, err := client2.Watch(context.Background(), watchReq2)
	if err != nil {
		t.Fatalf("Watch() failed for client2: %v", err)
	}

	// Canais para receber notificações
	notifications1 := make([]string, 0)
	notifications2 := make([]string, 0)
	done1 := make(chan bool)
	done2 := make(chan bool)

	// Goroutines para receber notificações
	go func() {
		for {
			resp, err := stream1.Recv()
			if err != nil {
				break
			}
			notifications1 = append(notifications1, resp.Message)
		}
		done1 <- true
	}()

	go func() {
		for {
			resp, err := stream2.Recv()
			if err != nil {
				break
			}
			notifications2 = append(notifications2, resp.Message)
		}
		done2 <- true
	}()

	// Aguarda streams serem estabelecidos
	time.Sleep(200 * time.Millisecond)

	// Faz operações PUT
	putReq := &pb.PutRequest{Key: "shared_key", Value: "value1"}
	_, err = client1.Put(context.Background(), putReq)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	putReq = &pb.PutRequest{Key: "shared_key", Value: "value2"}
	_, err = client1.Put(context.Background(), putReq)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Aguarda notificações
	time.Sleep(300 * time.Millisecond)

	// Fecha streams
	stream1.CloseSend()
	stream2.CloseSend()

	// Aguarda goroutines terminarem
	<-done1
	<-done2

	// Verifica se ambos os clientes receberam as notificações
	expectedNotifications := 2

	if len(notifications1) != expectedNotifications {
		t.Errorf("Client1 expected %d notifications, got %d", expectedNotifications, len(notifications1))
	}

	if len(notifications2) != expectedNotifications {
		t.Errorf("Client2 expected %d notifications, got %d", expectedNotifications, len(notifications2))
	}

	// Verifica se as notificações são idênticas
	for i := 0; i < expectedNotifications; i++ {
		if notifications1[i] != notifications2[i] {
			t.Errorf("Notification %d differs between clients: %s vs %s", i, notifications1[i], notifications2[i])
		}
	}
}

func TestIntegration_ErrorHandling(t *testing.T) {
	its := setupIntegrationTestServer(t)
	defer cleanupIntegrationTestServer(t, its)

	client := createIntegrationTestClient(t, its.addr)

	// Testa operações com dados inválidos ou extremos

	// Testa com strings muito longas
	longKey := string(make([]byte, 10000))
	longValue := string(make([]byte, 10000))

	putReq := &pb.PutRequest{Key: longKey, Value: longValue}
	putResp, err := client.Put(context.Background(), putReq)
	if err != nil {
		t.Fatalf("Put() with long strings failed: %v", err)
	}
	if !putResp.Success {
		t.Error("Put() with long strings returned success=false")
	}

	// Verifica se foi salvo corretamente
	getReq := &pb.GetRequest{Key: longKey}
	getResp, err := client.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("Get() with long key failed: %v", err)
	}
	if getResp.Value != longValue {
		t.Error("Long value was not stored correctly")
	}

	// Testa com caracteres especiais
	specialKey := "key\nwith\ttabs\r\nand\r\nnewlines"
	specialValue := "value\"with'quotes\\and\\backslashes"

	putReq = &pb.PutRequest{Key: specialKey, Value: specialValue}
	putResp, err = client.Put(context.Background(), putReq)
	if err != nil {
		t.Fatalf("Put() with special characters failed: %v", err)
	}
	if !putResp.Success {
		t.Error("Put() with special characters returned success=false")
	}

	// Verifica se foi salvo corretamente
	getReq = &pb.GetRequest{Key: specialKey}
	getResp, err = client.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("Get() with special key failed: %v", err)
	}
	if getResp.Value != specialValue {
		t.Error("Special characters were not stored correctly")
	}
}

func TestIntegration_ConcurrentOperations(t *testing.T) {
	its := setupIntegrationTestServer(t)
	defer cleanupIntegrationTestServer(t, its)

	client := createIntegrationTestClient(t, its.addr)

	// Testa operações concorrentes com múltiplas goroutines
	numGoroutines := 10
	numOperations := 50

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("concurrent_key_%d_%d", id, j)
				value := fmt.Sprintf("concurrent_value_%d_%d", id, j)

				// Put
				putReq := &pb.PutRequest{Key: key, Value: value}
				putResp, err := client.Put(context.Background(), putReq)
				if err != nil {
					t.Errorf("Put() failed: %v", err)
					continue
				}
				if !putResp.Success {
					t.Errorf("Put() returned success=false")
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
					t.Errorf("Concurrent test failed: expected %s, got %s", value, getResp.Value)
				}

				// Delete
				deleteReq := &pb.DeleteRequest{Key: key}
				deleteResp, err := client.Delete(context.Background(), deleteReq)
				if err != nil {
					t.Errorf("Delete() failed: %v", err)
					continue
				}
				if deleteResp.Key != key {
					t.Errorf("Delete() returned wrong key: expected %s, got %s", key, deleteResp.Key)
				}

				// Verifica se foi deletado
				getResp, err = client.Get(context.Background(), getReq)
				if err != nil {
					t.Errorf("Get() after delete failed: %v", err)
					continue
				}
				if getResp.Value != "" {
					t.Errorf("Delete() failed to remove key %s", key)
				}
			}
			done <- true
		}(i)
	}

	// Aguarda todas as goroutines terminarem
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verifica se o store está vazio (todos os itens foram deletados)
	getAllReq := &pb.GetAllRequest{}
	getAllResp, err := client.GetAll(context.Background(), getAllReq)
	if err != nil {
		t.Fatalf("GetAll() failed: %v", err)
	}

	if len(getAllResp.Values) != 0 {
		t.Errorf("Expected empty store after concurrent operations, got %d items", len(getAllResp.Values))
	}
}

func TestIntegration_WALIntegration(t *testing.T) {
	its := setupIntegrationTestServer(t)
	defer cleanupIntegrationTestServer(t, its)

	client := createIntegrationTestClient(t, its.addr)

	// Testa se o WAL está funcionando corretamente
	testData := map[string]string{
		"wal:key1": "value1",
		"wal:key2": "value2",
		"wal:key3": "value3",
	}

	// Faz operações PUT
	for key, value := range testData {
		putReq := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), putReq)
		if err != nil {
			t.Fatalf("Put() failed for key %s: %v", key, err)
		}
	}

	// Faz operações DELETE
	for key := range testData {
		deleteReq := &pb.DeleteRequest{Key: key}
		_, err := client.Delete(context.Background(), deleteReq)
		if err != nil {
			t.Fatalf("Delete() failed for key %s: %v", key, err)
		}
	}

	// Verifica se o arquivo WAL foi criado
	if _, err := os.Stat("walog.ndjson"); os.IsNotExist(err) {
		t.Error("WAL file was not created")
	}

	// Lê o arquivo WAL e verifica se contém as operações
	file, err := os.Open("walog.ndjson")
	if err != nil {
		t.Fatalf("Failed to open WAL file: %v", err)
	}
	defer file.Close()

	// Conta linhas no arquivo WAL
	lineCount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineCount++
	}

	expectedLines := len(testData) * 2 // PUT + DELETE para cada chave
	if lineCount != expectedLines {
		t.Errorf("WAL file should contain %d lines, got %d", expectedLines, lineCount)
	}
}

// TestIntegration_RealWorldScenario simula um cenário real de uso
func TestIntegration_RealWorldScenario(t *testing.T) {
	its := setupIntegrationTestServer(t)
	defer cleanupIntegrationTestServer(t, its)

	client := createIntegrationTestClient(t, its.addr)

	// Simula um sistema de cache de usuários
	users := map[string]map[string]string{
		"user:1": {
			"name":  "John Doe",
			"email": "john@example.com",
			"role":  "admin",
		},
		"user:2": {
			"name":  "Jane Smith",
			"email": "jane@example.com",
			"role":  "user",
		},
		"user:3": {
			"name":  "Bob Johnson",
			"email": "bob@example.com",
			"role":  "user",
		},
	}

	// Salva usuários
	for userID, userData := range users {
		for field, value := range userData {
			key := fmt.Sprintf("%s:%s", userID, field)
			putReq := &pb.PutRequest{Key: key, Value: value}
			_, err := client.Put(context.Background(), putReq)
			if err != nil {
				t.Fatalf("Put() failed for key %s: %v", key, err)
			}
		}
	}

	// Recupera usuários
	for userID, expectedData := range users {
		for field, expectedValue := range expectedData {
			key := fmt.Sprintf("%s:%s", userID, field)
			getReq := &pb.GetRequest{Key: key}
			getResp, err := client.Get(context.Background(), getReq)
			if err != nil {
				t.Fatalf("Get() failed for key %s: %v", key, err)
			}
			if getResp.Value != expectedValue {
				t.Errorf("Get() returned wrong value for key %s. Expected %s, got %s", key, expectedValue, getResp.Value)
			}
		}
	}

	// Lista todos os usuários
	getAllReq := &pb.GetAllRequest{}
	getAllResp, err := client.GetAll(context.Background(), getAllReq)
	if err != nil {
		t.Fatalf("GetAll() failed: %v", err)
	}

	expectedTotalKeys := 0
	for _, userData := range users {
		expectedTotalKeys += len(userData)
	}

	if len(getAllResp.Values) != expectedTotalKeys {
		t.Errorf("GetAll() returned wrong number of items. Expected %d, got %d", expectedTotalKeys, len(getAllResp.Values))
	}

	// Atualiza um usuário
	putReq := &pb.PutRequest{Key: "user:1:role", Value: "superadmin"}
	_, err = client.Put(context.Background(), putReq)
	if err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Verifica atualização
	getReq := &pb.GetRequest{Key: "user:1:role"}
	getResp, err := client.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if getResp.Value != "superadmin" {
		t.Errorf("Update failed. Expected superadmin, got %s", getResp.Value)
	}

	// Remove um usuário
	for field := range users["user:2"] {
		key := fmt.Sprintf("user:2:%s", field)
		deleteReq := &pb.DeleteRequest{Key: key}
		_, err := client.Delete(context.Background(), deleteReq)
		if err != nil {
			t.Fatalf("Delete() failed for key %s: %v", key, err)
		}
	}

	// Verifica se usuário foi removido
	getReq = &pb.GetRequest{Key: "user:2:name"}
	getResp, err = client.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if getResp.Value != "" {
		t.Error("User deletion failed")
	}
}
