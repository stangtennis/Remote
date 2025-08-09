-- Fix schema mismatch: Add is_online column to remote_devices table
-- Agent expects is_online boolean but table only has status text column

-- Add the is_online column as boolean
ALTER TABLE remote_devices 
ADD COLUMN is_online BOOLEAN DEFAULT false;

-- Update existing records to set is_online based on status
UPDATE remote_devices 
SET is_online = (status = 'online');

-- Create an index on is_online for better query performance
CREATE INDEX IF NOT EXISTS idx_remote_devices_is_online 
ON remote_devices(is_online);

-- Optional: Create a trigger to keep is_online and status in sync
CREATE OR REPLACE FUNCTION sync_device_status()
RETURNS TRIGGER AS $$
BEGIN
    -- When is_online changes, update status accordingly
    IF NEW.is_online != OLD.is_online THEN
        NEW.status = CASE 
            WHEN NEW.is_online THEN 'online'
            ELSE 'offline'
        END;
    END IF;
    
    -- When status changes, update is_online accordingly
    IF NEW.status != OLD.status THEN
        NEW.is_online = (NEW.status = 'online');
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to automatically sync the columns
DROP TRIGGER IF EXISTS trigger_sync_device_status ON remote_devices;
CREATE TRIGGER trigger_sync_device_status
    BEFORE UPDATE ON remote_devices
    FOR EACH ROW
    EXECUTE FUNCTION sync_device_status();

-- Verify the schema change
SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns 
WHERE table_name = 'remote_devices' 
AND column_name IN ('status', 'is_online')
ORDER BY column_name;
