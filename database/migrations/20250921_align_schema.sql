-- Align/Idempotent schema migration to avoid failures on pre-existing tables
-- Created: 2025-09-21

-- Ensure extension for gen_random_uuid
create extension if not exists pgcrypto;

-- 1) remote_devices
create table if not exists public.remote_devices (
  id              bigint generated always as identity primary key,
  device_id       text not null,
  device_name     text,
  platform        text,
  arch            text,
  cpu_count       int,
  ram_bytes       bigint,
  is_online       boolean default false,
  last_seen       timestamptz default now()
);

-- add missing columns (safe/idempotent)
alter table public.remote_devices add column if not exists device_id   text;
alter table public.remote_devices add column if not exists device_name text;
alter table public.remote_devices add column if not exists platform    text;
alter table public.remote_devices add column if not exists arch        text;
alter table public.remote_devices add column if not exists cpu_count   int;
alter table public.remote_devices add column if not exists ram_bytes   bigint;
alter table public.remote_devices add column if not exists is_online   boolean default false;
alter table public.remote_devices add column if not exists last_seen   timestamptz default now();

-- uniqueness via index (avoids constraint rename collisions)
create unique index if not exists remote_devices_device_id_uidx on public.remote_devices (device_id);
create index if not exists remote_devices_online_idx on public.remote_devices (is_online);
create index if not exists remote_devices_last_seen_idx on public.remote_devices (last_seen);

-- 2) remote_sessions
create table if not exists public.remote_sessions (
  id            uuid primary key default gen_random_uuid(),
  device_id     text not null
);

alter table public.remote_sessions add column if not exists created_by  text;
alter table public.remote_sessions add column if not exists status      text;
alter table public.remote_sessions add column if not exists pin         text;
alter table public.remote_sessions add column if not exists token       text;
alter table public.remote_sessions add column if not exists created_at  timestamptz default now();
alter table public.remote_sessions add column if not exists expires_at  timestamptz;

-- optional status constraint (skip if a different one exists)
-- DO $$
-- begin
--   if not exists (
--     select 1 from pg_constraint where conname = 'remote_sessions_status_check'
--   ) then
--     alter table public.remote_sessions
--       add constraint remote_sessions_status_check
--       check (status in ('pending','active','ended','expired'));
--   end if;
-- end $$;

create index if not exists remote_sessions_device_idx on public.remote_sessions (device_id);
create index if not exists remote_sessions_status_idx on public.remote_sessions (status);
create index if not exists remote_sessions_expires_idx on public.remote_sessions (expires_at);

-- 3) session_signaling
create table if not exists public.session_signaling (
  id           bigint generated always as identity primary key,
  session_id   uuid not null,
  from_side    text not null,
  msg_type     text not null,
  payload      jsonb not null,
  created_at   timestamptz default now()
);

alter table public.session_signaling add column if not exists session_id  uuid;
alter table public.session_signaling add column if not exists from_side   text;
alter table public.session_signaling add column if not exists msg_type    text;
alter table public.session_signaling add column if not exists payload     jsonb;
alter table public.session_signaling add column if not exists created_at  timestamptz default now();

-- optionally attach FK if missing (safe guards if table already existed without FK)
DO $$
begin
  if not exists (
    select 1 from pg_constraint where conname = 'session_signaling_session_id_fkey'
  ) then
    alter table public.session_signaling
      add constraint session_signaling_session_id_fkey
      foreign key (session_id) references public.remote_sessions(id) on delete cascade;
  end if;
end $$;

create index if not exists session_signaling_session_idx on public.session_signaling (session_id, created_at);

-- 4) audit_logs
create table if not exists public.audit_logs (
  id           bigint generated always as identity primary key,
  session_id   uuid,
  device_id    text,
  actor        text,
  event        text,
  details      jsonb,
  created_at   timestamptz default now()
);

alter table public.audit_logs add column if not exists session_id  uuid;
alter table public.audit_logs add column if not exists device_id   text;
alter table public.audit_logs add column if not exists actor       text;
alter table public.audit_logs add column if not exists event       text;
alter table public.audit_logs add column if not exists details     jsonb;
alter table public.audit_logs add column if not exists created_at  timestamptz default now();

create index if not exists audit_logs_session_idx on public.audit_logs (session_id, created_at);
create index if not exists audit_logs_device_idx on public.audit_logs (device_id, created_at);
