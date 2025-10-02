-- Verify session cleanup is working

-- 1. Check if cron job exists
SELECT 
  jobid,
  jobname, 
  schedule, 
  command,
  active
FROM cron.job 
WHERE jobname = 'session-cleanup';

-- 2. Check if cleanup function exists
SELECT 
  routine_name, 
  routine_type
FROM information_schema.routines 
WHERE routine_schema = 'public' 
  AND routine_name = 'cleanup_old_sessions_direct';

-- 3. Test cleanup function (run it manually)
SELECT cleanup_old_sessions_direct();

-- 4. Check recent cron runs
SELECT 
  runid,
  job_pid,
  status,
  return_message,
  start_time,
  end_time
FROM cron.job_run_details 
WHERE jobid = (SELECT jobid FROM cron.job WHERE jobname = 'session-cleanup')
ORDER BY start_time DESC 
LIMIT 5;
