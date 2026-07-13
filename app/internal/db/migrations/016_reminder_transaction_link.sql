-- Migration 016: link resolved reminders to the imported transaction that
-- confirms them, and remember the auto-created next occurrence so its
-- due_date can be adjusted after confirmation (e.g. recurrences that shift
-- based on the actual pay date rather than a fixed day).

alter table payment_reminders add column transaction_id uuid references transactions(id) on delete set null;
alter table payment_reminders add column next_reminder_id uuid references payment_reminders(id) on delete set null;
