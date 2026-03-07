import logging
import win32api
import win32con
import win32com.client
import time

logger = logging.getLogger(__name__)

class InputHandler:
    def __init__(self):
        self.shell = win32com.client.Dispatch("WScript.Shell")
        self.VIRTUAL_KEYS = {
            'Backspace': 0x08,
            'Tab': 0x09,
            'Enter': 0x0D,
            'Shift': 0x10,
            'Control': 0x11,
            'Alt': 0x12,
            'Pause': 0x13,
            'CapsLock': 0x14,
            'Escape': 0x1B,
            'Space': 0x20,
            'PageUp': 0x21,
            'PageDown': 0x22,
            'End': 0x23,
            'Home': 0x24,
            'ArrowLeft': 0x25,
            'ArrowUp': 0x26,
            'ArrowRight': 0x27,
            'ArrowDown': 0x28,
            'Insert': 0x2D,
            'Delete': 0x2E,
            'F1': 0x70,
            'F2': 0x71,
            'F3': 0x72,
            'F4': 0x73,
            'F5': 0x74,
            'F6': 0x75,
            'F7': 0x76,
            'F8': 0x77,
            'F9': 0x78,
            'F10': 0x79,
            'F11': 0x7A,
            'F12': 0x7B,
        }
        # Add number keys
        for i in range(10):
            self.VIRTUAL_KEYS[str(i)] = 0x30 + i
        # Add letter keys
        for i in range(26):
            self.VIRTUAL_KEYS[chr(65 + i)] = 0x41 + i
            self.VIRTUAL_KEYS[chr(97 + i)] = 0x41 + i

    def move_mouse(self, x: int, y: int) -> None:
        """Move mouse to absolute position"""
        try:
            win32api.SetCursorPos((x, y))
        except Exception as e:
            logger.error(f"Error moving mouse: {str(e)}")

    def click_mouse(self, x: int, y: int, button: str = 'left', action: str = 'down') -> None:
        """Perform mouse click at position"""
        try:
            # Move mouse to position first
            self.move_mouse(x, y)
            
            # Determine event based on button and action
            if button == 'left':
                event = win32con.MOUSEEVENTF_LEFTDOWN if action == 'down' else win32con.MOUSEEVENTF_LEFTUP
            elif button == 'right':
                event = win32con.MOUSEEVENTF_RIGHTDOWN if action == 'down' else win32con.MOUSEEVENTF_RIGHTUP
            elif button == 'middle':
                event = win32con.MOUSEEVENTF_MIDDLEDOWN if action == 'down' else win32con.MOUSEEVENTF_MIDDLEUP
            else:
                return
            
            win32api.mouse_event(event, 0, 0, 0, 0)
        except Exception as e:
            logger.error(f"Error clicking mouse: {str(e)}")

    def scroll_mouse(self, delta: int) -> None:
        """Scroll mouse wheel"""
        try:
            win32api.mouse_event(win32con.MOUSEEVENTF_WHEEL, 0, 0, delta, 0)
        except Exception as e:
            logger.error(f"Error scrolling mouse: {str(e)}")

    def handle_key(self, key: str, action: str = 'down', ctrl: bool = False, alt: bool = False, shift: bool = False) -> None:
        """Handle keyboard input with modifiers"""
        try:
            # Handle modifier keys
            if ctrl:
                win32api.keybd_event(win32con.VK_CONTROL, 0, 0, 0)
            if alt:
                win32api.keybd_event(win32con.VK_MENU, 0, 0, 0)
            if shift:
                win32api.keybd_event(win32con.VK_SHIFT, 0, 0, 0)

            # Handle the main key
            if key in self.VIRTUAL_KEYS:
                vk_code = self.VIRTUAL_KEYS[key]
                flags = 0 if action == 'down' else win32con.KEYEVENTF_KEYUP
                win32api.keybd_event(vk_code, 0, flags, 0)
            elif len(key) == 1:  # Single character
                if action == 'down':
                    self.shell.SendKeys(key)

            # Release modifier keys in reverse order
            if shift:
                win32api.keybd_event(win32con.VK_SHIFT, 0, win32con.KEYEVENTF_KEYUP, 0)
            if alt:
                win32api.keybd_event(win32con.VK_MENU, 0, win32con.KEYEVENTF_KEYUP, 0)
            if ctrl:
                win32api.keybd_event(win32con.VK_CONTROL, 0, win32con.KEYEVENTF_KEYUP, 0)

        except Exception as e:
            logger.error(f"Error handling key press: {str(e)}")

    def handle_mouse_event(self, x: float, y: float, event_type: str, button: str = 'left', wheel_delta: int = 0) -> None:
        """Handle mouse events with normalized coordinates"""
        try:
            # Convert normalized coordinates to screen coordinates
            screen_x = int(x * win32api.GetSystemMetrics(win32con.SM_CXSCREEN))
            screen_y = int(y * win32api.GetSystemMetrics(win32con.SM_CYSCREEN))
            
            if event_type == 'move':
                self.move_mouse(screen_x, screen_y)
            elif event_type == 'down':
                self.click_mouse(screen_x, screen_y, button, 'down')
            elif event_type == 'up':
                self.click_mouse(screen_x, screen_y, button, 'up')
            elif event_type == 'wheel':
                self.scroll_mouse(wheel_delta)
        except Exception as e:
            logger.error(f"Error handling mouse event: {str(e)}")
