package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/carvalhodanielg/kvstore/pb/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultKey  = "pedra"
	defaultFlag = "get"
)

var (
	addr         = flag.String("addr", "localhost:50051", "the address to connect to")
	key          = flag.String("key", defaultKey, "Key recibida")
	value        = flag.String("value", "dV", "valor recebido")
	typeOfAction = flag.String("flag", defaultFlag, "Tipo de ação desejada pelo cliente")
)

func main() {
	flag.Parse()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	defer conn.Close()

	c := pb.NewKvStoreClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	defer cancel()

	switch *typeOfAction {
	case "put":
		r, err := c.Put(ctx, &pb.PutRequest{Key: *key, Value: *value})

		if err != nil {
			log.Fatalf("could not greet: %v", err)
		}

		log.Printf("Sucess %v, ", r.GetSuccess())

	case "delete":
		r, err := c.Delete(ctx, &pb.DeleteRequest{Key: *key})
		if err != nil {
			log.Fatalf("could not delete: %v", err)
		}

		log.Printf("DELETE-> key: %s", r.GetKey())
	case "all":
		r, err := c.GetAll(ctx, &pb.GetAllRequest{})
		if err != nil {
			log.Fatalf("could not get all: %v", err)
		}

		log.Printf("All values-> %v", r.GetValues())
	case "populate":
		for i := range 15 {
			_, err := c.Put(ctx, &pb.PutRequest{Key: fmt.Sprintf("key-%v", i), Value: fmt.Sprintf("value-%v", i)})
			if err != nil {
				log.Fatalf("could not delete: %v", err)
			}

			_, err = c.Put(ctx, &pb.PutRequest{Key: fmt.Sprintf("key-%v", string(rune('A'+i-1))), Value: fmt.Sprintf("value-%v", string(rune('A'+i-1)))})
			if err != nil {
				log.Fatalf("could not delete: %v", err)
			}

		}
		log.Printf("POPULATED")
	case "watch":

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
		defer cancel()
		client := pb.NewKvStoreClient(conn)
		stream, err := client.Watch(ctx, &pb.WatchRequest{Key: *key})
		if err != nil {
			log.Fatalf("client.watch failed w/nil: %v", err)
		}

		for {
			w, err := stream.Recv()

			if err == io.EOF {
				break
			}

			if err != nil {
				log.Fatalf("client.watch failed: %v", err)

			}

			log.Printf("Result is %v", w.GetMessage())
		}

	default:
		r, err := c.Get(ctx, &pb.GetRequest{Key: *key})

		if err != nil {
			log.Fatalf("could not get: %v", err)
		}

		log.Printf("GET-> %s::%s", r.GetKey(), r.GetValue())
	}

}
