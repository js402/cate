package main

import (
	"log"
	"net"
	"strings"

	"github.com/js402/cate/core/serverapi/tokenizerapi"
	"github.com/js402/cate/core/serverops"
	"github.com/js402/cate/tokenizer/service"

	"google.golang.org/grpc"
)

func main() {
	config := &serverops.ConfigTokenizerService{}
	if err := serverops.LoadConfig(config); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}
	models := strings.Split(config.PreloadModels, ",")
	useDefaultURLs := false
	if config.UseDefaultURLs == "true" {
		useDefaultURLs = true
	}
	coreSvc, err := service.New(
		service.Config{
			FallbackModel:  config.FallbackModel,
			AuthToken:      config.ModelSourceAuthToken,
			UseDefaultURLs: useDefaultURLs,
			PreloadModels:  models,
		},
	)
	if err != nil {
		log.Fatalf("failed to create tokenizer service: %v", err)
	}
	if coreSvc == nil {
		log.Fatalf("core tokenizerservice.Service instance is nil")
	}
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
