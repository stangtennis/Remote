import socket
import threading
import json
import mss
import pyautogui
import keyboard
from PIL import Image
import io
import base64

class RemoteDesktopServer:
    def __init__(self, host='0.0.0.0', port=5000):
        self.server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.server.bind((host, port))
        self.server.listen(1)
        self.sct = mss.mss()
        print(f"Server listening on {host}:{port}")
        
    def handle_client(self, client_socket):
        try:
            while True:
                try:
                    # Receive command from client
                    data = client_socket.recv(1024).decode()
                    if not data:
                        print("Client disconnected")
                        break
                    
                    command = json.loads(data)
                    
                    if command['type'] == 'screen':
                        # Capture screen
                        screenshot = self.sct.grab(self.sct.monitors[0])
                        img = Image.frombytes('RGB', screenshot.size, screenshot.rgb)
                        
                        # Resize to reduce bandwidth
                        img = img.resize((800, 600), Image.Resampling.LANCZOS)
                        
                        # Convert to bytes with higher compression
                        img_bytes = io.BytesIO()
                        img.save(img_bytes, format='JPEG', quality=30)
                        img_bytes = img_bytes.getvalue()
                        
                        try:
                            # Send size first
                            size = len(img_bytes)
                            client_socket.send(str(size).encode().ljust(16))
                            
                            # Send image data in chunks
                            chunk_size = 4096
                            for i in range(0, len(img_bytes), chunk_size):
                                chunk = img_bytes[i:i + chunk_size]
                                client_socket.send(chunk)
                        except socket.error:
                            print("Error sending screen data")
                            break
                    
                    elif command['type'] == 'mouse':
                        x, y = command['x'], command['y']
                        if 'click' in command:
                            pyautogui.click(x, y)
                        else:
                            pyautogui.moveTo(x, y)
                    
                    elif command['type'] == 'keyboard':
                        keyboard.write(command['key'])
                
                except json.JSONDecodeError:
                    print("Invalid command received")
                    continue
                except socket.error as e:
                    print(f"Socket error: {e}")
                    break
                except Exception as e:
                    print(f"Error processing command: {e}")
                    continue
                    
        except Exception as e:
            print(f"Client handler error: {e}")
        finally:
            try:
                client_socket.close()
                print("Client connection closed")
            except:
                pass

    def start(self):
        print("Starting server...")
        try:
            while True:
                try:
                    client_socket, addr = self.server.accept()
                    print(f"Accepted connection from {addr}")
                    client_thread = threading.Thread(target=self.handle_client, args=(client_socket,))
                    client_thread.daemon = True
                    client_thread.start()
                except socket.error as e:
                    print(f"Error accepting connection: {e}")
                    continue
        except KeyboardInterrupt:
            print("\nServer shutting down...")
        finally:
            self.server.close()
            print("Server stopped")

if __name__ == "__main__":
    server = RemoteDesktopServer()
    server.start()
