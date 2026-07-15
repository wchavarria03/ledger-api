package services

import (
	"context"

	"ledger-api/app/internal/models"
)

type TransactionService struct {
	repo TransactionRepository
}

func NewTransactionService(repo TransactionRepository) *TransactionService {
	return &TransactionService{repo: repo}
}

func (s *TransactionService) ListByAccount(ctx context.Context, accountID string) ([]*models.Transaction, error) {
	return s.repo.GetByAccountID(ctx, accountID)
}

func (s *TransactionService) ListFiltered(ctx context.Context, accountID string, filter models.TxFilter) ([]*models.Transaction, int, error) {
	return s.repo.ListFiltered(ctx, accountID, filter)
}

func (s *TransactionService) Create(ctx context.Context, tx *models.Transaction) (*models.Transaction, error) {
	return s.repo.Create(ctx, tx)
}

func (s *TransactionService) UpdateNote(ctx context.Context, id, note string) (*models.Transaction, error) {
	if err := s.repo.UpdateNote(ctx, id, note); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
