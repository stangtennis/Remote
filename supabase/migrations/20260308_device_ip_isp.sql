-- Add public IP and ISP columns to remote_devices
ALTER TABLE public.remote_devices ADD COLUMN IF NOT EXISTS public_ip text;
ALTER TABLE public.remote_devices ADD COLUMN IF NOT EXISTS isp text;
