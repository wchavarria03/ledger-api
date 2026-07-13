package handlers

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

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
	Delete(ctx context.Context, id string) error
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
	MatchForPeriod(ctx context.Context, from, to time.Time, fxMin, fxMax *float64) ([]models.TransferMatch, error)
	CreateTransfer(ctx context.Context, input models.TransferInput) (*models.TransferResult, error)
	LinkTransactions(ctx context.Context, fromTxID, toTxID string) (*models.TransferResult, error)
	LinkExisting(ctx context.Context, existingTxID, counterpartAccountID string) (*models.TransferResult, error)
	UpdateTransactionType(ctx context.Context, txID string, newType models.TransactionType) (*models.Transaction, error)
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
	PreviewRule(ctx context.Context, ruleID string) ([]*models.Transaction, error)
	ApplyRule(ctx context.Context, ruleID string) (int, error)
}

type BudgetManager interface {
	List(ctx context.Context, month string) ([]models.BudgetStatus, error)
	Create(ctx context.Context, input models.BudgetInput) (*models.Budget, error)
	Update(ctx context.Context, id string, amount decimal.Decimal) (*models.Budget, error)
	Delete(ctx context.Context, id string) error
	Acknowledge(ctx context.Context, budgetID, month, action string, transferID *string) (*models.BudgetAcknowledgment, error)
}

type EnvelopeManager interface {
	List(ctx context.Context) ([]models.EnvelopeStatus, error)
	ListByAccountID(ctx context.Context, accountID string) ([]models.EnvelopeStatus, error)
	Create(ctx context.Context, input models.EnvelopeInput) (*models.EnvelopeStatus, error)
	Update(ctx context.Context, id string, fields map[string]any) (*models.EnvelopeStatus, error)
	Delete(ctx context.Context, id string) error
	Contribute(ctx context.Context, id string, input models.ContributionInput) (*models.EnvelopeStatus, error)
}

type ReminderManager interface {
	List(ctx context.Context) ([]models.ReminderWithStatus, error)
	ListByAccountID(ctx context.Context, accountID string) ([]models.ReminderWithStatus, error)
	Create(ctx context.Context, input models.ReminderInput) (*models.ReminderWithStatus, error)
	Update(ctx context.Context, id string, fields map[string]any) (*models.ReminderWithStatus, error)
	Delete(ctx context.Context, id string) error
	Complete(ctx context.Context, id string) (*models.ReminderWithStatus, error)
	Link(ctx context.Context, id, transactionID, nextDueDate string) (*models.ReminderWithStatus, error)
}

type SalaryProfileManager interface {
	Get(ctx context.Context, userID string) (*models.SalaryProfile, error)
	Save(ctx context.Context, p *models.SalaryProfile) (*models.SalaryProfile, error)
	CheckPurchase(ctx context.Context, p *models.SalaryProfile, price float64) (*models.PurchaseCheck, error)
}
