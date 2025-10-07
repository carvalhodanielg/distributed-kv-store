package main

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

// benchServer representa o servidor gRPC para benchmarks
type benchServer struct {
	pb.UnimplementedKvStoreServer
	store *store.KVStore
}

func (s *benchServer) GetAll(_ context.Context, in *pb.GetAllRequest) (*pb.GetAllResponse, error) {
	res := s.store.GetAll()
	return &pb.GetAllResponse{Values: res}, nil
}

func (s *benchServer) Delete(_ context.Context, in *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	s.store.Delete(in.GetKey())
	return &pb.DeleteResponse{Key: in.GetKey()}, nil
}

func (s *benchServer) Get(_ context.Context, in *pb.GetRequest) (*pb.GetResponse, error) {
	return &pb.GetResponse{Key: in.GetKey(), Value: s.store.Get(in.GetKey())}, nil
}

func (s *benchServer) Put(_ context.Context, in *pb.PutRequest) (*pb.PutResponse, error) {
	s.store.Put(in.GetKey(), in.GetValue())
	return &pb.PutResponse{Success: true}, nil
}

func (s *benchServer) Watch(in *pb.WatchRequest, stream pb.KvStore_WatchServer) error {
	w := s.store.Watch(in.Key)
	defer s.store.Unwatch(w)

	for event := range w.Events {
		if err := stream.Send(&pb.WatchResponse{Message: event}); err != nil {
			return err
		}
	}
	return nil
}

// setupBenchmarkServer cria um servidor para benchmarks
func setupBenchmarkServer(b *testing.B) (*grpc.Server, string) {
	// Cria um banco de dados temporário
	dbPath := "benchmark_test.db"

	db, err := bolt.Open(dbPath, constants.DBFilePermission, nil)
	if err != nil {
		b.Fatalf("failed to open benchmark db: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(constants.BucketStore))
		return err
	})

	if err != nil {
		b.Fatalf("failed to create bucket in benchmark db: %v", err)
	}

	// Inicializa o store
	store.Init(db)

	// Cria o servidor
	srv := grpc.NewServer()
	kvStore := store.NewKVStore()
	s := &benchServer{
		store: kvStore,
	}

	pb.RegisterKvStoreServer(srv, s)

	// Escolhe uma porta disponível
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		b.Fatalf("failed to listen: %v", err)
	}

	// Inicia o servidor em uma goroutine
	go func() {
		if err := srv.Serve(listener); err != nil {
			b.Logf("server error: %v", err)
		}
	}()

	// Aguarda um pouco para o servidor iniciar
	time.Sleep(100 * time.Millisecond)

	return srv, listener.Addr().String()
}

// cleanupBenchmarkServer limpa o servidor de benchmark
func cleanupBenchmarkServer(b *testing.B, srv *grpc.Server) {
	if srv != nil {
		srv.Stop()
	}
	os.Remove("benchmark_test.db")
	os.Remove("walog.ndjson")
}

// createBenchmarkClient cria um cliente para benchmarks
func createBenchmarkClient(b *testing.B, addr string) pb.KvStoreClient {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}

	return pb.NewKvStoreClient(conn)
}

func BenchmarkPut(b *testing.B) {
	srv, addr := setupBenchmarkServer(b)
	defer cleanupBenchmarkServer(b, srv)

	client := createBenchmarkClient(b, addr)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)

		req := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}
}

func BenchmarkGet(b *testing.B) {
	srv, addr := setupBenchmarkServer(b)
	defer cleanupBenchmarkServer(b, srv)

	client := createBenchmarkClient(b, addr)

	// Pre-popula dados
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)

		req := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		req := &pb.GetRequest{Key: key}
		_, err := client.Get(context.Background(), req)
		if err != nil {
			b.Fatalf("Get() failed: %v", err)
		}
	}
}

func BenchmarkDelete(b *testing.B) {
	srv, addr := setupBenchmarkServer(b)
	defer cleanupBenchmarkServer(b, srv)

	client := createBenchmarkClient(b, addr)

	// Pre-popula dados
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)

		req := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i)
		req := &pb.DeleteRequest{Key: key}
		_, err := client.Delete(context.Background(), req)
		if err != nil {
			b.Fatalf("Delete() failed: %v", err)
		}
	}
}

func BenchmarkGetAll(b *testing.B) {
	srv, addr := setupBenchmarkServer(b)
	defer cleanupBenchmarkServer(b, srv)

	client := createBenchmarkClient(b, addr)

	// Pre-popula dados
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)

		req := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := &pb.GetAllRequest{}
		_, err := client.GetAll(context.Background(), req)
		if err != nil {
			b.Fatalf("GetAll() failed: %v", err)
		}
	}
}

func BenchmarkConcurrentPut(b *testing.B) {
	srv, addr := setupBenchmarkServer(b)
	defer cleanupBenchmarkServer(b, srv)

	client := createBenchmarkClient(b, addr)

	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		i := 0
		for p.Next() {
			key := fmt.Sprintf("key_%d", i)
			value := fmt.Sprintf("value_%d", i)

			req := &pb.PutRequest{Key: key, Value: value}
			_, err := client.Put(context.Background(), req)
			if err != nil {
				b.Fatalf("Put() failed: %v", err)
			}
			i++
		}
	})
}

func BenchmarkConcurrentGet(b *testing.B) {
	srv, addr := setupBenchmarkServer(b)
	defer cleanupBenchmarkServer(b, srv)

	client := createBenchmarkClient(b, addr)

	// Pre-popula dados
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)

		req := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}

	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		i := 0
		for p.Next() {
			key := fmt.Sprintf("key_%d", i%1000)
			req := &pb.GetRequest{Key: key}
			_, err := client.Get(context.Background(), req)
			if err != nil {
				b.Fatalf("Get() failed: %v", err)
			}
			i++
		}
	})
}

