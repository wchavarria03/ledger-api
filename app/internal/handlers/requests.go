package handlers

type createAccountRequest struct {
	Name          string `json:"name" binding:"required"`
	BankName      string `json:"bank_name" binding:"required"`
	Currency      string `json:"currency" binding:"required"`
	AccountNumber string `json:"account_number"`
	Locked        bool   `json:"locked"`
	External      bool   `json:"external"`
}

type updateAccountRequest struct {
	Alias    string `json:"alias"`
	Currency string `json:"currency"`
	Locked   *bool  `json:"locked"`
	External *bool  `json:"external"`
}

type createTransactionRequest struct {
	Date        string  `json:"date" binding:"required"`
	Description string  `json:"description" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	Type        string  `json:"type" binding:"required"`
	Currency    string  `json:"currency" binding:"required"`
	Reference   string  `json:"reference"`
}

type createCategoryRequest struct {
	Name     string `json:"name" binding:"required"`
	ParentID string `json:"parent_id"`
	Color    string `json:"color"`
}

type updateCategoryRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type createCategoryRuleRequest struct {
	AccountID  string `json:"account_id"`
	Pattern    string `json:"pattern" binding:"required"`
	CategoryID string `json:"category_id" binding:"required"`
	Priority   int    `json:"priority"`
}

type setTransactionCategoriesRequest struct {
	CategoryIDs []string `json:"category_ids" binding:"required"`
}

type createTransferRequest struct {
	FromAccountID string  `json:"from_account_id" binding:"required"`
	ToAccountID   string  `json:"to_account_id" binding:"required"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	Currency      string  `json:"currency" binding:"required"`
	Date          string  `json:"date" binding:"required"`
	Description   string  `json:"description"`
}

type linkTransferRequest struct {
	FromTxID string `json:"from_tx_id" binding:"required"`
	ToTxID   string `json:"to_tx_id" binding:"required"`
}

type createBudgetRequest struct {
	CategoryID string  `json:"category_id" binding:"required"`
	Currency   string  `json:"currency" binding:"required"`
	Amount     float64 `json:"amount" binding:"required,gt=0"`
}

type updateBudgetRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

type acknowledgeBudgetRequest struct {
	Month         string  `json:"month" binding:"required"` // YYYY-MM
	Action        string  `json:"action" binding:"required"` // kept | moved
	FromAccountID string  `json:"from_account_id"`
	ToAccountID   string  `json:"to_account_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Date          string  `json:"date"` // YYYY-MM-DD, required when action=moved
}

type createEnvelopeRequest struct {
	AccountID            string   `json:"account_id" binding:"required"`
	Name                 string   `json:"name" binding:"required"`
	Currency             string   `json:"currency" binding:"required"`
	TargetAmount         *float64 `json:"target_amount"`
	RecurringAmount      *float64 `json:"recurring_amount"`
	RecurrenceType       string   `json:"recurrence_type"` // monthly | biweekly
	NextContributionDate string   `json:"next_contribution_date"` // YYYY-MM-DD
}

type updateEnvelopeRequest struct {
	Name                 string   `json:"name"`
	TargetAmount         *float64 `json:"target_amount"`
	RecurringAmount      *float64 `json:"recurring_amount"`
	RecurrenceType       string   `json:"recurrence_type"`
	NextContributionDate string   `json:"next_contribution_date"`
}

type contributeEnvelopeRequest struct {
	Amount         float64 `json:"amount" binding:"required"`
	Note           string  `json:"note"`
	Date           string  `json:"date"` // YYYY-MM-DD; defaults to today
	ApplyRecurring bool    `json:"apply_recurring"`
}

type createReminderRequest struct {
	AccountID      string   `json:"account_id"`
	Title          string   `json:"title" binding:"required"`
	Amount         *float64 `json:"amount"`
	Currency       string   `json:"currency"`
	DueDate        string   `json:"due_date" binding:"required"`
	RecurrenceType string   `json:"recurrence_type"` // weekly|biweekly|monthly|yearly
	Notes          string   `json:"notes"`
}

type updateReminderRequest struct {
	AccountID      *string  `json:"account_id"`
	Title          string   `json:"title"`
	Amount         *float64 `json:"amount"`
	Currency       string   `json:"currency"`
	DueDate        string   `json:"due_date"`
	RecurrenceType string   `json:"recurrence_type"`
	Notes          string   `json:"notes"`
}

type completeReminderRequest struct {
	CreateTransfer bool    `json:"create_transfer"`
	FromAccountID  string  `json:"from_account_id"`
	ToAccountID    string  `json:"to_account_id"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	Date           string  `json:"date"`        // YYYY-MM-DD
	Description    string  `json:"description"` // optional transfer description
}
