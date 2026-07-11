-- Migration 012: savings envelopes

-- ── envelopes ─────────────────────────────────────────────────────────────────

create table envelopes (
    id                     uuid primary key default gen_random_uuid(),
    user_id                uuid not null references auth.users(id) on delete cascade,
    account_id             uuid not null references accounts(id) on delete cascade,
    name                   text not null,
    target_amount          numeric(12,2),
    currency               text not null,
    recurring_amount       numeric(12,2),
    recurrence_type        text check (recurrence_type in ('monthly','biweekly')),
    next_contribution_date date,
    created_at             timestamptz not null default now()
);

create index on envelopes (user_id);
create index on envelopes (account_id);

alter table envelopes enable row level security;

create policy "envelopes: user owns rows"
on envelopes for all
using     (user_id = auth.uid())
with check (user_id = auth.uid());

-- ── envelope_contributions ────────────────────────────────────────────────────
-- Each row is a deposit (positive) or withdrawal (negative) into the envelope.
-- Balance = SUM(amount) for a given envelope_id.

create table envelope_contributions (
    id          uuid primary key default gen_random_uuid(),
    envelope_id uuid not null references envelopes(id) on delete cascade,
    amount      numeric(12,2) not null,
    note        text,
    date        date not null default current_date,
    created_at  timestamptz not null default now()
);

create index on envelope_contributions (envelope_id);

alter table envelope_contributions enable row level security;

create policy "envelope_contributions: user owns via envelope"
on envelope_contributions for all
using (
    exists (select 1 from envelopes e where e.id = envelope_id and e.user_id = auth.uid())
)
with check (
    exists (select 1 from envelopes e where e.id = envelope_id and e.user_id = auth.uid())
);

-- ── Grants ────────────────────────────────────────────────────────────────────

grant all on table envelopes               to anon, authenticated, service_role;
grant all on table envelope_contributions  to anon, authenticated, service_role;
