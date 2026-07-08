# Frontend Changes: Automatic Transfer Reconciliation

The API now auto-links transfer pairs when a statement is imported. Two fields
changed on the response shapes you already consume. This doc covers what to
update.

---

## 1. `ImportSummary` — new field

`POST /v1/import` (the confirm step) now returns `linked_transfers_count` in
addition to the existing fields.

### Old shape
```json
{
  "account_name": "BAC - ****5567",
  "account_number": "CR04010200009331755567",
  "currency": "USD",
  "bank": "BAC",
  "imported_count": 42
}
```

### New shape
```json
{
  "account_name": "BAC - ****5567",
  "account_number": "CR04010200009331755567",
  "currency": "USD",
  "bank": "BAC",
  "imported_count": 42,
  "linked_transfers_count": 3
}
```

### What to change in the frontend

**`src/types/index.ts`** — add the new field to `ImportSummary`:

```ts
export interface ImportSummary {
  account_name: string;
  account_number: string;
  currency: string;
  bank: string;
  imported_count: number;
  linked_transfers_count: number; // NEW
}
```

**`src/pages/Import.tsx`** — surface the count on the confirmation step.
Show it only when non-zero. Suggested copy (keep it terse):

```tsx
{summary.linked_transfers_count > 0 && (
  <p>
    {summary.linked_transfers_count} transfer
    {summary.linked_transfers_count === 1 ? '' : 's'} auto-matched with your
    other accounts.
  </p>
)}
```

Place this alongside (not instead of) the existing "X transactions imported"
line so the user sees both at a glance.

---

## 2. `TransferMatch` — new `confidence` field

`GET /v1/transfers/matches` now includes `confidence` on every match object.

### Old shape
```json
[
  {
    "from": { ... },
    "to": { ... }
  }
]
```

### New shape
```json
[
  {
    "from": { ... },
    "to": { ... },
    "confidence": "reference"
  }
]
```

Possible values:

| Value | Meaning |
|---|---|
| `"reference"` | Both legs share the same reference number — strongest signal |
| `"short_number"` | One leg's description contains the other account's short number (BAC TEF pattern) |
| `"amount_date"` | Same date + opposite amounts + same currency — weakest, user confirmation needed |

### What to change in the frontend

**`src/types/index.ts`** — add the type and field:

```ts
export type MatchConfidence = 'reference' | 'short_number' | 'amount_date';

export interface TransferMatch {
  from: Transaction;
  to: Transaction;
  confidence: MatchConfidence; // NEW
}
```

Any UI that renders match results (e.g. a `/transfers` review page) can use
this to label or sort matches — `reference` and `short_number` are safe to
auto-accept; `amount_date` should prompt user confirmation before linking.

> **Note:** The API already **skips** auto-linking `amount_date` pairs at
> import time. They will appear in `GET /v1/transfers/matches` but will never
> show up in `linked_transfers_count` on the import summary unless the user
> manually confirms them.

---

## 3. `Transaction` — `transfer_id` now populated on reads

`GET /v1/accounts/:id/transactions` and related list endpoints now return
`transfer_id` when the transaction has been linked to a transfer pair (it was
always in the type definition but was previously always `null`/omitted on
reads). No type change needed if you already have `transfer_id?: string` on
your `Transaction` type.

---

## Summary of files to touch

| File | Change |
|---|---|
| `src/types/index.ts` | Add `linked_transfers_count` to `ImportSummary`; add `MatchConfidence` type and `confidence` field to `TransferMatch` |
| `src/pages/Import.tsx` | Show "N transfers auto-matched" on the confirmation step when `linked_transfers_count > 0` |
