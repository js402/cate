package modelservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/js402/CATE/internal/serverops"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/libs/libdb"
)

var (
	ErrInvalidModel = errors.New("invalid model data")
)

type Service struct {
	dbInstance libdb.DBManager
}

func New(db libdb.DBManager) *Service {
	return &Service{dbInstance: db}
}

func (s *Service) Append(ctx context.Context, model *store.Model) error {
	if err := validate(model); err != nil {
		return err
	}
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).AppendModel(ctx, model)
}

func (s *Service) List(ctx context.Context) ([]*store.Model, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).ListModels(ctx)
}

func (s *Service) Delete(ctx context.Context, modelName string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).DeleteModel(ctx, modelName)
}

func validate(model *store.Model) error {
	if model.Model == "" {
		return fmt.Errorf("%w: model name is required", ErrInvalidModel)
	}
	return nil
}

func (s *Service) GetServiceName() string {
	return "modelservice"
}

func (s *Service) GetServiceGroup() string {
	return serverops.DefaultDefaultServiceGroup
}
