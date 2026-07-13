package supabase

import (
	"context"
	"fmt"
	"net/url"

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

// GetByID looks up a transfer row by ID.
func (r *TransferRepository) GetByID(ctx context.Context, id string) (*models.Transfer, error) {
	rows, err := databases.Get[[]*transferRow](ctx, r.client, "/rest/v1/transfers", url.Values{
		"id":     []string{"eq." + id},
		"limit":  []string{"1"},
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	return &models.Transfer{
		ID:             row.ID,
		FromTxID:       row.FromTxID,
		ToTxID:         row.ToTxID,
		ExchangeRate:   row.ExchangeRate,
		ExchangeSource: row.ExchangeSource,
	}, nil
}

// Delete removes a transfer link row, e.g. when a linked transaction is
// corrected to no longer be a transfer.
func (r *TransferRepository) Delete(ctx context.Context, id string) error {
	return databases.Delete(ctx, r.client, "/rest/v1/transfers?id=eq."+id)
}
