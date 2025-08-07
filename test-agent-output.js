/**
 * Test script to run the Supabase Realtime Agent and capture its output
 */

const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

// Create log file
const logFile = fs.createWriteStream(path.join(__dirname, 'agent-output.log'), { flags: 'w' });

console.log('Starting Supabase Realtime Agent with output logging...');

// Spawn the agent process
const agent = spawn('node', ['supabase-realtime-agent.js'], {
  stdio: ['ignore', 'pipe', 'pipe']
});

// Pipe stdout and stderr to both console and log file
agent.stdout.on('data', (data) => {
  const output = data.toString();
  process.stdout.write(output);
  logFile.write(output);
});

agent.stderr.on('data', (data) => {
  const output = data.toString();
  process.stderr.write(output);
  logFile.write(`[ERROR] ${output}`);
});

agent.on('close', (code) => {
  console.log(`Agent process exited with code ${code}`);
  logFile.end();
});

// Keep the script running
process.stdin.resume();

// Handle script termination
process.on('SIGINT', () => {
  console.log('Stopping agent...');
  agent.kill();
  setTimeout(() => {
    process.exit(0);
  }, 1000);
});

console.log('Test script running. Press Ctrl+C to stop.');
