package tokenizerapi

import (
	"context"
	"fmt"

	"github.com/js402/cate/core/serverapi/tokenizerapi/proto"
	"github.com/js402/cate/core/services/tokenizerservice"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type service struct {
	proto.UnimplementedTokenizerServiceServer
	coreService tokenizerservice.Tokenizer
}

func RegisterTokenizerService(grpcSrv *grpc.Server, coreSvc tokenizerservice.Tokenizer) error {
	if grpcSrv == nil {
		return fmt.Errorf("grpc.Server instance is nil")
	}
	if coreSvc == nil {
		return fmt.Errorf("core tokenizerservice.Service instance is nil")
	}
	adapter := &service{
		coreService: coreSvc,
	}
	proto.RegisterTokenizerServiceServer(grpcSrv, adapter)
	return nil
}

func (s *service) Tokenize(ctx context.Context, req *proto.TokenizeRequest) (*proto.TokenizeResponse, error) {
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

	return &proto.TokenizeResponse{Tokens: responseTokens}, nil
}

func (s *service) CountTokens(ctx context.Context, req *proto.CountTokensRequest) (*proto.CountTokensResponse, error) {
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

	return &proto.CountTokensResponse{Count: int32(count)}, nil
}

// func (s *service) AvailableModels(ctx context.Context, req *emptypb.Empty) (*tokenizerservicepb.AvailableModelsResponse, error) {
// 	models, err := s.coreService.AvailableModels(ctx)
// 	if err != nil {
// 		return nil, status.Errorf(codes.Internal, "core service failed to get available models: %v", err)
// 	}
// 	if models == nil {
// 		models = []string{}
// 	}
// 	return &tokenizerservicepb.AvailableModelsResponse{ModelNames: models}, nil
// }

func (s *service) OptimalModel(ctx context.Context, req *proto.OptimalModelRequest) (*proto.OptimalModelResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}
	optimalModel, err := s.coreService.OptimalModel(ctx, req.BaseModel)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "core service failed to find optimal model: %v", err)
	}

	return &proto.OptimalModelResponse{OptimalModelName: optimalModel}, nil
}
