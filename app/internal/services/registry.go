package services

import "ledger-api/app/internal/repositories"

type Registry struct {
	Account        *AccountService
	Budget         *BudgetService
	Envelope       *EnvelopeService
	Reminder       *ReminderService
	Category       *CategoryService
	Classification *ClassificationService
	Import         *ImportService
	Report         *ReportService
	RuleExceptions AccountRuleExceptionRepository
	Transaction    *TransactionService
	Transfer       *TransferService
	SalaryProfile  *SalaryProfileService
}

func NewRegistry(repos *repositories.Registry, userID string) *Registry {
	classifier := NewClassificationService(repos.Classifications)
	transfer := NewTransferService(repos.Accounts, repos.Transactions, repos.Transfers)
	reminder := NewReminderService(repos.Reminders)
	return &Registry{
		Account:        NewAccountService(repos.Accounts, repos.Transactions),
		Budget:         NewBudgetService(repos.Budgets, repos.Accounts, repos.Transactions),
		Envelope:       NewEnvelopeService(repos.Envelopes),
		Reminder:       reminder,
		Category:       NewCategoryService(repos.Categories, repos.CategoryRules, repos.TransactionCategories, repos.Transactions),
		Classification: classifier,
		Import:         NewImportService(repos.Accounts, repos.Transactions, classifier, repos.CategoryRules, repos.TransactionCategories, repos.RuleExceptions, transfer, reminder, userID),
		Report:         NewReportService(repos.Transactions, repos.Categories),
		RuleExceptions: repos.RuleExceptions,
		Transaction:    NewTransactionService(repos.Transactions),
		Transfer:       transfer,
		SalaryProfile:  NewSalaryProfileService(repos.SalaryProfiles),
	}
}
