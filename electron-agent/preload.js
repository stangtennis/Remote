const { contextBridge, ipcRenderer } = require('electron');

// Expose protected methods that allow the renderer process to use
// the ipcRenderer without exposing the entire object
contextBridge.exposeInMainWorld('electronAPI', {
  // Control APIs
  control: {
    mouseMove: (x, y) => ipcRenderer.invoke('control:mouse-move', { x, y }),
    mouseClick: (button, double) => ipcRenderer.invoke('control:mouse-click', { button, double }),
    mouseButton: (button, action) => ipcRenderer.invoke('control:mouse-button', { button, action }),
    mouseScroll: (deltaX, deltaY) => ipcRenderer.invoke('control:mouse-scroll', { deltaX, deltaY }),
    keyboardPress: (key, modifiers) => ipcRenderer.invoke('control:keyboard-press', { key, modifiers }),
    keyboardType: (text) => ipcRenderer.invoke('control:keyboard-type', { text })
  },
  
  // Screen capture
  getSources: () => ipcRenderer.invoke('get-sources'),
  
  // Platform info
  platform: process.platform
});

// Log that preload script has loaded
console.log('🔌 Preload script loaded - Electron APIs exposed');
