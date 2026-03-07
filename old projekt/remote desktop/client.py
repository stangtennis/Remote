import socket
import json
import tkinter as tk
from PIL import Image, ImageTk
import io
import threading
import time

class RemoteDesktopClient:
    def __init__(self, host='localhost', port=5000):
        self.host = host
        self.port = port
        self.connected = False
        self.running = True
        
        # Create GUI
        self.root = tk.Tk()
        self.root.title("Remote Desktop Client")
        self.root.protocol("WM_DELETE_WINDOW", self.on_closing)
        
        # Create status label
        self.status_label = tk.Label(self.root, text="Disconnected", fg="red")
        self.status_label.pack()
        
        # Create canvas for displaying remote screen
        self.canvas = tk.Canvas(self.root, width=800, height=600, bg='black')
        self.canvas.pack()
        
        # Bind mouse events
        self.canvas.bind("<Motion>", self.on_mouse_move)
        self.canvas.bind("<Button-1>", self.on_mouse_click)
        self.root.bind("<Key>", self.on_key_press)
        
        # Start connection thread
        self.connect_thread = threading.Thread(target=self.maintain_connection)
        self.connect_thread.daemon = True
        self.connect_thread.start()
    
    def connect(self):
        try:
            self.client = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            self.client.connect((self.host, self.port))
            self.connected = True
            self.status_label.config(text="Connected", fg="green")
            print("Connected to server")
            return True
        except Exception as e:
            print(f"Connection error: {e}")
            self.connected = False
            self.status_label.config(text=f"Connection failed: {str(e)}", fg="red")
            return False
    
    def maintain_connection(self):
        while self.running:
            if not self.connected:
                if self.connect():
                    self.update_screen()
            time.sleep(1)
    
    def send_command(self, command):
        if not self.connected:
            return
        try:
            self.client.send(json.dumps(command).encode())
        except:
            self.connected = False
            self.status_label.config(text="Connection lost", fg="red")
    
    def receive_data(self, size):
        data = b""
        try:
            while len(data) < size:
                chunk = self.client.recv(min(4096, size - len(data)))
                if not chunk:
                    raise socket.error("Connection closed")
                data += chunk
            return data
        except socket.error as e:
            print(f"Error receiving data: {e}")
            self.connected = False
            raise
    
    def update_screen(self):
        def update():
            while self.connected and self.running:
                try:
                    # Request screen update
                    self.send_command({"type": "screen"})
                    
                    # Receive image size
                    size_data = self.client.recv(16).strip()
                    if not size_data:
                        raise socket.error("Connection closed")
                    
                    size = int(size_data)
                    
                    # Receive image data
                    data = self.receive_data(size)
                    
                    # Convert to image and display
                    image = Image.open(io.BytesIO(data))
                    photo = ImageTk.PhotoImage(image)
                    self.canvas.create_image(0, 0, image=photo, anchor="nw")
                    self.canvas.image = photo
                    
                    time.sleep(0.1)  # Limit update rate
                    
                except Exception as e:
                    print(f"Screen update error: {e}")
                    self.connected = False
                    self.status_label.config(text="Connection lost", fg="red")
                    break
        
        screen_thread = threading.Thread(target=update)
        screen_thread.daemon = True
        screen_thread.start()
    
    def on_mouse_move(self, event):
        command = {
            "type": "mouse",
            "x": event.x,
            "y": event.y
        }
        self.send_command(command)
    
    def on_mouse_click(self, event):
        command = {
            "type": "mouse",
            "x": event.x,
            "y": event.y,
            "click": True
        }
        self.send_command(command)
    
    def on_key_press(self, event):
        command = {
            "type": "keyboard",
            "key": event.char
        }
        self.send_command(command)
    
    def on_closing(self):
        self.running = False
        if self.connected:
            try:
                self.client.close()
            except:
                pass
        self.root.destroy()
    
    def start(self):
        self.root.mainloop()

if __name__ == "__main__":
    client = RemoteDesktopClient()
    client.start()
