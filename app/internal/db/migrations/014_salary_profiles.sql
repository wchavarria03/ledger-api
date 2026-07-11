-- Migration 014: salary profiles (Hours of Work Calculator)

create table salary_profiles (
    id             uuid primary key default gen_random_uuid(),
    user_id        uuid not null unique references auth.users(id) on delete cascade,
    net_salary     numeric(12,2) not null,
    salary_period  text not null check (salary_period in ('monthly', 'annual')),
    hours_per_week numeric(5,2) not null,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now()
);

create index on salary_profiles (user_id);

alter table salary_profiles enable row level security;

create policy "salary_profiles: user owns rows"
on salary_profiles for all
using     (user_id = auth.uid())
with check (user_id = auth.uid());

grant all on table salary_profiles to anon, authenticated, service_role;
