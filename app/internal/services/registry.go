package services

import "ledger-api/app/internal/repositories"

type Registry struct {
	Account        *AccountService
	Category       *CategoryService
	Classification *ClassificationService
	Import         *ImportService
	Report         *ReportService
	RuleExceptions AccountRuleExceptionRepository
	Transaction    *TransactionService
	Transfer       *TransferService
}

func NewRegistry(repos *repositories.Registry, userID string) *Registry {
	classifier := NewClassificationService(repos.Classifications)
	return &Registry{
		Account:        NewAccountService(repos.Accounts, repos.Transactions),
		Category:       NewCategoryService(repos.Categories, repos.CategoryRules, repos.TransactionCategories),
		Classification: classifier,
		Import:         NewImportService(repos.Accounts, repos.Transactions, classifier, repos.CategoryRules, repos.TransactionCategories, repos.RuleExceptions, userID),
		Report:         NewReportService(repos.Transactions, repos.Categories),
		RuleExceptions: repos.RuleExceptions,
		Transaction:    NewTransactionService(repos.Transactions),
		Transfer:       NewTransferService(repos.Accounts, repos.Transactions),
	}
}
