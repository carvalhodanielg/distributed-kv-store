package testutils

import (
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

// server representa o servidor gRPC para testes
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

// TestServer representa um servidor de teste com todos os componentes
type TestServer struct {
	Server   *grpc.Server
	Store    *store.KVStore
	DB       *bolt.DB
	Listener net.Listener
	Addr     string
}

// TestClient representa um cliente de teste
type TestClient struct {
	Client pb.KvStoreClient
	Conn   *grpc.ClientConn
}

// SetupTestServer cria um servidor de teste completo
func SetupTestServer(t testing.TB) *TestServer {
	// Cria um banco de dados temporário
	dbPath := "test_" + t.Name() + ".db"
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
	time.Sleep(100 * time.Millisecond)

	return &TestServer{
		Server:   srv,
		Store:    kvStore,
		DB:       db,
		Listener: listener,
		Addr:     listener.Addr().String(),
	}
}

// CleanupTestServer limpa o servidor de teste
func CleanupTestServer(t testing.TB, ts *TestServer) {
	if ts.Server != nil {
		ts.Server.Stop()
	}
	if ts.DB != nil {
		ts.DB.Close()
	}
	if ts.Listener != nil {
		ts.Listener.Close()
	}

	// Remove arquivos de teste
	dbPath := "test_" + t.Name() + ".db"
	os.Remove(dbPath)
	os.Remove("walog.ndjson")
}

// CreateTestClient cria um cliente de teste
func CreateTestClient(t testing.TB, addr string) *TestClient {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	return &TestClient{
		Client: pb.NewKvStoreClient(conn),
		Conn:   conn,
	}
}

// Close fecha a conexão do cliente
func (tc *TestClient) Close() {
	if tc.Conn != nil {
		tc.Conn.Close()
	}
}

// PutData adiciona dados de teste ao servidor
func (tc *TestClient) PutData(t testing.TB, data map[string]string) {
	for key, value := range data {
		req := &pb.PutRequest{Key: key, Value: value}
		resp, err := tc.Client.Put(context.Background(), req)
		if err != nil {
			t.Fatalf("Put() failed for key %s: %v", key, err)
		}
		if !resp.Success {
			t.Errorf("Put() returned success=false for key %s", key)
		}
	}
}

// GetData recupera dados do servidor
func (tc *TestClient) GetData(t testing.TB, keys []string) map[string]string {
	result := make(map[string]string)

	for _, key := range keys {
		req := &pb.GetRequest{Key: key}
		resp, err := tc.Client.Get(context.Background(), req)
		if err != nil {
			t.Fatalf("Get() failed for key %s: %v", key, err)
		}
		result[key] = resp.Value
	}

	return result
}

// GetAllData recupera todos os dados do servidor
func (tc *TestClient) GetAllData(t testing.TB) map[string]string {
	req := &pb.GetAllRequest{}
	resp, err := tc.Client.GetAll(context.Background(), req)
	if err != nil {
		t.Fatalf("GetAll() failed: %v", err)
	}
	return resp.Values
}

// DeleteData remove dados do servidor
func (tc *TestClient) DeleteData(t testing.TB, keys []string) {
	for _, key := range keys {
		req := &pb.DeleteRequest{Key: key}
		resp, err := tc.Client.Delete(context.Background(), req)
		if err != nil {
			t.Fatalf("Delete() failed for key %s: %v", key, err)
		}
		if resp.Key != key {
			t.Errorf("Delete() returned wrong key. Expected %s, got %s", key, resp.Key)
		}
	}
}

// WatchData cria um stream de watch e retorna um canal para receber notificações
func (tc *TestClient) WatchData(t testing.TB, key string) (<-chan string, func()) {
	req := &pb.WatchRequest{Key: key}
	stream, err := tc.Client.Watch(context.Background(), req)
	if err != nil {
		t.Fatalf("Watch() failed: %v", err)
	}

	notifications := make(chan string, 10)
	done := make(chan bool)

	go func() {
		defer close(notifications)
		for {
			resp, err := stream.Recv()
			if err != nil {
				break
			}
			select {
			case notifications <- resp.Message:
			case <-done:
				return
			}
		}
	}()

	cleanup := func() {
		close(done)
		stream.CloseSend()
	}

	return notifications, cleanup
}

// AssertDataEqual verifica se os dados são iguais
func AssertDataEqual(t testing.TB, expected, actual map[string]string) {
	if len(expected) != len(actual) {
		t.Errorf("Data length mismatch: expected %d, got %d", len(expected), len(actual))
		return
	}

	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			t.Errorf("Key %s not found in actual data", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("Value mismatch for key %s: expected %s, got %s", key, expectedValue, actualValue)
		}
	}
}

