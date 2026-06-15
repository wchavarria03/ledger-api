package services

import (
	"context"
	"fmt"

	"ledger-api/app/internal/models"
)

func NewAccountService(accounts AccountRepository, transactions TransactionRepository) *AccountService {
	return &AccountService{accounts: accounts, transactions: transactions}
}

func (s *AccountService) List(ctx context.Context) ([]*models.Account, error) {
	accounts, err := s.accounts.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	ids := make([]string, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID
	}
	balances, err := s.transactions.GetCurrentBalances(ctx, ids)
	if err == nil {
		for _, a := range accounts {
			if bal, ok := balances[a.ID]; ok {
				a.Balance = &bal
			}
		}
	}

	return accounts, nil
}

func (s *AccountService) GetByID(ctx context.Context, id string) (*models.Account, error) {
	account, err := s.accounts.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return account, nil
}

func (s *AccountService) Create(ctx context.Context, a *models.Account) (*models.Account, error) {
	account, err := s.accounts.Upsert(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}
	return account, nil
}

func (s *AccountService) Update(ctx context.Context, id string, fields map[string]any) (*models.Account, error) {
	account, err := s.accounts.Update(ctx, id, fields)
	if err != nil {
		return nil, fmt.Errorf("update account: %w", err)
	}
	return account, nil
}
