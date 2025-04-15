package poolservice

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/js402/cate/core/serverops"
	"github.com/js402/cate/core/serverops/store"
	"github.com/js402/cate/libs/libdb"
)

var (
	ErrInvalidPool = errors.New("invalid pool data")
	ErrNotFound    = libdb.ErrNotFound
)

type Service struct {
	dbInstance libdb.DBManager
}

func New(db libdb.DBManager) *Service {
	return &Service{dbInstance: db}
}

func (s *Service) Create(ctx context.Context, pool *store.Pool) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	pool.ID = uuid.New().String()
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).CreatePool(ctx, pool)
}

func (s *Service) GetByID(ctx context.Context, id string) (*store.Pool, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).GetPool(ctx, id)
}

func (s *Service) GetByName(ctx context.Context, name string) (*store.Pool, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).GetPoolByName(ctx, name)
}

func (s *Service) Update(ctx context.Context, pool *store.Pool) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).UpdatePool(ctx, pool)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).DeletePool(ctx, id)
}

func (s *Service) ListAll(ctx context.Context) ([]*store.Pool, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).ListPools(ctx)
}

func (s *Service) ListByPurpose(ctx context.Context, purpose string) ([]*store.Pool, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).ListPoolsByPurpose(ctx, purpose)
}

func (s *Service) AssignBackend(ctx context.Context, poolID, backendID string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).AssignBackendToPool(ctx, poolID, backendID)
}

func (s *Service) RemoveBackend(ctx context.Context, poolID, backendID string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).RemoveBackendFromPool(ctx, poolID, backendID)
}

func (s *Service) ListBackends(ctx context.Context, poolID string) ([]*store.Backend, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).ListBackendsForPool(ctx, poolID)
}

func (s *Service) ListPoolsForBackend(ctx context.Context, backendID string) ([]*store.Pool, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).ListPoolsForBackend(ctx, backendID)
}

func (s *Service) AssignModel(ctx context.Context, poolID, modelID string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).AssignModelToPool(ctx, poolID, modelID)
}

func (s *Service) RemoveModel(ctx context.Context, poolID, modelID string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).RemoveModelFromPool(ctx, poolID, modelID)
}

func (s *Service) ListModels(ctx context.Context, poolID string) ([]*store.Model, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).ListModelsForPool(ctx, poolID)
}

func (s *Service) ListPoolsForModel(ctx context.Context, modelID string) ([]*store.Pool, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).ListPoolsForModel(ctx, modelID)
}

func (s *Service) GetServiceName() string {
	return "poolservice"
}

func (s *Service) GetServiceGroup() string {
	return serverops.DefaultDefaultServiceGroup
}
