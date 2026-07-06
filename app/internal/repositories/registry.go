package repositories

import (
	"ledger-api/app/internal/databases"
	supabaserepo "ledger-api/app/internal/repositories/supabase"
)

type Registry struct {
	Accounts              *supabaserepo.AccountRepository
	Transactions          *supabaserepo.TransactionRepository
	Transfers             *supabaserepo.TransferRepository
	Classifications       *supabaserepo.ClassificationRepository
	Categories            *supabaserepo.CategoryRepository
	CategoryRules         *supabaserepo.CategoryRuleRepository
	TransactionCategories *supabaserepo.TransactionCategoryRepository
	RuleExceptions        *supabaserepo.AccountRuleExceptionRepository
}

func NewRegistry(dbs *databases.Registry) *Registry {
	return &Registry{
		Accounts:              supabaserepo.NewAccountRepository(dbs.Supabase),
		Transactions:          supabaserepo.NewTransactionRepository(dbs.Supabase),
		Transfers:             supabaserepo.NewTransferRepository(dbs.Supabase),
		Classifications:       supabaserepo.NewClassificationRepository(dbs.Supabase),
		Categories:            supabaserepo.NewCategoryRepository(dbs.Supabase),
		CategoryRules:         supabaserepo.NewCategoryRuleRepository(dbs.Supabase),
		TransactionCategories: supabaserepo.NewTransactionCategoryRepository(dbs.Supabase),
		RuleExceptions:        supabaserepo.NewAccountRuleExceptionRepository(dbs.Supabase),
	}
}
