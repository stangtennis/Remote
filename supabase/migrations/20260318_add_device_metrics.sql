-- Add device metrics columns
ALTER TABLE remote_devices ADD COLUMN IF NOT EXISTS cpu_percent double precision DEFAULT 0;
ALTER TABLE remote_devices ADD COLUMN IF NOT EXISTS memory_used_mb integer DEFAULT 0;
ALTER TABLE remote_devices ADD COLUMN IF NOT EXISTS memory_total_mb integer DEFAULT 0;
ALTER TABLE remote_devices ADD COLUMN IF NOT EXISTS disk_used_gb integer DEFAULT 0;
ALTER TABLE remote_devices ADD COLUMN IF NOT EXISTS disk_total_gb integer DEFAULT 0;
