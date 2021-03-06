package main

import (
	"net"
	"log"
	"google.golang.org/grpc"
	"turbo/example/inventoryservice/service/impl"
	p "turbo/example/inventoryservice/gen"
	"google.golang.org/grpc/reflection"
)

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	p.RegisterInventoryServiceServer(grpcServer, &impl.InventoryService{})

	reflection.Register(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
