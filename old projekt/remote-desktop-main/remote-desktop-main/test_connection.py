import sys
import logging
import requests
import socket
import ssl
import websocket
from urllib3.exceptions import InsecureRequestWarning
from pathlib import Path

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def test_https_connection(url, verify=False):
    """Test HTTPS connection to the server"""
    try:
        # Disable warnings for self-signed certificates if not verifying
        if not verify:
            requests.packages.urllib3.disable_warnings(InsecureRequestWarning)
        
        # Make request to server with verify=False during development
        response = requests.get(url, verify=False)
        logger.info(f"HTTPS Connection: SUCCESS")
        logger.info(f"Status Code: {response.status_code}")
        logger.info(f"Server Headers: {dict(response.headers)}")
        return True
    except requests.exceptions.SSLError as e:
        logger.error(f"SSL Error: {str(e)}")
        return False
    except Exception as e:
        logger.error(f"Connection Error: {str(e)}")
        return False

def test_websocket_connection(ws_url, verify=False):
    """Test WebSocket connection to the server"""
    try:
        # Create WebSocket connection with SSL verification disabled
        ws = websocket.create_connection(
            ws_url,
            sslopt={
                "cert_reqs": ssl.CERT_NONE,
                "check_hostname": False
            }
        )
        
        # Receive the initial Engine.IO packet
        result = ws.recv()
        ws.close()
        
        logger.info("WebSocket Connection: SUCCESS")
        logger.info(f"WebSocket Response: {result}")
        return True
    except Exception as e:
        logger.error(f"WebSocket Error: {str(e)}")
        return False

def test_certificate(hostname, port):
    """Test SSL certificate details"""
    try:
        # Create a context that doesn't verify certificates
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        
        with socket.create_connection((hostname, port)) as sock:
            with context.wrap_socket(sock, server_hostname=hostname) as ssock:
                cert = ssock.getpeercert(binary_form=True)
                logger.info("Certificate Connection: SUCCESS")
                logger.info("Note: Certificate details not available in binary form")
                return True
    except Exception as e:
        logger.error(f"Certificate Error: {str(e)}")
        return False

def main():
    # Test parameters
    hostname = "localhost"
    port = 7080
    use_ssl_verification = False  # Disable SSL verification during development
    
    # URLs for testing
    https_url = f"https://{hostname}:{port}"
    ws_url = f"wss://{hostname}:{port}/socket.io/?EIO=4&transport=websocket"
    
    logger.info("=== Starting Connection Tests ===")
    
    # Test HTTPS connection
    logger.info("\n1. Testing HTTPS Connection...")
    https_success = test_https_connection(https_url, verify=use_ssl_verification)
    
    # Test WebSocket connection
    logger.info("\n2. Testing WebSocket Connection...")
    ws_success = test_websocket_connection(ws_url, verify=use_ssl_verification)
    
    # Test certificate
    logger.info("\n3. Testing SSL Certificate...")
    cert_success = test_certificate(hostname, port)
    
    # Print summary
    logger.info("\n=== Test Summary ===")
    logger.info(f"HTTPS Connection: {'✓' if https_success else '✗'}")
    logger.info(f"WebSocket Connection: {'✓' if ws_success else '✗'}")
    logger.info(f"Certificate Connection: {'✓' if cert_success else '✗'}")
    
    # Return success status
    return all(x for x in [https_success, ws_success, cert_success] if x is not None)

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
