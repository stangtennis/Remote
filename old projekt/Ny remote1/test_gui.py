from PyQt6.QtWidgets import QApplication, QMainWindow, QLabel
import sys

app = QApplication(sys.argv)
window = QMainWindow()
window.setWindowTitle("PyQt6 Test")
window.setGeometry(100, 100, 400, 200)

label = QLabel("If you can see this, PyQt6 is working!", window)
label.setGeometry(50, 80, 300, 30)

window.show()
sys.exit(app.exec())
