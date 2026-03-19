// Package grpc provides gRPC service definitions and server setup
package grpc

import (
	"log"
	"net"
)

// Server represents the gRPC server
type Server struct {
	listener net.Listener
}

// NewServer creates a new gRPC server
func NewServer() *Server {
	return &Server{}
}

// Start begins listening for gRPC connections
func (s *Server) Start(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = lis

	log.Printf("gRPC server listening on %s", addr)

	// TODO: Register actual gRPC services
	// grpcServer := grpc.NewServer()
	// pb.RegisterDestinationAgentServiceServer(grpcServer, &destinationAgentServer{})
	// return grpcServer.Serve(lis)

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	log.Println("gRPC server stopped")
}