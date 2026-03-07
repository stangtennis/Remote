import os
import sys
import logging
import subprocess
from pathlib import Path

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def install_ca_certificate():
    try:
        # Get the absolute path to the CA certificate
        cert_path = Path("certs/ca.crt").absolute()
        
        if not cert_path.exists():
            logger.error(f"Certificate not found at: {cert_path}")
            return False
            
        logger.info(f"Installing certificate from: {cert_path}")
        
        # Use certutil to install the certificate
        result = subprocess.run(
            ["certutil", "-addstore", "-user", "ROOT", str(cert_path)],
            capture_output=True,
            text=True
        )
        
        if result.returncode == 0:
            logger.info("Certificate installed successfully!")
            logger.info("You can now access the remote desktop without SSL warnings")
            return True
        else:
            logger.error(f"Failed to install certificate: {result.stderr}")
            return False
            
    except Exception as e:
        logger.error(f"Error installing certificate: {str(e)}")
        return False

if __name__ == "__main__":
    # Check if running with admin privileges
    try:
        is_admin = os.getuid() == 0
    except AttributeError:
        import ctypes
        is_admin = ctypes.windll.shell32.IsUserAnAdmin() != 0
        
    if not is_admin:
        logger.error("This script must be run with administrator privileges!")
        logger.info("Please right-click and select 'Run as administrator'")
        sys.exit(1)
        
    # Install the certificate
    success = install_ca_certificate()
    
    # Keep window open if there was an error
    if not success:
        input("Press Enter to exit...")
