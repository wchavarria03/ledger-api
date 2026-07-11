package models

type SalaryProfile struct {
	ID           string  `json:"id,omitempty"`
	UserID       string  `json:"user_id"`
	NetSalary    float64 `json:"net_salary"`
	SalaryPeriod string  `json:"salary_period"` // monthly | annual
	HoursPerWeek float64 `json:"hours_per_week"`
}

// PurchaseCheck is the result of converting a price into work time
// against a SalaryProfile's effective hourly rate.
type PurchaseCheck struct {
	Price      float64 `json:"price"`
	HourlyRate float64 `json:"hourly_rate"`
	Hours      float64 `json:"hours"`
	Days       int     `json:"days"`
	RemHours   float64 `json:"remainder_hours"`
}
