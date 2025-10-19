// Background Service Worker - Handles native messaging

console.log('🚀 Remote Desktop Control Extension - Background Script Loaded');

let nativePort = null;
let isNativeHostConnected = false;

// Native host name (must match the native messaging host manifest)
const NATIVE_HOST_NAME = 'com.remote.desktop.control';

// Connect to native host
function connectNativeHost() {
  console.log('🔗 Attempting to connect to native host...');
  
  try {
    nativePort = chrome.runtime.connectNative(NATIVE_HOST_NAME);
    
    nativePort.onMessage.addListener((message) => {
      console.log('📥 Received from native host:', message);
      
      if (message.type === 'connected') {
        isNativeHostConnected = true;
        console.log('✅ Native host connected successfully');
        notifyContentScripts({ type: 'native_host_status', status: 'connected' });
      }
      
      if (message.type === 'input_success') {
        console.log('✅ Input command executed successfully');
      }
      
      if (message.type === 'input_error') {
        console.error('❌ Input command failed:', message.error);
      }
    });
    
    nativePort.onDisconnect.addListener(() => {
      isNativeHostConnected = false;
      console.warn('⚠️ Native host disconnected:', chrome.runtime.lastError?.message || 'Unknown reason');
      nativePort = null;
      
      notifyContentScripts({ type: 'native_host_status', status: 'disconnected' });
      
      // Retry connection after 5 seconds
      setTimeout(connectNativeHost, 5000);
    });
    
    // Send initial ping
    nativePort.postMessage({ type: 'ping' });
    
  } catch (error) {
    console.error('❌ Failed to connect to native host:', error);
    isNativeHostConnected = false;
    
    // Retry after 5 seconds
    setTimeout(connectNativeHost, 5000);
  }
}

// Notify all content scripts about status
function notifyContentScripts(message) {
  chrome.tabs.query({}, (tabs) => {
    tabs.forEach((tab) => {
      chrome.tabs.sendMessage(tab.id, message).catch(() => {
        // Ignore errors for tabs without content script
      });
    });
  });
}

// Handle messages from content scripts
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  console.log('📨 Background: Received message:', message.type);
  
  if (message.type === 'input_command') {
    if (!nativePort || !isNativeHostConnected) {
      console.error('❌ Native host not connected');
      sendResponse({ success: false, error: 'Native host not connected' });
      return true;
    }
    
    // Forward input command to native host
    console.log('📤 Forwarding to native host:', message.command.type);
    nativePort.postMessage({
      type: 'input',
      command: message.command
    });
    
    sendResponse({ success: true });
  }
  
  return true; // Keep message channel open for async response
});

// Check native host status
chrome.runtime.onInstalled.addListener(() => {
  console.log('🎉 Extension installed/updated');
  connectNativeHost();
});

// Connect on startup
connectNativeHost();

console.log('✅ Background script initialized');
