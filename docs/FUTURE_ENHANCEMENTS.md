# Future Enhancements

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
