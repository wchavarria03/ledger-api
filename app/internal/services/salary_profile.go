package services

import (
	"context"
	"fmt"
	"math"

	"ledger-api/app/internal/models"
)

// weeksPerMonth is the average number of weeks in a month (52 weeks / 12 months),
// used to derive a monthly salary's effective weekly pay.
const weeksPerMonth = 52.0 / 12.0

func NewSalaryProfileService(profiles SalaryProfileRepository) *SalaryProfileService {
	return &SalaryProfileService{profiles: profiles}
}

func (s *SalaryProfileService) Get(ctx context.Context, userID string) (*models.SalaryProfile, error) {
	profile, err := s.profiles.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get salary profile: %w", err)
	}
	return profile, nil
}

func (s *SalaryProfileService) Save(ctx context.Context, p *models.SalaryProfile) (*models.SalaryProfile, error) {
	profile, err := s.profiles.Upsert(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("save salary profile: %w", err)
	}
	return profile, nil
}

// HourlyRate derives the effective hourly rate from net salary, pay period,
// and weekly hours worked.
func HourlyRate(p *models.SalaryProfile) (float64, error) {
	if p.HoursPerWeek <= 0 {
		return 0, fmt.Errorf("invalid hours_per_week")
	}

	var weeklyPay float64
	switch p.SalaryPeriod {
	case "monthly":
		weeklyPay = p.NetSalary / weeksPerMonth
	case "annual":
		weeklyPay = p.NetSalary / 52.0
	default:
		return 0, fmt.Errorf("invalid salary_period %q: must be monthly or annual", p.SalaryPeriod)
	}

	return weeklyPay / p.HoursPerWeek, nil
}

// CheckPurchase converts price into hours (and days) of work using the
// profile's effective hourly rate. Stateless — nothing is persisted.
// A workday is assumed to be HoursPerWeek / 5.
func (s *SalaryProfileService) CheckPurchase(_ context.Context, p *models.SalaryProfile, price float64) (*models.PurchaseCheck, error) {
	if p == nil {
		return nil, fmt.Errorf("check purchase: no salary profile set")
	}

	hourlyRate, err := HourlyRate(p)
	if err != nil {
		return nil, fmt.Errorf("check purchase: %w", err)
	}
	if hourlyRate <= 0 {
		return nil, fmt.Errorf("check purchase: hourly rate must be positive")
	}

	hours := price / hourlyRate
	workday := p.HoursPerWeek / 5

	days := 0
	remHours := hours
	if workday > 0 {
		days = int(math.Floor(hours / workday))
		remHours = hours - float64(days)*workday
	}

	return &models.PurchaseCheck{
		Price:      price,
		HourlyRate: hourlyRate,
		Hours:      hours,
		Days:       days,
		RemHours:   remHours,
	}, nil
}
