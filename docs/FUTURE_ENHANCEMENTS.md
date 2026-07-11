# Future Enhancements

## Savings envelopes

Virtual sub-accounts within a real account. A user defines a named envelope
(e.g. "Emergency Fund") inside a real account, sets a target amount, and
optionally configures a recurring contribution (monthly or bi-weekly).

Key rules:
- The money stays in the real account; the envelope simply reserves (locks) a
  portion of its balance.
- The reserved amount is excluded from the account's "spendable" balance shown
  in account views.
- Recurring contributions create a scheduled entry that transfers the
  configured amount into the envelope on the recurrence date.
- Envelopes appear in two places: a dedicated Envelopes management dashboard
  and inline in each account's detail view.

Likely schema additions: `envelopes` (id, account_id, name, target, currency,
recurring_amount, recurrence_type [monthly|biweekly], next_date, created_at)
and `envelope_contributions` (id, envelope_id, amount, date, created_at).

## Multi-bank support

Additional parsers beyond BAC (e.g. BCR, Scotiabank Costa Rica). Each bank gets its own
directory under `app/internal/parser/parsers/` following the existing BAC pattern.

## Supabase auth middleware — RLS forwarding

Forward the user's JWT from the `Authorization` header to PostgREST so Row Level Security
policies enforce per-user data isolation. Required before the frontend can call the API
directly on behalf of a logged-in user.

## BCCR exchange rate integration

Fetch official Banco Central de Costa Rica rates for the transaction date when recording
cross-currency transfers, populating `transfers.exchange_rate` and setting
`exchange_source = 'bccr'`.
