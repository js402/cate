package tokenizerservice

import (
	"context"
	"fmt"

	tokenizerservicepb "github.com/js402/cate/serverapi/tokenizerapi/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type grpcClient struct {
	client tokenizerservicepb.TokenizerServiceClient
	conn   *grpc.ClientConn
}

var _ Tokenizer = (*grpcClient)(nil)

type ConfigGRPC struct {
	ServerAddress string
	DialOptions   []grpc.DialOption
}

func NewGRPCTokenizer(ctx context.Context, cfg ConfigGRPC) (Tokenizer, func() error, error) {
	opts := cfg.DialOptions
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(cfg.ServerAddress, opts...)
	if err != nil {
		err = fmt.Errorf("failed to dial gRPC server at %s: %w", cfg.ServerAddress, err)
		return nil, nil, err
	}

	closeFunc := func() error {
		fmt.Println("Closing gRPC connection via returned closer...")
		if conn != nil {
			return conn.Close()
		}
		return nil
	}

	defer func() {
		if err != nil && conn != nil {
			closeFunc()
		}
	}()

	stub := tokenizerservicepb.NewTokenizerServiceClient(conn)
	client := &grpcClient{
		client: stub,
		conn:   conn,
	}

	return client, closeFunc, nil
}

func (c *grpcClient) Tokenize(ctx context.Context, modelName string, prompt string) ([]int, error) {
	req := &tokenizerservicepb.TokenizeRequest{
		ModelName: modelName,
		Prompt:    prompt,
	}

	resp, err := c.client.Tokenize(ctx, req)
	if err != nil {
		// Consider logging the error or wrapping it for more context
		return nil, fmt.Errorf("gRPC Tokenize call failed: %w", err)
	}

	// Convert []int32 from protobuf to []int for the interface contract
	tokens := make([]int, len(resp.Tokens))
	for i, t := range resp.Tokens {
		tokens[i] = int(t) // TODO: Potential data loss if int32 > max int on platform, usually ok.
	}

	return tokens, nil
}

// CountTokens implements the Tokenizer interface.
func (c *grpcClient) CountTokens(ctx context.Context, modelName string, prompt string) (int, error) {
	req := &tokenizerservicepb.CountTokensRequest{
		ModelName: modelName,
		Prompt:    prompt,
	}

	resp, err := c.client.CountTokens(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("gRPC CountTokens call failed: %w", err)
	}

	return int(resp.Count), nil
}

func (c *grpcClient) AvailableModels(ctx context.Context) ([]string, error) {
	req := &emptypb.Empty{}

	resp, err := c.client.AvailableModels(ctx, req)
	if err != nil {
		return []string{}, fmt.Errorf("gRPC AvailableModels call failed: %w.", err)
	}
	if resp == nil {
		return []string{}, fmt.Errorf("gRPC AvailableModels returned nil response.")

	}
	return resp.ModelNames, nil
}

func (c *grpcClient) OptimalModel(ctx context.Context, baseModel string) (string, error) {
	req := &tokenizerservicepb.OptimalModelRequest{
		BaseModel: baseModel,
	}

	resp, err := c.client.OptimalModel(ctx, req)
	if err != nil {
		return "", fmt.Errorf("gRPC OptimalModel call failed: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("gRPC OptimalModel returned nil response")
	}

	return resp.OptimalModelName, nil
}
