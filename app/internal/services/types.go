package services

type ImportService struct {
	accounts       AccountRepository
	transactions   TransactionRepository
	classifier     *ClassificationService
	categoryRules  CategoryRuleRepository
	txCategories   TransactionCategoryRepository
	ruleExceptions AccountRuleExceptionRepository
	transferSvc    *TransferService
	userID         string
}

type ClassificationService struct {
	rules ClassificationRuleRepository
}

type TransferService struct {
	accounts     AccountRepository
	transactions TransactionRepository
	transfers    TransferRepository
}

type CategoryService struct {
	categories CategoryRepository
	rules      CategoryRuleRepository
	txCats     TransactionCategoryRepository
}

type AccountService struct {
	accounts     AccountRepository
	transactions TransactionRepository
}

type ReportService struct {
	repo       TransactionRepository
	categories CategoryRepository
}

type BudgetService struct {
	budgets      BudgetRepository
	accounts     AccountRepository
	transactions TransactionRepository
}
