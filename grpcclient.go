package turbo

import (
	"google.golang.org/grpc"
	"log"
)

var (
	client      = new(grpcClient)
	grpcService interface{}
)

type grpcClient struct {
	conn *grpc.ClientConn
}

func (g *grpcClient) dial(address string) (err error) {
	if g.conn, err = grpc.Dial(address, grpc.WithInsecure()); err != nil {
		log.Fatalln("connect error:" + err.Error())
	}
	return err
}

func (g *grpcClient) close() error {
	if g.conn == nil {
		return nil
	}
	return g.conn.Close()
}

func initGrpcConnection(clientCreator func(conn *grpc.ClientConn) interface{}) error {
	// TODO support multiple grpc clients
	// TODO support service discovery
	if grpcService != nil {
		return nil
	}
	if err := client.dial(configs[SERVICE_ADDRESS]); err == nil {
		grpcService = clientCreator(client.conn)
	}
	return nil
}

func closeGrpcConnection() error {
	return client.close()
}

func GrpcService() interface{} {
	if grpcService == nil {
		log.Fatalln("grpc connection not initiated!")
	}
	return grpcService
}