func BenchmarkLargeValues(b *testing.B) {
	srv, addr := setupBenchmarkServer(b)
	defer cleanupBenchmarkServer(b, srv)

	client := createBenchmarkClient(b, addr)

	// Cria valores grandes
	largeValue := string(make([]byte, 1024)) // 1KB

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("large_key_%d", i)

		req := &pb.PutRequest{Key: key, Value: largeValue}
		_, err := client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}
}

func BenchmarkManySmallValues(b *testing.B) {
	srv, addr := setupBenchmarkServer(b)
	defer cleanupBenchmarkServer(b, srv)

	client := createBenchmarkClient(b, addr)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("small_key_%d", i)
		value := "v" // Valor muito pequeno

		req := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), req)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}
	}
}

func BenchmarkMixedOperations(b *testing.B) {
	srv, addr := setupBenchmarkServer(b)
	defer cleanupBenchmarkServer(b, srv)

	client := createBenchmarkClient(b, addr)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("mixed_key_%d", i)
		value := fmt.Sprintf("value_%d", i)

		// Put
		putReq := &pb.PutRequest{Key: key, Value: value}
		_, err := client.Put(context.Background(), putReq)
		if err != nil {
			b.Fatalf("Put() failed: %v", err)
		}

		// Get
		getReq := &pb.GetRequest{Key: key}
		_, err = client.Get(context.Background(), getReq)
		if err != nil {
			b.Fatalf("Get() failed: %v", err)
		}

		// Delete (apenas a cada 10 iterações para não esgotar os dados)
		if i%10 == 0 {
			deleteReq := &pb.DeleteRequest{Key: key}
			_, err = client.Delete(context.Background(), deleteReq)
			if err != nil {
				b.Fatalf("Delete() failed: %v", err)
			}
		}
	}
}

// Benchmarks específicos para o store
func BenchmarkStorePut(b *testing.B) {
	db := setupTestDB(b)
	defer cleanupTestDB(b, db)

	store.Init(db)
	kv := store.NewKVStore()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("store_key_%d", i)
		value := fmt.Sprintf("store_value_%d", i)
		kv.Put(key, value)
	}
}

func BenchmarkStoreGet(b *testing.B) {
	db := setupTestDB(b)
	defer cleanupTestDB(b, db)

	store.Init(db)
	kv := store.NewKVStore()

	// Pre-popula dados
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("store_key_%d", i)
		value := fmt.Sprintf("store_value_%d", i)
		kv.Put(key, value)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("store_key_%d", i)
		kv.Get(key)
	}
}

func BenchmarkStoreDelete(b *testing.B) {
	db := setupTestDB(b)
	defer cleanupTestDB(b, db)

	store.Init(db)
	kv := store.NewKVStore()

	// Pre-popula dados
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("store_key_%d", i)
		value := fmt.Sprintf("store_value_%d", i)
		kv.Put(key, value)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("store_key_%d", i)
		kv.Delete(key)
	}
}

func BenchmarkStoreGetAll(b *testing.B) {
	db := setupTestDB(b)
	defer cleanupTestDB(b, db)

	store.Init(db)
	kv := store.NewKVStore()

	// Pre-popula dados
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("store_key_%d", i)
		value := fmt.Sprintf("store_value_%d", i)
		kv.Put(key, value)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		kv.GetAll()
	}
}

func BenchmarkStoreConcurrentPut(b *testing.B) {
	db := setupTestDB(b)
	defer cleanupTestDB(b, db)

	store.Init(db)
	kv := store.NewKVStore()

	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		i := 0
		for p.Next() {
			key := fmt.Sprintf("store_key_%d", i)
			value := fmt.Sprintf("store_value_%d", i)
			kv.Put(key, value)
			i++
		}
	})
}

func BenchmarkStoreConcurrentGet(b *testing.B) {
	db := setupTestDB(b)
	defer cleanupTestDB(b, db)

	store.Init(db)
	kv := store.NewKVStore()

	// Pre-popula dados
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("store_key_%d", i)
		value := fmt.Sprintf("store_value_%d", i)
		kv.Put(key, value)
	}

	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		i := 0
		for p.Next() {
			key := fmt.Sprintf("store_key_%d", i%1000)
			kv.Get(key)
			i++
		}
	})
}

// Benchmarks para WAL
func BenchmarkWALWrite(b *testing.B) {
	originalLogFile := "walog.ndjson"
	os.Remove(originalLogFile)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("wal_key_%d", i)
		value := fmt.Sprintf("wal_value_%d", i)
		store.LogWrite(key, value)
	}

	// Limpa o arquivo
	os.Remove(originalLogFile)
}

func BenchmarkWALDelete(b *testing.B) {
	originalLogFile := "walog.ndjson"
	os.Remove(originalLogFile)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("wal_key_%d", i)
		store.LogDelete(key)
	}

	// Limpa o arquivo
	os.Remove(originalLogFile)
}

// Funções auxiliares para benchmarks
func setupTestDB(b *testing.B) *bolt.DB {
	dbPath := "benchmark_store.db"
	os.Remove(dbPath)

	db, err := bolt.Open(dbPath, constants.DBFilePermission, nil)
	if err != nil {
		b.Fatalf("failed to open test db: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(constants.BucketStore))
		return err
	})

	if err != nil {
		b.Fatalf("failed to create bucket in test db: %v", err)
	}

	return db
}

func cleanupTestDB(b *testing.B, db *bolt.DB) {
	if db != nil {
		db.Close()
	}
	os.Remove("benchmark_store.db")
}
