-- Migration 011: budgets and budget acknowledgments

-- ── budgets ──────────────────────────────────────────────────────────────────

create table budgets (
    id          uuid primary key default gen_random_uuid(),
    user_id     uuid not null references auth.users(id) on delete cascade,
    category_id uuid not null references categories(id) on delete cascade,
    currency    text not null,
    amount      numeric(12,2) not null check (amount > 0),
    created_at  timestamptz not null default now(),
    unique(user_id, category_id, currency)
);

create index on budgets (user_id);

alter table budgets enable row level security;

create policy "budgets: user owns rows"
on budgets for all
using     (user_id = auth.uid())
with check (user_id = auth.uid());

-- ── budget_acknowledgments ────────────────────────────────────────────────────
-- Records when a user reviews an underspend and decides what to do with it.
-- action: 'kept' = left in account as-is, 'moved' = transferred to another account.

create table budget_acknowledgments (
    id          uuid primary key default gen_random_uuid(),
    budget_id   uuid not null references budgets(id) on delete cascade,
    month       text not null,  -- YYYY-MM
    action      text not null check (action in ('kept', 'moved')),
    transfer_id uuid references transfers(id) on delete set null,
    created_at  timestamptz not null default now(),
    unique(budget_id, month)
);

alter table budget_acknowledgments enable row level security;

create policy "budget_acknowledgments: user owns via budget"
on budget_acknowledgments for all
using (
    exists (select 1 from budgets b where b.id = budget_id and b.user_id = auth.uid())
)
with check (
    exists (select 1 from budgets b where b.id = budget_id and b.user_id = auth.uid())
);

-- ── Grants ────────────────────────────────────────────────────────────────────

grant all on table budgets                to anon, authenticated, service_role;
grant all on table budget_acknowledgments to anon, authenticated, service_role;
