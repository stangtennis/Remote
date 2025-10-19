// Content script - Runs on the agent page
// Bridges between the web page and the extension's background script

console.log('🔌 Remote Desktop Control Extension - Content Script Loaded');

// Listen for messages from the web page (agent.html)
window.addEventListener('message', (event) => {
  // Only accept messages from same origin
  if (event.source !== window) return;

  // Check if it's an input command
  if (event.data.type === 'input_command') {
    console.log('📨 Content: Received input command from page:', event.data.command.type);
    
    // Forward to background script (which talks to native host)
    chrome.runtime.sendMessage({
      type: 'input_command',
      command: event.data.command
    }).catch(error => {
      console.error('❌ Failed to send to background:', error);
    });
  }
});

// Listen for messages from background script
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'native_host_status') {
    console.log('📡 Content: Native host status:', message.status);
    
    // Notify the page about extension availability
    window.postMessage({
      type: 'extension_ready',
      nativeHostConnected: message.status === 'connected'
    }, '*');
  }
  
  if (message.type === 'input_result') {
    console.log('✅ Content: Input executed successfully');
  }
  
  if (message.type === 'input_error') {
    console.error('❌ Content: Input execution failed:', message.error);
  }
});

// Notify page that extension is loaded
window.postMessage({
  type: 'extension_ready',
  nativeHostConnected: false // Will be updated when native host connects
}, '*');

console.log('✅ Extension content script initialized');
