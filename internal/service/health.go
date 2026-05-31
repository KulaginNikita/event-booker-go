package service

import (
	"context"
	"fmt"
)

type HealthRepository interface {
	Ping(ctx context.Context) error
}

type HealthService struct {
	repo HealthRepository
}

func NewHealthService(repo HealthRepository) *HealthService {
	return &HealthService{repo: repo}
}

func (s *HealthService) Ready(ctx context.Context) error {
	if err := s.repo.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	return nil
}
