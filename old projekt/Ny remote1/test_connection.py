import requests
import socketio

def test_http():
    print("Testing HTTP connection...")
    try:
        response = requests.get('http://141.148.244.140:8000', timeout=30)
        print(f"HTTP Response: {response.status_code}")
        print(f"Content: {response.text}")
    except Exception as e:
        print(f"HTTP Error: {e}")

def test_socketio():
    print("\nTesting Socket.IO connection...")
    sio = socketio.Client(request_timeout=30)
    
    @sio.event
    def connect():
        print("Socket.IO Connected!")
        sio.disconnect()

    @sio.event
    def connect_error(data):
        print(f"Socket.IO Connection failed: {data}")

    try:
        sio.connect('http://141.148.244.140:8000', wait_timeout=30)
    except Exception as e:
        print(f"Socket.IO Error: {e}")

if __name__ == "__main__":
    test_http()
    test_socketio()
