# Backend Change Needed: Link Existing Transactions as a Transfer

The frontend now has a `/transfers` review page that lists candidate transfer
pairs from `GET /v1/transfers/matches` and lets the user manually confirm the
weak (`amount_date`) ones. There's currently no endpoint that can act on that
confirmation — this doc specs the one that's needed.

---

## Why the existing `POST /v1/transfers` doesn't work here

`POST /v1/transfers` (`TransferService.CreateTransfer`,
`app/internal/services/transfer.go:26`) always **creates two brand-new
transaction rows** from `from_account_id`/`to_account_id` + `amount`/`date`.
It's built for manually recording a transfer that doesn't exist yet in either
account's statement (e.g. the Loans page).

The `/transfers` review page is different: both transactions **already
exist** (they came from an imported statement) and just need to be linked
together. Reusing `POST /v1/transfers` here would create duplicate
transaction rows instead of linking the real ones.

## New endpoint: `POST /v1/transfers/link`

### Request
```json
{
  "from_tx_id": "uuid",
  "to_tx_id": "uuid"
}
```

### Response — `201 Created`
Same shape as `POST /v1/transfers`:
```json
{
  "transfer": {
    "id": "uuid",
    "from_tx_id": "uuid",
    "to_tx_id": "uuid",
    "exchange_rate": null,
    "exchange_source": null
  },
  "from_transaction": { /* Transaction */ },
  "to_transaction": { /* Transaction */ }
}
```

### Behavior
1. Look up both transactions by ID (404 if either is missing).
2. Reject if either transaction already has a non-empty `transfer_id`
   (409 or 422 — already linked).
3. Validate the pair actually looks like a transfer: opposite-signed amounts
   that net to zero, same currency (422 otherwise — same rule
   `CreateTransfer` already applies for cross-currency).
4. Link them using the same tail end of `CreateTransfer`'s logic
   (`app/internal/services/transfer.go:96-108`), skipping the "create new
   transaction" step since both legs already exist:
   ```go
   transfer, err := s.transfers.Create(ctx, fromTxID, toTxID, nil, "manual")
   // ...
   s.transactions.SetTransferID(ctx, fromTxID, transfer.ID)
   s.transactions.SetTransferID(ctx, toTxID, transfer.ID)
   ```

### Error cases
| Status | Condition |
|---|---|
| 400 | Missing `from_tx_id` or `to_tx_id`, or they're equal |
| 404 | Either transaction ID doesn't exist |
| 409 | Either transaction already has a `transfer_id` |
| 422 | Amounts don't net to zero, or currencies don't match |

---

## Related, already-shipped behavior (context, no action needed)

- `GET /v1/transfers/matches` requires `from`/`to` query params
  (`app/internal/handlers/transfer.go` `GetMatches`) — the frontend now
  always calls it with an explicit date range.
- `MatchForPeriod` does **not** filter out pairs that are already linked; the
  frontend filters client-side using `transfer_id` on each leg. No backend
  change needed for this, just flagging it so it isn't "fixed" on both ends
  redundantly.
