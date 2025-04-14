package main

import (
	"log"
	"net"
	"strings"

	"github.com/js402/cate/serverapi/tokenizerapi"
	"github.com/js402/cate/serverops"
	"github.com/js402/cate/services/tokenizerservice"

	"google.golang.org/grpc"
)

func main() {
	config := &serverops.ConfigTokenizerService{}
	if err := serverops.LoadConfig(config); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}
	models := strings.Split(config.PreloadModels, ",")
	coreSvc, err := tokenizerservice.New(
		tokenizerservice.Config{
			FallbackModel:  config.FallbackModel,
			AuthToken:      config.ModelSourceAuthToken,
			UseDefaultURLs: config.UseDefaultURLs,
			PreloadModels:  models,
		},
	)
	grpcServer := grpc.NewServer()

	if err := tokenizerapi.RegisterTokenizerService(grpcServer, coreSvc); err != nil {
		log.Fatalf("failed to register tokenizer service: %v", err)
	}

	listenAddr := config.Addr
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", listenAddr, err)
	}

	log.Printf("Tokenizer gRPC server listening on %s", listenAddr)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
