package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/carvalhodanielg/kvstore/internal/constants"
	pb "github.com/carvalhodanielg/kvstore/pb/proto"
	"github.com/carvalhodanielg/kvstore/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	bolt "go.etcd.io/bbolt"
)

var (
	port = flag.Int("port", 50051, "The server port")
)

type server struct {
	pb.UnimplementedKvStoreServer
	pb.UnimplementedNodeCommunicationServer
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

func (s *server) Heartbeat(_ context.Context, in *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	log.Printf("Received Heartbeat from %v at %v", in.NodeId, in.Timestamp)

	return &pb.HeartbeatResponse{Alive: true, Timestamp: time.Now().Unix()}, nil
}

func (s *server) sendHeartbeatToPeers() {
	peers := os.Getenv("PEERS")

	if peers == "" {
		fmt.Printf("Sem pares definidos")
		return
	}

	peersList := strings.Split(peers, ",")

	nodeID := os.Getenv("NODE_ID")

	for _, peer := range peersList {
		go func(peerAddr string) {
			conn, err := grpc.NewClient(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Printf("Failed to connect to %s: %v", peerAddr, err)

				return
			}

			defer conn.Close()

			client := pb.NewNodeCommunicationClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			req := &pb.HeartbeatRequest{
				NodeId:    nodeID,
				Timestamp: time.Now().Unix(),
			}

			resp, err := client.Heartbeat(ctx, req)
			if err != nil {
				log.Printf("Heartbeat failed to %s: %v", peerAddr, err)
				return
			}

			log.Printf("Heartbeat to %s: alive=%v, timestamp=%d", peerAddr, resp.Alive, resp.Timestamp)
		}(peer)
	}

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
	pb.RegisterNodeCommunicationServer(srv, s)

	if os.Getenv("NODE_ID") == os.Getenv("LEADER") {
		go func() {
			ticker := time.NewTicker(10 * time.Second) //10 segundos
			defer ticker.Stop()

			for range ticker.C {
				s.sendHeartbeatToPeers()
			}
		}()
	}

	db := InitDb(constants.DBFileName)
	defer db.Close()
	store.Init(db)

	s.store.Open("localhost:5000", "NODE_01")

	//restore memomy based on dbData
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(constants.BucketStore))

		b.ForEach(func(k, v []byte) error {
			s.store.PutFromDb(string(k), string(v))
			return nil
		})
		return nil
	})

	log.Printf("server listening at %v", lis.Addr())
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}
