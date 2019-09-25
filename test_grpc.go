package main

import (
	"log"

	pb "github.com/iScript/etcd-cr/etcdserver/etcdserverpb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	address = "localhost:2379"
)

func main() {
	conn, err := grpc.Dial(address, grpc.WithInsecure())

	if err != nil {
		log.Fatal("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewKVClient(conn)

	r, err := c.Put(context.Background(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})

	if err != nil {
		log.Fatal("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", r)
}
