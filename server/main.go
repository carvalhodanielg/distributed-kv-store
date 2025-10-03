package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/carvalhodanielg/kvstore/internal/constants"
	pb "github.com/carvalhodanielg/kvstore/pb/proto"
	"github.com/carvalhodanielg/kvstore/store"
	"google.golang.org/grpc"

	bolt "go.etcd.io/bbolt"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

type server struct {
	pb.UnimplementedKvStoreServer
	store *store.KVStore
}

func (s *server) GetAll(_ context.Context, in *pb.GetAllRequest) (*pb.GetAllResponse, error) {

	//Isso aqui pode ser problemático pq quem recebe os dados pode alterar a store
	//pra evitar isso precisar fazer e retornar uma cópia.
	//pra isso, devemos fazer um for aqui pra copiar tudo, ou criar um snapshop atualizado a cada update
	//e retornar ele aqui
	res := s.store.GetAll()

	return &pb.GetAllResponse{Values: res}, nil
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

func InitDb(path string) *bolt.DB {
	db, err := bolt.Open(path, constants.DBFilePermission, nil)

	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(constants.BucketStore))
		return err
	})

	if err != nil {
		log.Fatalf("failed to create bucket db: %v", err)
	}
	return db
}

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))

	if err != nil {
		log.Fatalf("SOME'IN aint righ: %v", err)
	}

	srv := grpc.NewServer()

	s := &server{
		store: store.NewKVStore(),
	}

	pb.RegisterKvStoreServer(srv, s)

	db := InitDb(constants.DBFileName)
	defer db.Close()
	store.Init(db)

	log.Printf("server listening at %v", lis.Addr())
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}
