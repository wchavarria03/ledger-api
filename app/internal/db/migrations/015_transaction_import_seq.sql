-- Migration 015: preserve bank-statement row order for same-day transactions
--
-- Transactions only carry a bare `date` (day precision) plus the bank's own
-- running `balance`. Same-day rows previously had no tie-breaker in ORDER BY,
-- so Postgres returned them in an undefined order — the displayed running
-- balance could jump around instead of matching the statement's sequence.
-- import_seq is stamped by the parser in the order rows appear in the source
-- PDF/CSV, giving a stable secondary sort key within a single import.

alter table transactions add column import_seq integer;
