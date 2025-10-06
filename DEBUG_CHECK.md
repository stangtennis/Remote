# Debug Check - Agent Signals Not Received

## Problem
Dashboard never receives agent's answer/ICE candidates from Supabase real-time.

## Test This

### Option 1: Check Supabase Dashboard
1. Go to https://supabase.com
2. Open your project
3. Go to **Table Editor**
4. Open `session_signaling` table
5. Look for rows where `from_side = 'agent'` for your latest session

**If you see agent rows** → Real-time subscription is broken
**If you DON'T see agent rows** → Agent can't insert into database (RLS/permissions)

### Option 2: Add Manual Query in Browser Console
```javascript
// Run this in dashboard console AFTER connecting
const { data, error } = await supabase
  .from('session_signaling')
  .select('*')
  .eq('session_id', 'YOUR_SESSION_ID_HERE') // Replace with actual session ID
  .eq('from_side', 'agent');

console.log('Agent signals:', data);
console.log('Error:', error);
```

## Expected Results

**Working:**
- Should see agent signals in database
- Should see: answer, multiple ICE candidates

**Broken:**
- No agent signals = RLS policy blocking agent
- Agent signals exist but not received = Real-time issue
