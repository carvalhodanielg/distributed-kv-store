package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"

	pb "github.com/carvalhodanielg/kvstore/pb/proto"
	"github.com/carvalhodanielg/kvstore/store"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

type server struct {
	pb.UnimplementedKvStoreServer
	store store.KVStore
	mu    sync.Mutex
}

func (s *server) Delete(_ context.Context, in *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	log.Printf("Received key: %v", in.GetKey())

	s.store.Delete(in.GetKey())

	return &pb.DeleteResponse{Key: in.GetKey()}, nil
}

func (s *server) Get(_ context.Context, in *pb.GetRequest) (*pb.GetResponse, error) {
	log.Printf("Received %v", in.GetKey())

	return &pb.GetResponse{Key: in.GetKey(), Value: s.store.Get(in.GetKey())}, nil
}

func (s *server) Put(_ context.Context, in *pb.PutRequest) (*pb.PutResponse, error) {
	log.Printf("Received key - %v and value - %v in PUT,", in.GetKey(), in.GetValue())

	s.store.Put(in.GetKey(), in.GetValue())

	return &pb.PutResponse{Success: true}, nil
}

func main() {
	// kv := &store.KVStore{}
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))

	if err != nil {
		log.Fatalf("SOME'IN aint righ: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterKvStoreServer(s, &server{})

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	// kv := &store.KVStore{}

	// var wg sync.WaitGroup

	// for i := 0; i < 10; i++ {
	// 	wg.Add(1)

	// 	go func(i int) {
	// 		defer wg.Done()

	// 		key := fmt.Sprintf("key-%d", i)
	// 		value := fmt.Sprintf("value-%d", i)

	// 		kv.Put(key, value)
	// 	}(i)

	// }

	// wg.Wait()

	// fmt.Println("FIM da execução")

	// for i := 0; i < 10; i++ {
	// 	key := fmt.Sprintf("key-%d", i)
	// 	fmt.Printf("%s => %s\n", key, kv.Get(key))
	// }

}
