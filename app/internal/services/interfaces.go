package services

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"ledger-api/app/internal/models"
)

type AccountRepository interface {
	FindAll(ctx context.Context) ([]*models.Account, error)
	FindByID(ctx context.Context, id string) (*models.Account, error)
	FindByAccountNumber(ctx context.Context, number string) (*models.Account, error)
	Upsert(ctx context.Context, a *models.Account) (*models.Account, error)
	Update(ctx context.Context, id string, fields map[string]any) (*models.Account, error)
	Delete(ctx context.Context, id string) error
}

type TransactionRepository interface {
	Create(ctx context.Context, tx *models.Transaction) (*models.Transaction, error)
	UpsertBatch(ctx context.Context, accountID string, sourceFile string, txs []models.Transaction) error
	GetByID(ctx context.Context, id string) (*models.Transaction, error)
	GetByAccountID(ctx context.Context, accountID string) ([]*models.Transaction, error)
	ListFiltered(ctx context.Context, accountID string, filter models.TxFilter) ([]*models.Transaction, int, error)
	GetByAccountIDsInRange(ctx context.Context, accountIDs []string, from, to time.Time) ([]*models.Transaction, error)
	GetCurrentBalances(ctx context.Context, accountIDs []string) (map[string]float64, error)
	GetLastBalancePerAccount(ctx context.Context, accountIDs []string, before time.Time) (map[string]float64, error)
	FindMatchingPattern(ctx context.Context, pattern, accountID string) ([]*models.Transaction, error)
	Delete(ctx context.Context, id string) error
	SetTransferID(ctx context.Context, txID, transferID string) error
	ClearTransferID(ctx context.Context, txID string) error
	UpdateType(ctx context.Context, txID string, txType models.TransactionType) error
}

// TransferRepository persists the link row connecting two transactions
// that represent the same money moving between accounts.
type TransferRepository interface {
	Create(ctx context.Context, fromTxID, toTxID string, exchangeRate *float64, exchangeSource string) (*models.Transfer, error)
	GetByID(ctx context.Context, id string) (*models.Transfer, error)
	Delete(ctx context.Context, id string) error
}

type ClassificationRuleRepository interface {
	FindAll(ctx context.Context) ([]models.ClassificationRule, error)
}

type CategoryRepository interface {
	FindAll(ctx context.Context) ([]*models.Category, error)
	FindByID(ctx context.Context, id string) (*models.Category, error)
	Create(ctx context.Context, c *models.Category) (*models.Category, error)
	Update(ctx context.Context, id string, fields map[string]string) (*models.Category, error)
	SoftDelete(ctx context.Context, id string) error
}

type CategoryRuleRepository interface {
	FindAll(ctx context.Context) ([]*models.CategoryRule, error)
	FindByID(ctx context.Context, id string) (*models.CategoryRule, error)
	FindByAccountID(ctx context.Context, accountID string) ([]*models.CategoryRule, error)
	Create(ctx context.Context, r *models.CategoryRule) (*models.CategoryRule, error)
	Delete(ctx context.Context, id string) error
}

type TransactionCategoryRepository interface {
	SetCategories(ctx context.Context, transactionID string, categoryIDs []string) error
	AddCategoryBatch(ctx context.Context, txIDs []string, categoryID string) error
}

type AccountRuleExceptionRepository interface {
	FindByAccount(ctx context.Context, accountID string) ([]string, error)
	Create(ctx context.Context, accountID, ruleID string) error
	Delete(ctx context.Context, accountID, ruleID string) error
}

type BudgetRepository interface {
	List(ctx context.Context) ([]*models.Budget, error)
	Create(ctx context.Context, input models.BudgetInput) (*models.Budget, error)
	FindByID(ctx context.Context, id string) (*models.Budget, error)
	Update(ctx context.Context, id string, amount decimal.Decimal) (*models.Budget, error)
	Delete(ctx context.Context, id string) error
	Acknowledge(ctx context.Context, budgetID, month, action string, transferID *string) (*models.BudgetAcknowledgment, error)
	ListAcknowledgments(ctx context.Context, budgetIDs []string, month string) ([]*models.BudgetAcknowledgment, error)
}

type EnvelopeRepository interface {
	List(ctx context.Context) ([]*models.Envelope, error)
	ListByAccountID(ctx context.Context, accountID string) ([]*models.Envelope, error)
	FindByID(ctx context.Context, id string) (*models.Envelope, error)
	Create(ctx context.Context, input models.EnvelopeInput) (*models.Envelope, error)
	Update(ctx context.Context, id string, fields map[string]any) (*models.Envelope, error)
	Delete(ctx context.Context, id string) error
	Contribute(ctx context.Context, envelopeID string, input models.ContributionInput) (*models.EnvelopeContribution, error)
	GetBalances(ctx context.Context, envelopeIDs []string) (map[string]decimal.Decimal, error)
	SetNextContributionDate(ctx context.Context, id, date string) error
}

type ReminderRepository interface {
	List(ctx context.Context) ([]*models.Reminder, error)
	ListByAccountID(ctx context.Context, accountID string) ([]*models.Reminder, error)
	FindByID(ctx context.Context, id string) (*models.Reminder, error)
	Create(ctx context.Context, input models.ReminderInput) (*models.Reminder, error)
	Update(ctx context.Context, id string, fields map[string]any) (*models.Reminder, error)
	Delete(ctx context.Context, id string) error
	MarkCompleted(ctx context.Context, id string) error
}

type SalaryProfileRepository interface {
	FindByUserID(ctx context.Context, userID string) (*models.SalaryProfile, error)
	Upsert(ctx context.Context, p *models.SalaryProfile) (*models.SalaryProfile, error)
}
