package supabase

import "ledger-api/app/internal/databases"

type AccountRepository struct {
	client *databases.SupabaseClient
}

type TransactionRepository struct {
	client *databases.SupabaseClient
}

type ClassificationRepository struct {
	client *databases.SupabaseClient
}

type CategoryRepository struct {
	client *databases.SupabaseClient
}

type CategoryRuleRepository struct {
	client *databases.SupabaseClient
}

type TransactionCategoryRepository struct {
	client *databases.SupabaseClient
}

type AccountRuleExceptionRepository struct {
	client *databases.SupabaseClient
}

type TransferRepository struct {
	client *databases.SupabaseClient
}

type BudgetRepository struct {
	client *databases.SupabaseClient
}

type EnvelopeRepository struct {
	client *databases.SupabaseClient
}

type ReminderRepository struct {
	client *databases.SupabaseClient
}

type SalaryProfileRepository struct {
	client *databases.SupabaseClient
}
