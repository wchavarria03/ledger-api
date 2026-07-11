-- Migration 013: payment reminders

create table payment_reminders (
    id              uuid primary key default gen_random_uuid(),
    user_id         uuid not null references auth.users(id) on delete cascade,
    account_id      uuid references accounts(id) on delete set null,
    title           text not null,
    amount          numeric(12,2),
    currency        text,
    due_date        date not null,
    recurrence_type text check (recurrence_type in ('weekly','biweekly','monthly','yearly')),
    completed_at    timestamptz,
    notes           text,
    created_at      timestamptz not null default now()
);

create index on payment_reminders (user_id);
create index on payment_reminders (account_id);
create index on payment_reminders (due_date);

alter table payment_reminders enable row level security;

create policy "payment_reminders: user owns rows"
on payment_reminders for all
using     (user_id = auth.uid())
with check (user_id = auth.uid());

grant all on table payment_reminders to anon, authenticated, service_role;
