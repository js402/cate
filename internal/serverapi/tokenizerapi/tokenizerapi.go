package tokenizerapi

import (
	"context"
	"fmt"

	tokenizerservicepb "github.com/js402/CATE/internal/serverapi/tokenizerapi/proto"
	"github.com/js402/CATE/internal/services/tokenizerservice"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type server struct {
	tokenizerservicepb.UnimplementedTokenizerServiceServer
	coreService tokenizerservice.Tokenizer
}

func RegisterTokenizerServiceServer(grpcSrv *grpc.Server, coreSvc tokenizerservice.Tokenizer) error {
	if grpcSrv == nil {
		return fmt.Errorf("grpc.Server instance is nil")
	}
	if coreSvc == nil {
		panic("core tokenizerservice.Service instance is nil")
	}
	adapter := &server{
		coreService: coreSvc,
	}
	tokenizerservicepb.RegisterTokenizerServiceServer(grpcSrv, adapter)
	return nil
}

func (s *server) Tokenize(ctx context.Context, req *tokenizerservicepb.TokenizeRequest) (*tokenizerservicepb.TokenizeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.ModelName == "" {
		return nil, status.Error(codes.InvalidArgument, "model_name is required")
	}
	tokens, err := s.coreService.Tokenize(ctx, req.ModelName, req.Prompt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "core service failed to tokenize: %v", err)
	}
	responseTokens := make([]int32, len(tokens))
	for i, t := range tokens {
		responseTokens[i] = int32(t)
	}

	return &tokenizerservicepb.TokenizeResponse{Tokens: responseTokens}, nil
}

func (s *server) CountTokens(ctx context.Context, req *tokenizerservicepb.CountTokensRequest) (*tokenizerservicepb.CountTokensResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	if req.ModelName == "" {
		return nil, status.Error(codes.InvalidArgument, "model_name is required")
	}
	count, err := s.coreService.CountTokens(ctx, req.ModelName, req.Prompt)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "core service failed to count tokens: %v", err)
	}

	return &tokenizerservicepb.CountTokensResponse{Count: int32(count)}, nil
}

func (s *server) AvailableModels(ctx context.Context, req *emptypb.Empty) (*tokenizerservicepb.AvailableModelsResponse, error) {
	models, err := s.coreService.AvailableModels(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "core service failed to get available models: %v", err)
	}
	if models == nil {
		models = []string{}
	}
	return &tokenizerservicepb.AvailableModelsResponse{ModelNames: models}, nil
}

func (s *server) OptimalModel(ctx context.Context, req *tokenizerservicepb.OptimalModelRequest) (*tokenizerservicepb.OptimalModelResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	optimalModel, err := s.coreService.OptimalModel(ctx, req.BaseModel)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "core service failed to find optimal model: %v", err)
	}

	return &tokenizerservicepb.OptimalModelResponse{OptimalModelName: optimalModel}, nil
}
