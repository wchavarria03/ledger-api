package supabase

import (
	"context"
	"fmt"

	"ledger-api/app/internal/databases"
	"ledger-api/app/internal/models"
)

// transferRow is the write/read shape for the transfers table.
type transferRow struct {
	ID             string   `json:"id,omitempty"`
	FromTxID       string   `json:"from_tx_id"`
	ToTxID         string   `json:"to_tx_id"`
	ExchangeRate   *float64 `json:"exchange_rate,omitempty"`
	ExchangeSource string   `json:"exchange_source,omitempty"`
}

func NewTransferRepository(client *databases.SupabaseClient) *TransferRepository {
	return &TransferRepository{client: client}
}

// Create links two existing transactions as the two sides of a transfer.
func (r *TransferRepository) Create(ctx context.Context, fromTxID, toTxID string, exchangeRate *float64, exchangeSource string) (*models.Transfer, error) {
	row := transferRow{
		FromTxID:       fromTxID,
		ToTxID:         toTxID,
		ExchangeRate:   exchangeRate,
		ExchangeSource: exchangeSource,
	}

	results, err := databases.Post[[]*transferRow](ctx, r.client, "/rest/v1/transfers", row, "return=representation")
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("create transfer: no result returned")
	}

	res := results[0]
	return &models.Transfer{
		ID:             res.ID,
		FromTxID:       res.FromTxID,
		ToTxID:         res.ToTxID,
		ExchangeRate:   res.ExchangeRate,
		ExchangeSource: res.ExchangeSource,
	}, nil
}
