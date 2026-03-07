import sys
import socket
import platform
import os
from PyQt6.QtWidgets import (QApplication, QMainWindow, QWidget, QVBoxLayout, 
                            QLabel, QMessageBox, QPushButton, QTextEdit, QHBoxLayout)
from PyQt6.QtCore import Qt, QTimer, QThread, pyqtSignal, QByteArray, QBuffer, QIODevice
from PyQt6.QtGui import QImage, QPixmap, QScreen
import socketio
import logging
import base64
from PIL import ImageGrab
import io
import time

# Set up logging
logging.basicConfig(level=logging.DEBUG,
                   format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class ScreenCaptureThread(QThread):
    """
    Thread for capturing screen frames at a specified FPS.
    
    Attributes:
        frame_ready (pyqtSignal): Signal emitted when a new frame is captured
        running (bool): Controls the thread execution state
        fps (int): Frames per second for capture rate
        frame_interval (float): Time between captures (1/fps)
    """
    frame_ready = pyqtSignal(QImage)
    error_occurred = pyqtSignal(str)

    def __init__(self):
        super().__init__()
        self.running = False
        self.fps = 10
        self.frame_interval = 1 / self.fps
        
    def start_capture(self):
        self.running = True
        if not self.isRunning():
            self.start()

    def stop_capture(self):
        self.running = False
        self.wait()

    def run(self):
        """
        Main thread loop for capturing screen frames.
        """
        while self.running:
            try:
                screen = QApplication.primaryScreen()
                if screen is not None:
                    pixmap = screen.grabWindow(0)
                    image = pixmap.toImage()
                    self.frame_ready.emit(image)
                time.sleep(self.frame_interval)
            except Exception as e:
                error_msg = f"Screen capture error: {str(e)}"
                logger.error(error_msg)
                self.error_occurred.emit(error_msg)
                time.sleep(1)
    
    def stop(self):
        """
        Stop the screen capture thread.
        """
        self.running = False

class RemoteDesktopClient(QMainWindow):
    """
    Main client application window for remote desktop functionality.
    
    Attributes:
        sio (socketio.Client): Socket.IO client instance
        screen_capture_thread (ScreenCaptureThread): Thread for screen capture
    """
    def __init__(self):
        super().__init__()
        logger.debug("Initializing RemoteDesktopClient")
        self.setWindowTitle("Remote Desktop Client")
        self.setGeometry(100, 100, 800, 600)

        # Create central widget and layout
        central_widget = QWidget()
        self.setCentralWidget(central_widget)
        layout = QVBoxLayout(central_widget)

        # Create status label
        self.status_label = QLabel("Initializing...")
        self.status_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(self.status_label)

        # Create screen display label
        self.screen_label = QLabel()
        self.screen_label.setMinimumSize(640, 480)
        self.screen_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        layout.addWidget(self.screen_label)

        # Create button layout
        button_layout = QHBoxLayout()
        
        # Create connection buttons
        self.connect_button = QPushButton("Connect")
        self.connect_button.clicked.connect(self.connect_to_server)
        button_layout.addWidget(self.connect_button)

        self.disconnect_button = QPushButton("Disconnect")
        self.disconnect_button.clicked.connect(self.disconnect_from_server)
        self.disconnect_button.setEnabled(False)
        button_layout.addWidget(self.disconnect_button)

        # Create screen capture buttons
        self.start_capture_button = QPushButton("Start Capture")
        self.start_capture_button.clicked.connect(self.start_screen_capture)
        self.start_capture_button.setEnabled(False)
        button_layout.addWidget(self.start_capture_button)

        self.stop_capture_button = QPushButton("Stop Capture")
        self.stop_capture_button.clicked.connect(self.stop_screen_capture)
        self.stop_capture_button.setEnabled(False)
        button_layout.addWidget(self.stop_capture_button)

        layout.addLayout(button_layout)

        # Create debug log area
        self.log_area = QTextEdit()
        self.log_area.setReadOnly(True)
        self.log_area.setMaximumHeight(100)
        layout.addWidget(self.log_area)

        # Initialize Socket.IO client with Engine.IO configuration
        engineio_config = {
            'reconnection': True,
            'reconnection_attempts': 5,
            'reconnection_delay': 1,
            'reconnection_delay_max': 5
        }
        self.sio = socketio.Client(
            logger=True,
            engineio_logger=True,
            **engineio_config
        )
        self.setup_socket_events()
        
        # Initialize screen capture thread
        self.capture_thread = ScreenCaptureThread()
        self.capture_thread.frame_ready.connect(self.update_screen)
        self.capture_thread.error_occurred.connect(self.handle_capture_error)

        # Initialize control state
        self.is_controlled = False
        self.is_locked = False
        
        # Add timer for periodic connection check
        self.check_timer = QTimer()
        self.check_timer.timeout.connect(self.check_connection)
        self.check_timer.start(5000)  # Check every 5 seconds

        self.log_message("Client initialized")

    def log_message(self, message):
        logger.debug(message)
        self.log_area.append(f"{message}")
        self.log_area.verticalScrollBar().setValue(
            self.log_area.verticalScrollBar().maximum()
        )

    def setup_socket_events(self):
        @self.sio.event
        def connect():
            self.log_message("Connected to server")
            self.status_label.setText("Connected to server")
            self.status_label.setStyleSheet("color: green")
            self.connect_button.setEnabled(False)
            self.disconnect_button.setEnabled(True)
            self.start_capture_button.setEnabled(True)
            self.register_client()
            
        @self.sio.event
        def disconnect():
            self.log_message("Disconnected from server")
            self.status_label.setText("Disconnected from server")
            self.status_label.setStyleSheet("color: red")
            self.connect_button.setEnabled(True)
            self.disconnect_button.setEnabled(False)
            self.start_capture_button.setEnabled(False)
            self.stop_capture_button.setEnabled(False)
            self.stop_screen_capture()
            self.is_controlled = False
            self.is_locked = False
            
        @self.sio.event
        def connect_error(data):
            error_msg = f"Connection error: {data}"
            self.log_message(error_msg)
            self.status_label.setText(error_msg)
            self.status_label.setStyleSheet("color: red")
            self.connect_button.setEnabled(True)
            self.disconnect_button.setEnabled(False)
            self.start_capture_button.setEnabled(False)
            
        @self.sio.event
        def admin_command(data):
            try:
                command = data.get('command')
                self.log_message(f"Received admin command: {command}")

                if command == 'view_screen':
                    self.handle_view_screen()
                elif command == 'take_control':
                    self.handle_take_control()
                elif command == 'stop_control':
                    self.handle_stop_control()
                elif command == 'message':
                    self.handle_message(data.get('message', ''))
                elif command == 'lock':
                    self.handle_lock()
                elif command == 'unlock':
                    self.handle_unlock()
                else:
                    self.log_message(f"Unknown command: {command}")
            except Exception as e:
                self.log_message(f"Error handling admin command: {str(e)}")

    def handle_view_screen(self):
        if not self.capture_thread.isRunning():
            self.capture_thread.start_capture()
            self.log_message("Screen capture started")

    def handle_take_control(self):
        try:
            if not self.is_locked:
                self.log_message("Admin taking control")
                self.is_controlled = True
                self.status_label.setText("Admin has control")
                self.status_label.setStyleSheet("color: orange")
                # Start screen capture if not already running
                if not self.capture_thread.running:
                    self.start_screen_capture()
                # Disable local controls
                self.start_capture_button.setEnabled(False)
                self.stop_capture_button.setEnabled(False)
            else:
                self.log_message("Cannot take control - client is locked")
        except Exception as e:
            self.log_message(f"Error handling take control: {str(e)}")

    def handle_stop_control(self):
        try:
            self.log_message("Admin releasing control")
            self.is_controlled = False
            self.status_label.setText("Connected to server")
            self.status_label.setStyleSheet("color: green")
            # Re-enable local controls if not locked
            if not self.is_locked:
                self.start_capture_button.setEnabled(True)
                if self.capture_thread.running:
                    self.stop_capture_button.setEnabled(True)
        except Exception as e:
            self.log_message(f"Error handling stop control: {str(e)}")

    def handle_message(self, message):
        try:
            self.log_message(f"Message from admin: {message}")
            QMessageBox.information(self, "Message from Admin", message)
        except Exception as e:
            self.log_message(f"Error handling message: {str(e)}")

    def handle_lock(self):
        try:
            self.log_message("Client locked by admin")
            self.is_locked = True
            self.status_label.setText("Client locked by admin")
            self.status_label.setStyleSheet("color: red")
            # Disable all controls
            self.start_capture_button.setEnabled(False)
            self.stop_capture_button.setEnabled(False)
            # Stop screen capture if running
            if self.capture_thread.running and not self.is_controlled:
                self.stop_screen_capture()
        except Exception as e:
            self.log_message(f"Error handling lock: {str(e)}")

    def handle_unlock(self):
        try:
            self.log_message("Client unlocked by admin")
            self.is_locked = False
            if self.is_controlled:
                self.status_label.setText("Admin has control")
                self.status_label.setStyleSheet("color: orange")
            else:
                self.status_label.setText("Connected to server")
                self.status_label.setStyleSheet("color: green")
                self.start_capture_button.setEnabled(True)
                if self.capture_thread.running:
                    self.stop_capture_button.setEnabled(True)
        except Exception as e:
            self.log_message(f"Error handling unlock: {str(e)}")

    def connect_to_server(self, server_url="http://localhost:8000"):
        if not isinstance(server_url, str):
            self.log_message(f"Invalid server URL: {server_url}")
            self.status_label.setText("Invalid server URL")
            self.status_label.setStyleSheet("color: red")
            return

        # Ensure URL has proper scheme
        if not server_url.startswith(('http://', 'https://')):
            server_url = 'http://' + server_url

        # Store the server URL for retries
        self.server_url = server_url

        # Set up connection timeout timer
        self.connection_timer = QTimer()
        self.connection_timer.setSingleShot(True)
        self.connection_timer.timeout.connect(self.handle_connection_timeout)

        try:
            self.log_message(f"Attempting to connect to server: {server_url}")
            self.connection_timer.start(60000)  # 60 second timeout
            self.sio.connect(server_url, 
                           transports=['websocket'])

            self.status_label.setText("Connected to server")
            self.status_label.setStyleSheet("color: green")
            self.connect_button.setEnabled(False)
            self.disconnect_button.setEnabled(True)
            self.start_capture_button.setEnabled(True)

        except socketio.exceptions.ConnectionError as e:
            self.connection_timer.stop()
            error_msg = f"Connection error: {str(e)}"
            self.log_message(error_msg)
            self.status_label.setText("Connection failed")
            self.status_label.setStyleSheet("color: red")
            # Retry connection after 5 seconds
            QTimer.singleShot(5000, lambda: self.connect_to_server(self.server_url))

        except Exception as e:
            self.connection_timer.stop()
            error_msg = f"Unexpected error: {str(e)}"
            self.log_message(error_msg)
            self.status_label.setText("Connection error")
            self.status_label.setStyleSheet("color: red")

    def handle_connection_timeout(self):
        self.log_message("Connection attempt timed out")
        self.status_label.setText("Connection timeout")
        self.status_label.setStyleSheet("color: red")
        self.sio.disconnect()
        # Retry connection after 5 seconds
        QTimer.singleShot(5000, lambda: self.connect_to_server(self.server_url))

    def disconnect_from_server(self):
        try:
            self.log_message("Disconnecting from server")
            self.stop_screen_capture()
            self.sio.disconnect()
        except Exception as e:
            self.log_message(f"Error disconnecting: {str(e)}")

    def start_screen_capture(self):
        try:
            if not self.is_locked or self.is_controlled:
                self.log_message("Starting screen capture")
                self.capture_thread.start_capture()
                self.start_capture_button.setEnabled(False)
                self.stop_capture_button.setEnabled(True)
            else:
                self.log_message("Cannot start capture - client is locked")
        except Exception as e:
            self.log_message(f"Error starting capture: {str(e)}")

    def stop_screen_capture(self):
        try:
            if self.capture_thread.running and not self.is_controlled:
                self.log_message("Stopping screen capture")
                self.capture_thread.stop_capture()
                if not self.is_locked:
                    self.start_capture_button.setEnabled(True)
                self.stop_capture_button.setEnabled(False)
            elif self.is_controlled:
                self.log_message("Cannot stop capture - admin has control")
        except Exception as e:
            self.log_message(f"Error stopping capture: {str(e)}")

    def update_screen(self, image):
        try:
            # Convert QImage to bytes
            buffer = QBuffer()
            buffer.open(QBuffer.OpenModeFlag.ReadWrite)
            image.save(buffer, "JPEG") # Changed from PNG to JPEG
            image_bytes = buffer.data()
            buffer.close()
            
            # Convert to base64 and send
            image_base64 = base64.b64encode(image_bytes).decode('utf-8')
            self.sio.emit('screen_data', {
                'image': image_base64
            })
        except Exception as e:
            self.log_message(f"Error sending screen data: {str(e)}")

    def handle_capture_error(self, error_msg):
        self.log_message(error_msg)

    def check_connection(self):
        try:
            if self.sio.connected:
                self.log_message("Connection check: Connected")
            else:
                self.log_message("Connection check: Disconnected")
        except Exception as e:
            self.log_message(f"Connection check error: {str(e)}")

    def register_client(self):
        try:
            hostname = platform.node()
            username = os.getlogin()
            ip = socket.gethostbyname(socket.gethostname())
            
            self.sio.emit('register_client', {
                'hostname': hostname,
                'username': username,
                'ip': ip
            })
            self.log_message(f"Registered client: {hostname} ({username})")
        except Exception as e:
            self.log_message(f"Error registering client: {str(e)}")

    def closeEvent(self, event):
        self.log_message("Closing application")
        self.check_timer.stop()
        self.stop_screen_capture()
        if self.sio.connected:
            self.sio.disconnect()
        super().closeEvent(event)

if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description='Remote Desktop Client')
    parser.add_argument('--server', default='192.168.1.90:8000',
                       help='Server address to connect to')
    args = parser.parse_args()

    app = QApplication(sys.argv)
    client = RemoteDesktopClient()
    client.show()
    client.connect_to_server(args.server)
    sys.exit(app.exec())
