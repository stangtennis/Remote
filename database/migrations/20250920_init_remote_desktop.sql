-- Remote Desktop schema init (Supabase)
-- Created: 2025-09-20

-- Extensions
create extension if not exists pgcrypto;

-- Devices table: persistent device registry
create table if not exists public.remote_devices (
  id              bigint generated always as identity primary key,
  device_id       text not null unique,
  device_name     text,
  platform        text,
  arch            text,
  cpu_count       int,
  ram_bytes       bigint,
  is_online       boolean default false,
  last_seen       timestamptz default now()
);

create index if not exists remote_devices_online_idx on public.remote_devices (is_online);
create index if not exists remote_devices_last_seen_idx on public.remote_devices (last_seen);

-- Sessions table: each control session
create table if not exists public.remote_sessions (
  id            uuid primary key default gen_random_uuid(),
  device_id     text not null,
  created_by    text,
  status        text check (status in ('pending','active','ended','expired')) default 'pending',
  pin           text,
  token         text,
  created_at    timestamptz default now(),
  expires_at    timestamptz
);

create index if not exists remote_sessions_device_idx on public.remote_sessions (device_id);
create index if not exists remote_sessions_status_idx on public.remote_sessions (status);
create index if not exists remote_sessions_expires_idx on public.remote_sessions (expires_at);

-- Signaling messages: SDP/ICE exchange
create table if not exists public.session_signaling (
  id           bigint generated always as identity primary key,
  session_id   uuid not null references public.remote_sessions(id) on delete cascade,
  from_side    text check (from_side in ('dashboard','agent')) not null,
  msg_type     text check (msg_type in ('offer','answer','ice')) not null,
  payload      jsonb not null,
  created_at   timestamptz default now()
);

create index if not exists session_signaling_session_idx on public.session_signaling (session_id, created_at);

-- Audit logs: security and troubleshooting
create table if not exists public.audit_logs (
  id           bigint generated always as identity primary key,
  session_id   uuid,
  device_id    text,
  actor        text, -- dashboard|agent|system
  event        text, -- connect|disconnect|input|file_upload|file_download|error|...
  details      jsonb,
  created_at   timestamptz default now()
);

create index if not exists audit_logs_session_idx on public.audit_logs (session_id, created_at);
create index if not exists audit_logs_device_idx on public.audit_logs (device_id, created_at);
