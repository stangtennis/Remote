# Controller Logging System

## Overview

The controller application now includes comprehensive logging to help debug issues, particularly with client loading failures.

## Log Files

Log files are automatically created in the `logs/` directory with timestamps:
- **Location**: `./logs/controller_YYYY-MM-DD_HH-MM-SS.log`
- **Format**: Each log entry includes timestamp, log level, and source file location
- **Output**: Logs are written to both the console and the log file simultaneously

## Log Levels

### INFO
General application flow and successful operations:
- Application startup/shutdown
- Configuration loading
- Successful authentication
- Device loading success
- User approval status

### ERROR
Failures and error conditions:
- Configuration loading failures
- Authentication failures
- HTTP request failures
- JSON parsing errors
- Device loading failures

### DEBUG
Detailed diagnostic information:
- HTTP request/response details
- Payload contents
- Response body contents
- Token lengths and validation
- Individual device details

## Key Logging Points

### Application Startup
```
INFO: === Remote Desktop Controller Starting ===
INFO: Application startup initiated
INFO: Loading configuration...
DEBUG: Supabase URL: https://...
DEBUG: Supabase Key length: XXX characters
INFO: ✅ Supabase client initialized successfully
```

### Authentication Flow
```
INFO: Attempting login for user: user@example.com
DEBUG: [SignIn] Auth URL: https://...
DEBUG: [SignIn] Payload marshaled, size: XXX bytes
DEBUG: [SignIn] Received response with status: 200
INFO: [SignIn] Authentication successful for user: user@example.com
```

### Client Loading
```
INFO: Fetching devices for user: user-id
DEBUG: [GetDevices] RPC URL: https://...
DEBUG: [GetDevices] Payload: {"p_user_id":"..."}
DEBUG: [GetDevices] Response body: [...]
INFO: [GetDevices] Successfully fetched X devices
DEBUG: [GetDevices] Device 1: ID=..., Name=..., Platform=..., Status=...
```

## Debugging Client Loading Issues

When clients fail to load, check the logs for:

1. **Authentication Token**: Verify the auth token is present and has correct length
   ```
   DEBUG: [GetDevices] Auth token present, length: XXX
   ```

2. **HTTP Status Code**: Check if the request succeeded
   ```
   DEBUG: [GetDevices] Received response with status: 200
   ```

3. **Response Body**: Examine the actual response from Supabase
   ```
   DEBUG: [GetDevices] Response body: [{"device_id":"..."}]
   ```

4. **Error Messages**: Look for ERROR level messages
   ```
   ERROR: [GetDevices] Failed to fetch devices with status 403: ...
   ```

## Common Issues

### Empty Device List
- Check if user has devices assigned in the database
- Verify the `get_user_devices` RPC function exists in Supabase
- Check user approval status

### Authentication Failures
- Verify Supabase URL and API key in configuration
- Check network connectivity
- Verify user credentials

### HTTP Errors
- **401 Unauthorized**: Invalid or expired auth token
- **403 Forbidden**: User lacks permissions
- **404 Not Found**: RPC function doesn't exist
- **500 Server Error**: Database or RPC function error

## Log Retention

Logs are kept indefinitely. Consider implementing log rotation or manual cleanup of old log files in the `logs/` directory.

## Disabling Debug Logs

To reduce log verbosity in production, you can modify the logger to only output INFO and ERROR levels by commenting out DEBUG calls or adding a log level filter in `internal/logger/logger.go`.