// GenerateTestData gera dados de teste com padrões específicos
func GenerateTestData(prefix string, count int) map[string]string {
	data := make(map[string]string)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%s:key_%d", prefix, i)
		value := fmt.Sprintf("%s:value_%d", prefix, i)
		data[key] = value
	}
	return data
}

// GenerateLargeTestData gera dados de teste grandes
func GenerateLargeTestData(prefix string, count int, valueSize int) map[string]string {
	data := make(map[string]string)
	largeValue := string(make([]byte, valueSize))
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("%s:large_key_%d", prefix, i)
		data[key] = largeValue
	}
	return data
}

// WaitForNotifications aguarda notificações em um canal com timeout
func WaitForNotifications(t testing.TB, notifications <-chan string, expectedCount int, timeout time.Duration) []string {
	var received []string
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for len(received) < expectedCount {
		select {
		case notification, ok := <-notifications:
			if !ok {
				t.Errorf("Notification channel closed unexpectedly. Received %d, expected %d", len(received), expectedCount)
				return received
			}
			received = append(received, notification)
		case <-timer.C:
			t.Errorf("Timeout waiting for notifications. Received %d, expected %d", len(received), expectedCount)
			return received
		}
	}

	return received
}

// BenchmarkHelper fornece helpers para benchmarks
type BenchmarkHelper struct {
	Server *TestServer
	Client *TestClient
}

// SetupBenchmark cria um servidor para benchmarks
func SetupBenchmark(b *testing.B) *BenchmarkHelper {
	ts := SetupTestServer(b)
	tc := CreateTestClient(b, ts.Addr)

	return &BenchmarkHelper{
		Server: ts,
		Client: tc,
	}
}

// CleanupBenchmark limpa o benchmark
func (bh *BenchmarkHelper) CleanupBenchmark(b *testing.B) {
	if bh.Client != nil {
		bh.Client.Close()
	}
	CleanupTestServer(b, bh.Server)
}

// BenchmarkPut executa benchmark de PUT
func (bh *BenchmarkHelper) BenchmarkPut(b *testing.B, keyPrefix string) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("%s:%d", keyPrefix, i)
		value := fmt.Sprintf("value_%d", i)

		req := &pb.PutRequest{Key: key, Value: value}
		_, err := bh.Client.Client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}
}

// BenchmarkGet executa benchmark de GET
func (bh *BenchmarkHelper) BenchmarkGet(b *testing.B, keys []string) {
	// Pre-popula dados
	for i, key := range keys {
		value := fmt.Sprintf("value_%d", i)
		req := &pb.PutRequest{Key: key, Value: value}
		_, err := bh.Client.Client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		req := &pb.GetRequest{Key: key}
		_, err := bh.Client.Client.Get(context.Background(), req)
		if err != nil {
			b.Fatalf("Get() failed: %v", err)
		}
	}
}

// BenchmarkDelete executa benchmark de DELETE
func (bh *BenchmarkHelper) BenchmarkDelete(b *testing.B, keyPrefix string) {
	// Pre-popula dados
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("%s:%d", keyPrefix, i)
		value := fmt.Sprintf("value_%d", i)
		req := &pb.PutRequest{Key: key, Value: value}
		_, err := bh.Client.Client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("%s:%d", keyPrefix, i)
		req := &pb.DeleteRequest{Key: key}
		_, err := bh.Client.Client.Delete(context.Background(), req)
		if err != nil {
			b.Fatalf("Delete() failed: %v", err)
		}
	}
}

// MockServer representa um servidor mock para testes
type MockServer struct {
	store map[string]string
}

// NewMockServer cria um novo servidor mock
func NewMockServer() *MockServer {
	return &MockServer{
		store: make(map[string]string),
	}
}

// Put implementa Put para o mock
func (ms *MockServer) Put(key, value string) {
	ms.store[key] = value
}

// Get implementa Get para o mock
func (ms *MockServer) Get(key string) string {
	return ms.store[key]
}

// Delete implementa Delete para o mock
func (ms *MockServer) Delete(key string) {
	delete(ms.store, key)
}

// GetAll implementa GetAll para o mock
func (ms *MockServer) GetAll() map[string]string {
	result := make(map[string]string)
	for k, v := range ms.store {
		result[k] = v
	}
	return result
}

// Clear limpa o store do mock
func (ms *MockServer) Clear() {
	ms.store = make(map[string]string)
}

// Size retorna o tamanho do store do mock
func (ms *MockServer) Size() int {
	return len(ms.store)
}
