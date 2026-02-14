package app

import (
	"context"
	"log/slog"
)

type HealthStatus struct {
	Repository bool `json:"repository"`
}

type healthRepo interface {
	Ping(ctx context.Context) error
}

type HealthService struct {
	logger *slog.Logger
	repo   healthRepo
}

func NewHealthService(logger *slog.Logger, repo healthRepo) *HealthService {
	return &HealthService{
		logger: logger,
		repo:   repo,
	}
}

func (s *HealthService) Live() bool {
	return true
}

func (s *HealthService) Ready(ctx context.Context) HealthStatus {
	var status HealthStatus
	err := s.repo.Ping(ctx)
	if err != nil {
		s.logger.Error("Repository healthcheck failed.", "err", err)
	} else {
		status.Repository = true
	}
	return status
}
