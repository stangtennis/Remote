import sys
import logging
from PyQt6.QtWidgets import (QApplication, QMainWindow, QWidget, QVBoxLayout, 
                            QLabel, QListWidget, QPushButton, QTextEdit, QHBoxLayout,
                            QMessageBox, QInputDialog, QSplitter)
from PyQt6.QtCore import Qt, QTimer, QByteArray
from PyQt6.QtGui import QImage, QPixmap
import socketio
import base64

# Set up logging
logging.basicConfig(level=logging.DEBUG,
                   format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class AdminPanel(QMainWindow):
    """
    Admin panel for managing remote desktop clients.
    
    Attributes:
        selected_client (str): Currently selected client ID
        viewing_client (str): Currently viewing client ID
        original_width (int): Default screen width
        original_height (int): Default screen height
        sio (socketio.Client): Socket.IO client instance
        clients (dict): Dictionary of connected clients
    """
    def __init__(self):
        super().__init__()
        logger.debug("Initializing AdminPanel")
        self.setWindowTitle("Remote Desktop Admin Panel")
        self.setGeometry(100, 100, 1200, 800)
        
        # Initialize variables
        self.selected_client = None
        self.viewing_client = None
        self.clients = {}
        self.original_width = 1920
        self.original_height = 1080
        
        self.sio = socketio.Client(logger=True, engineio_logger=True)
        self.setup_socket_events()
        self.setup_ui()

    def setup_ui(self):
        """
        Initialize and configure the user interface.
        """
        self.central_widget = QWidget()
        self.setCentralWidget(self.central_widget)
        self.layout = QVBoxLayout(self.central_widget)
        
        # Create status label
        self.status_label = QLabel("Initializing...")
        self.status_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self.layout.addWidget(self.status_label)

        # Create splitter for client list and screen view
        splitter = QSplitter(Qt.Orientation.Horizontal)
        self.layout.addWidget(splitter)

        # Create left panel for client list and controls
        self.left_panel = QWidget()
        self.left_layout = QVBoxLayout(self.left_panel)
        
        # Create client list
        self.left_layout.addWidget(QLabel("Connected Clients:"))
        self.client_list = QListWidget()
        self.client_list.setMinimumWidth(300)
        self.left_layout.addWidget(self.client_list)

        # Create button layout
        self.button_layout = QVBoxLayout()

        # Create connection buttons
        self.connection_layout = QHBoxLayout()
        self.connect_button = QPushButton("Connect to Server")
        self.connect_button.clicked.connect(self.connect_to_server)
        self.connection_layout.addWidget(self.connect_button)

        self.disconnect_button = QPushButton("Disconnect")
        self.disconnect_button.clicked.connect(self.disconnect_from_server)
        self.disconnect_button.setEnabled(False)
        self.connection_layout.addWidget(self.disconnect_button)
        self.button_layout.addLayout(self.connection_layout)

        # Create control buttons
        self.control_layout = QHBoxLayout()
        self.view_button = QPushButton("View Screen")
        self.view_button.clicked.connect(self.start_viewing)
        self.view_button.setEnabled(False)
        self.control_layout.addWidget(self.view_button)

        self.stop_view_button = QPushButton("Stop Viewing")
        self.stop_view_button.clicked.connect(self.stop_viewing)
        self.stop_view_button.setEnabled(False)
        self.control_layout.addWidget(self.stop_view_button)
        self.button_layout.addLayout(self.control_layout)

        # Create control buttons
        self.control_layout = QHBoxLayout()
        self.control_button = QPushButton("Take Control")
        self.control_button.clicked.connect(self.control_selected_client)
        self.control_button.setEnabled(False)
        self.control_layout.addWidget(self.control_button)

        self.stop_control_button = QPushButton("Stop Control")
        self.stop_control_button.clicked.connect(self.stop_control)
        self.stop_control_button.setEnabled(False)
        self.control_layout.addWidget(self.stop_control_button)
        self.button_layout.addLayout(self.control_layout)

        # Create message and lock buttons
        self.action_layout = QHBoxLayout()
        self.message_button = QPushButton("Send Message")
        self.message_button.clicked.connect(self.send_message)
        self.message_button.setEnabled(False)
        self.action_layout.addWidget(self.message_button)

        self.lock_button = QPushButton("Lock Client")
        self.lock_button.clicked.connect(self.lock_selected_client)
        self.lock_button.setEnabled(False)
        self.action_layout.addWidget(self.lock_button)

        self.unlock_button = QPushButton("Unlock Client")
        self.unlock_button.clicked.connect(self.unlock_selected_client)
        self.unlock_button.setEnabled(False)
        self.action_layout.addWidget(self.unlock_button)
        self.button_layout.addLayout(self.action_layout)

        self.left_layout.addLayout(self.button_layout)

        # Create debug log area
        self.log_area = QTextEdit()
        self.log_area.setReadOnly(True)
        self.log_area.setMaximumHeight(150)
        self.left_layout.addWidget(QLabel("Debug Log:"))
        self.left_layout.addWidget(self.log_area)

        splitter.addWidget(self.left_panel)

        # Create right panel for screen view
        self.right_panel = QWidget()
        self.right_layout = QVBoxLayout(self.right_panel)
        
        # Create screen view label
        self.right_layout.addWidget(QLabel("Remote Screen:"))
        self.screen_label = QLabel()
        self.screen_label.setMinimumSize(800, 600)
        self.screen_label.setAlignment(Qt.AlignmentFlag.AlignCenter)
        self.screen_label.setStyleSheet("QLabel { background-color: black; color: white; }")
        self.screen_label.setText("No screen data")
        self.right_layout.addWidget(self.screen_label)
        
        splitter.addWidget(self.right_panel)

        # Set splitter sizes
        splitter.setSizes([400, 800])

        # Initial button states
        self.disconnect_button.setEnabled(False)
        self.view_button.setEnabled(False)
        self.control_button.setEnabled(False)
        self.message_button.setEnabled(False)
        self.lock_button.setEnabled(False)
        self.unlock_button.setEnabled(False)

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
            self.register_admin()

        @self.sio.event
        def disconnect():
            self.log_message("Disconnected from server")
            self.status_label.setText("Disconnected from server")
            self.status_label.setStyleSheet("color: red")
            self.connect_button.setEnabled(True)
            self.disconnect_button.setEnabled(False)
            self.view_button.setEnabled(False)
            self.client_list.clear()
            self.clients.clear()

        @self.sio.event
        def connect_error(data):
            error_msg = f"Connection error: {data}"
            self.log_message(error_msg)
            self.status_label.setText(error_msg)
            self.status_label.setStyleSheet("color: red")
            self.connect_button.setEnabled(True)
            self.disconnect_button.setEnabled(False)
            self.view_button.setEnabled(False)

        @self.sio.event
        def client_connected(data):
            try:
                client_id = data['client_id']
                client_info = data['client_info']
                item = f"{client_id} - {client_info['hostname']} ({client_info['username']})"
                self.client_list.addItem(item)
                self.view_button.setEnabled(True)
                self.log_message(f"Client connected: {client_info}")
            except Exception as e:
                self.log_message(f"Error handling client connection: {str(e)}")

        @self.sio.event
        def client_disconnected(data):
            try:
                client_id = data.get('client_id')
                if client_id in self.clients:
                    client_info = self.clients[client_id]
                    self.log_message(f"Client disconnected: {client_info}")
                    
                    # Remove from dictionary
                    del self.clients[client_id]
                    
                    # Update list widget
                    self.client_list.clear()
                    for info in self.clients.values():
                        display_text = f"{info.get('hostname', 'Unknown')} ({info.get('ip', 'Unknown')})"
                        self.client_list.addItem(display_text)
                    
                    # Disable buttons if no clients
                    self.enable_client_buttons(len(self.clients) > 0)
            except Exception as e:
                self.log_message(f"Error handling client disconnection: {str(e)}")

        @self.sio.event
        def screen_data(data):
            self.handle_screen_data(data)

    def handle_screen_data(self, data):
        try:
            if self.viewing_client and self.viewing_client == data['client_id']:
                # Decode base64 image
                image_data = base64.b64decode(data['image'])
                image = QImage.fromData(image_data)
                
                # Scale and display
                pixmap = QPixmap.fromImage(image)
                scaled_pixmap = pixmap.scaled(
                    self.screen_label.size(), 
                    Qt.AspectRatioMode.KeepAspectRatio, 
                    Qt.TransformationMode.SmoothTransformation
                )
                self.screen_label.setPixmap(scaled_pixmap)
        except Exception as e:
            self.log_message(f"Error handling screen data: {str(e)}")

    def start_viewing(self):
        if self.selected_client:
            self.viewing_client = self.selected_client
            self.sio.emit('request_screen', {'client_id': self.selected_client})
            self.view_button.setEnabled(False)
            self.stop_view_button.setEnabled(True)
            self.screen_label.setText("Waiting for screen data...")

    def stop_viewing(self):
        if self.viewing_client:
            self.sio.emit('stop_screen', {'client_id': self.viewing_client})
            self.viewing_client = None
            self.view_button.setEnabled(True)
            self.stop_view_button.setEnabled(False)
            self.screen_label.setText("No screen data")
            self.screen_label.setPixmap(QPixmap())

    def connect_to_server(self):
        try:
            server_url = f"http://{args.server}"
            self.log_message(f"Attempting to connect to server: {server_url}")
            self.sio.connect(server_url)
            # Register as admin with admin key
            self.sio.emit('register_admin', {'admin_key': 'your_admin_key'})
            self.status_label.setText("Connected to server")
            self.status_label.setStyleSheet("color: green")
            self.connect_button.setEnabled(False)
            self.disconnect_button.setEnabled(True)
            self.view_button.setEnabled(True)
            self.control_button.setEnabled(True)
            self.message_button.setEnabled(True)
            self.lock_button.setEnabled(True)
            self.unlock_button.setEnabled(True)
        except Exception as e:
            error_msg = f"Connection error: {str(e)}"
            self.log_message(error_msg)
            self.status_label.setText(error_msg)
            self.status_label.setStyleSheet("color: red")

    def disconnect_from_server(self):
        try:
            self.log_message("Disconnecting from server")
            self.sio.disconnect()
        except Exception as e:
            self.log_message(f"Error disconnecting: {str(e)}")

    def register_admin(self):
        try:
            self.sio.emit('register_admin', {
                'admin_key': 'your_admin_key'
            })
            self.log_message("Admin registered")
        except Exception as e:
            self.log_message(f"Error registering admin: {str(e)}")

    def control_selected_client(self):
        try:
            current_item = self.client_list.currentItem()
            if current_item:
                selected_text = current_item.text()
                client_id = self.get_client_id_from_text(selected_text)
                if client_id:
                    self.log_message(f"Taking control of client: {selected_text}")
                    self.send_control_command('take_control')
                    self.control_button.setEnabled(False)
                    self.stop_control_button.setEnabled(True)
        except Exception as e:
            self.log_message(f"Error taking control: {str(e)}")

    def stop_control(self):
        try:
            current_item = self.client_list.currentItem()
            if current_item:
                selected_text = current_item.text()
                client_id = self.get_client_id_from_text(selected_text)
                if client_id:
                    self.log_message(f"Stopping control of client: {selected_text}")
                    self.send_control_command('stop_control')
                    self.control_button.setEnabled(True)
                    self.stop_control_button.setEnabled(False)
        except Exception as e:
            self.log_message(f"Error stopping control: {str(e)}")

    def send_control_command(self, command):
        if self.selected_client:
            self.sio.emit('admin_command', {
                'client_id': self.selected_client,
                'command': command
            })

    def send_message(self):
        try:
            current_item = self.client_list.currentItem()
            if current_item:
                selected_text = current_item.text()
                client_id = self.get_client_id_from_text(selected_text)
                if client_id:
                    text, ok = QInputDialog.getText(
                        self, 
                        "Send Message", 
                        f"Enter message for {selected_text}:"
                    )
                    if ok and text:
                        self.log_message(f"Sending message to client: {selected_text}")
                        self.sio.emit('admin_command', {
                            'client_id': client_id,
                            'command': 'message',
                            'message': text
                        })
        except Exception as e:
            self.log_message(f"Error sending message: {str(e)}")

    def lock_selected_client(self):
        try:
            current_item = self.client_list.currentItem()
            if current_item:
                selected_text = current_item.text()
                client_id = self.get_client_id_from_text(selected_text)
                if client_id:
                    self.log_message(f"Locking client: {selected_text}")
                    self.sio.emit('admin_command', {
                        'client_id': client_id,
                        'command': 'lock'
                    })
                    self.lock_button.setEnabled(False)
                    self.unlock_button.setEnabled(True)
        except Exception as e:
            self.log_message(f"Error locking client: {str(e)}")

    def unlock_selected_client(self):
        try:
            current_item = self.client_list.currentItem()
            if current_item:
                selected_text = current_item.text()
                client_id = self.get_client_id_from_text(selected_text)
                if client_id:
                    self.log_message(f"Unlocking client: {selected_text}")
                    self.sio.emit('admin_command', {
                        'client_id': client_id,
                        'command': 'unlock'
                    })
                    self.lock_button.setEnabled(True)
                    self.unlock_button.setEnabled(False)
        except Exception as e:
            self.log_message(f"Error unlocking client: {str(e)}")

    def get_client_id_from_text(self, text):
        for client_id, info in self.clients.items():
            display_text = f"{info.get('hostname', 'Unknown')} ({info.get('ip', 'Unknown')})"
            if display_text == text:
                return client_id
        return None

    def enable_client_buttons(self, enable=True):
        self.view_button.setEnabled(enable)
        self.control_button.setEnabled(enable)
        self.message_button.setEnabled(enable)
        self.lock_button.setEnabled(enable)
        # Don't enable stop_control and unlock - they're enabled when their counterparts are used

    def check_connection(self):
        try:
            if self.sio.connected:
                self.log_message("Connection check: Connected")
            else:
                self.log_message("Connection check: Disconnected")
        except Exception as e:
            self.log_message(f"Connection check error: {str(e)}")

    def closeEvent(self, event):
        self.log_message("Closing admin panel")
        self.check_timer.stop()
        if self.sio.connected:
            self.sio.disconnect()
        super().closeEvent(event)

if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description='Remote Desktop Admin Panel')
    parser.add_argument('--server', default='192.168.1.90:8000',
                       help='Server address (e.g., http://example.com:8000)')
    args = parser.parse_args()

    app = QApplication(sys.argv)
    admin = AdminPanel()
    admin.show()
    sys.exit(app.exec())
