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
