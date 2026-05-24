package main

import (
	"context"
	"log"
	"time"

	pb "grpc-client/grpc-hello-world/tutorialpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	address = ":50051"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 5)
	defer cancel()

	cred := insecure.NewCredentials()

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(cred))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	c := pb.NewGreeterServiceClient(conn)

	resp, err := c.SayHello(ctx, &pb.HelloRequest{Name: "Bank"})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", resp.GetMessage())
}
