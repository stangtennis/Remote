const { app, BrowserWindow, ipcMain, desktopCapturer } = require('electron');
const path = require('path');
const { mouse, keyboard, Key, Button } = require('@nut-tree/nut-js');

let mainWindow;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true,
      enableRemoteModule: false
    },
    icon: path.join(__dirname, 'assets/icon.png')
  });

  mainWindow.loadFile('renderer/index.html');

  // Open DevTools in development
  if (process.env.NODE_ENV === 'development') {
    mainWindow.webContents.openDevTools();
  }

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

app.whenReady().then(createWindow);

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    createWindow();
  }
});

// ============================================================================
// IPC Handlers for Remote Control
// ============================================================================

// Handle mouse move
ipcMain.handle('control:mouse-move', async (event, { x, y }) => {
  try {
    await mouse.setPosition({ x, y });
    return { success: true };
  } catch (error) {
    console.error('Mouse move error:', error);
    return { success: false, error: error.message };
  }
});

// Handle mouse click
ipcMain.handle('control:mouse-click', async (event, { button, double }) => {
  try {
    const nutButton = button === 'left' ? Button.LEFT : 
                      button === 'right' ? Button.RIGHT : 
                      Button.MIDDLE;
    
    if (double) {
      await mouse.doubleClick(nutButton);
    } else {
      await mouse.click(nutButton);
    }
    return { success: true };
  } catch (error) {
    console.error('Mouse click error:', error);
    return { success: false, error: error.message };
  }
});

// Handle mouse down/up
ipcMain.handle('control:mouse-button', async (event, { button, action }) => {
  try {
    const nutButton = button === 'left' ? Button.LEFT : 
                      button === 'right' ? Button.RIGHT : 
                      Button.MIDDLE;
    
    if (action === 'down') {
      await mouse.pressButton(nutButton);
    } else {
      await mouse.releaseButton(nutButton);
    }
    return { success: true };
  } catch (error) {
    console.error('Mouse button error:', error);
    return { success: false, error: error.message };
  }
});

// Handle mouse scroll
ipcMain.handle('control:mouse-scroll', async (event, { deltaX, deltaY }) => {
  try {
    if (deltaY !== 0) {
      await mouse.scrollDown(Math.abs(deltaY));
    }
    if (deltaX !== 0) {
      await mouse.scrollRight(Math.abs(deltaX));
    }
    return { success: true };
  } catch (error) {
    console.error('Mouse scroll error:', error);
    return { success: false, error: error.message };
  }
});

// Handle keyboard press
ipcMain.handle('control:keyboard-press', async (event, { key, modifiers }) => {
  try {
    // Handle modifiers
    const keysToPress = [];
    
    if (modifiers?.ctrl) keysToPress.push(Key.LeftControl);
    if (modifiers?.alt) keysToPress.push(Key.LeftAlt);
    if (modifiers?.shift) keysToPress.push(Key.LeftShift);
    if (modifiers?.meta) keysToPress.push(Key.LeftSuper);
    
    // Map key to Nut.js Key
    const nutKey = mapKeyToNutJs(key);
    if (nutKey) keysToPress.push(nutKey);
    
    // Press all keys
    for (const k of keysToPress) {
      await keyboard.pressKey(k);
    }
    
    // Release in reverse order
    for (let i = keysToPress.length - 1; i >= 0; i--) {
      await keyboard.releaseKey(keysToPress[i]);
    }
    
    return { success: true };
  } catch (error) {
    console.error('Keyboard press error:', error);
    return { success: false, error: error.message };
  }
});

// Handle keyboard type (text input)
ipcMain.handle('control:keyboard-type', async (event, { text }) => {
  try {
    await keyboard.type(text);
    return { success: true };
  } catch (error) {
    console.error('Keyboard type error:', error);
    return { success: false, error: error.message };
  }
});

// Get available screen sources for capture
ipcMain.handle('get-sources', async () => {
  try {
    const sources = await desktopCapturer.getSources({
      types: ['screen', 'window'],
      thumbnailSize: { width: 150, height: 150 }
    });
    return sources;
  } catch (error) {
    console.error('Get sources error:', error);
    return [];
  }
});

// ============================================================================
// Helper Functions
// ============================================================================

function mapKeyToNutJs(key) {
  // Map common keys to Nut.js Key enum
  const keyMap = {
    'Enter': Key.Enter,
    'Backspace': Key.Backspace,
    'Delete': Key.Delete,
    'Tab': Key.Tab,
    'Escape': Key.Escape,
    'Space': Key.Space,
    'ArrowUp': Key.Up,
    'ArrowDown': Key.Down,
    'ArrowLeft': Key.Left,
    'ArrowRight': Key.Right,
    'Home': Key.Home,
    'End': Key.End,
    'PageUp': Key.PageUp,
    'PageDown': Key.PageDown,
    'Insert': Key.Insert,
    'F1': Key.F1,
    'F2': Key.F2,
    'F3': Key.F3,
    'F4': Key.F4,
    'F5': Key.F5,
    'F6': Key.F6,
    'F7': Key.F7,
    'F8': Key.F8,
    'F9': Key.F9,
    'F10': Key.F10,
    'F11': Key.F11,
    'F12': Key.F12,
  };
  
  // Check if it's a mapped key
  if (keyMap[key]) {
    return keyMap[key];
  }
  
  // For single character keys, use the character directly
  if (key.length === 1) {
    return Key[key.toUpperCase()] || null;
  }
  
  return null;
}
