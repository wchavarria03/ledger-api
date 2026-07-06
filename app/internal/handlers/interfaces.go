package handlers

import (
	"context"
	"time"

	"ledger-api/app/internal/models"
)

type Importer interface {
	Import(ctx context.Context, stmt *models.Statement, bankName string) error
}

type AccountLister interface {
	List(ctx context.Context) ([]*models.Account, error)
	GetByID(ctx context.Context, id string) (*models.Account, error)
	Create(ctx context.Context, a *models.Account) (*models.Account, error)
	Update(ctx context.Context, id string, fields map[string]any) (*models.Account, error)
}

type TransactionLister interface {
	ListFiltered(ctx context.Context, accountID string, filter models.TxFilter) ([]*models.Transaction, int, error)
	Create(ctx context.Context, tx *models.Transaction) (*models.Transaction, error)
}

type StatementImporter interface {
	ImportWithSummary(ctx context.Context, stmt *models.Statement, bankName string, catOverrides map[int][]string) (*models.ImportSummary, error)
	CheckOverlap(ctx context.Context, stmt *models.Statement) (int, error)
}

type ReportSummarizer interface {
	Summarize(ctx context.Context, accountIDs []string, from, to time.Time) (*models.ReportSummary, error)
}

type TransferService interface {
	MatchForPeriod(ctx context.Context, from, to time.Time) ([]models.TransferMatch, error)
	CreateTransfer(ctx context.Context, input models.TransferInput) (*models.TransferResult, error)
}

type RuleExceptionManager interface {
	FindByAccount(ctx context.Context, accountID string) ([]string, error)
	Create(ctx context.Context, accountID, ruleID string) error
	Delete(ctx context.Context, accountID, ruleID string) error
}

type CategoryManager interface {
	List(ctx context.Context) ([]*models.Category, error)
	Create(ctx context.Context, c *models.Category) (*models.Category, error)
	Update(ctx context.Context, id string, fields map[string]string) (*models.Category, error)
	Delete(ctx context.Context, id string) error
	ListRules(ctx context.Context) ([]*models.CategoryRule, error)
	CreateRule(ctx context.Context, r *models.CategoryRule) (*models.CategoryRule, error)
	DeleteRule(ctx context.Context, id string) error
	SetTransactionCategories(ctx context.Context, transactionID string, categoryIDs []string) error
}
