package models

type Account struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name"`
	Alias         string `json:"alias,omitempty"`
	BankName      string `json:"bank_name"`
	Currency      string `json:"currency"`
	AccountNumber string `json:"account_number"`
	ShortNumber   string `json:"short_number"`
	UserID        string   `json:"user_id"`
	Locked        bool     `json:"locked"`
	Balance       *float64 `json:"balance,omitempty"`
}
