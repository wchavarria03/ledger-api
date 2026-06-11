-- Migration 008: simplify transaction types to expense / income / transfer
-- fee and interest collapse into expense; transfer_in and transfer_out collapse into transfer.
-- Transfer direction is now inferred from amount sign at the service layer.

-- Drop constraints first so the UPDATEs are not blocked by old allowed values
alter table transactions drop constraint transactions_type_check;
alter table classification_rules drop constraint classification_rules_type_override_check;

-- Migrate existing rows
update transactions set type = 'expense'  where type in ('fee', 'interest');
update transactions set type = 'transfer' where type in ('transfer_in', 'transfer_out');
update classification_rules set type_override = 'expense'  where type_override in ('fee', 'interest');
update classification_rules set type_override = 'transfer' where type_override in ('transfer_in', 'transfer_out');

-- Re-add constraints with the new allowed values
alter table transactions add constraint transactions_type_check
    check (type in ('expense', 'income', 'transfer'));
alter table classification_rules add constraint classification_rules_type_override_check
    check (type_override in ('expense', 'income', 'transfer'));
