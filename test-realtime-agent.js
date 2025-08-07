/**
 * Test script for Supabase Realtime Agent
 * This script runs the agent and monitors its output with improved formatting
 */

const { spawn } = require('child_process');
const path = require('path');
const readline = require('readline');

console.log('\n🧪 SUPABASE REALTIME AGENT TEST');
console.log('======================================');

// Path to the agent script
const agentPath = path.join(__dirname, 'supabase-realtime-agent.js');

// Spawn the agent process with improved output handling
const agentProcess = spawn('node', [agentPath], {
  stdio: ['ignore', 'pipe', 'pipe'],
  shell: true
});

console.log(`📋 Agent process started with PID: ${agentProcess.pid}`);
console.log('📊 Monitoring agent output (30-second intervals)...');
console.log('======================================\n');

// Create readline interfaces for better line-by-line processing
const stdoutReader = readline.createInterface({
  input: agentProcess.stdout,
  terminal: false
});

const stderrReader = readline.createInterface({
  input: agentProcess.stderr,
  terminal: false
});

// Handle agent output line by line
stdoutReader.on('line', (line) => {
  console.log(`[AGENT] ${line}`);
});

// Handle agent errors line by line
stderrReader.on('line', (line) => {
  console.error(`[ERROR] ${line}`);
});

// Handle agent exit
agentProcess.on('close', (code) => {
  console.log(`\n======================================`);
  console.log(`🛑 Agent process exited with code: ${code}`);
  console.log(`======================================`);
  clearInterval(monitorInterval);
});

// Setup monitoring interval (every 30 seconds)
const monitorInterval = setInterval(() => {
  const timestamp = new Date().toLocaleTimeString();
  console.log(`\n======================================`);
  console.log(`⏱️ [${timestamp}] Monitoring agent status...`);
  console.log(`======================================`);
}, 30000);

// Handle script termination
process.on('SIGINT', () => {
  console.log('\n🛑 Stopping test script and agent...');
  clearInterval(monitorInterval);
  agentProcess.kill();
  process.exit(0);
});

console.log('ℹ️ Press Ctrl+C to stop the test\n');
