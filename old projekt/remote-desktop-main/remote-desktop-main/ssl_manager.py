import os
import ipaddress
from datetime import datetime, timedelta
from cryptography import x509
from cryptography.x509.oid import NameOID, ExtendedKeyUsageOID
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives.asymmetric.rsa import RSAPrivateKey
import logging

logger = logging.getLogger(__name__)

class SSLCertificateManager:
    def __init__(self):
        self.cert_dir = "certs"
        self.ca_key_path = os.path.join(self.cert_dir, "ca.key")
        self.ca_cert_path = os.path.join(self.cert_dir, "ca.crt")
        self.server_key_path = os.path.join(self.cert_dir, "server.key")
        self.server_cert_path = os.path.join(self.cert_dir, "server.crt")
        
        # Create certs directory if it doesn't exist
        if not os.path.exists(self.cert_dir):
            os.makedirs(self.cert_dir)

    def create_key_pair(self, key_size: int = 2048) -> RSAPrivateKey:
        """Generate a new RSA key pair"""
        return rsa.generate_private_key(
            public_exponent=65537,
            key_size=key_size
        )

    def create_ca_certificate(self) -> None:
        """Create a Certificate Authority certificate"""
        # Generate key
        private_key = self.create_key_pair()
        
        # Save private key
        with open(self.ca_key_path, "wb") as f:
            f.write(private_key.private_bytes(
                encoding=serialization.Encoding.PEM,
                format=serialization.PrivateFormat.PKCS8,
                encryption_algorithm=serialization.NoEncryption()
            ))

        # Create certificate
        subject = issuer = x509.Name([
            x509.NameAttribute(NameOID.COMMON_NAME, "Remote Desktop CA"),
            x509.NameAttribute(NameOID.ORGANIZATION_NAME, "Remote Desktop"),
            x509.NameAttribute(NameOID.COUNTRY_NAME, "US")
        ])

        # Generate serial number
        serial_number = x509.random_serial_number()

        # Create certificate builder
        builder = x509.CertificateBuilder()
        builder = builder.subject_name(subject)
        builder = builder.issuer_name(issuer)
        builder = builder.not_valid_before(datetime.now() - timedelta(days=1))
        builder = builder.not_valid_after(datetime.now() + timedelta(days=365*10))
        builder = builder.serial_number(serial_number)
        builder = builder.public_key(private_key.public_key())

        # Add extensions
        builder = builder.add_extension(
            x509.BasicConstraints(ca=True, path_length=None),
            critical=True
        )
        
        # Add Subject Key Identifier
        builder = builder.add_extension(
            x509.SubjectKeyIdentifier.from_public_key(private_key.public_key()),
            critical=False
        )

        # Add Key Usage
        builder = builder.add_extension(
            x509.KeyUsage(
                digital_signature=True,
                key_encipherment=True,
                key_cert_sign=True,
                crl_sign=True,
                content_commitment=False,
                data_encipherment=False,
                key_agreement=False,
                encipher_only=False,
                decipher_only=False
            ),
            critical=True
        )

        # Sign the certificate
        certificate = builder.sign(
            private_key=private_key,
            algorithm=hashes.SHA256()
        )

        # Save certificate
        with open(self.ca_cert_path, "wb") as f:
            f.write(certificate.public_bytes(serialization.Encoding.PEM))

    def create_server_certificate(self, local_ip: str) -> None:
        """Create a server certificate signed by our CA"""
        # Check if CA exists, if not create it
        if not os.path.exists(self.ca_cert_path) or not os.path.exists(self.ca_key_path):
            self.create_ca_certificate()

        # Load CA key and certificate
        with open(self.ca_key_path, "rb") as f:
            ca_key = serialization.load_pem_private_key(f.read(), password=None)
        
        with open(self.ca_cert_path, "rb") as f:
            ca_cert = x509.load_pem_x509_certificate(f.read())

        # Generate server key
        server_key = self.create_key_pair()

        # Save server private key
        with open(self.server_key_path, "wb") as f:
            f.write(server_key.private_bytes(
                encoding=serialization.Encoding.PEM,
                format=serialization.PrivateFormat.PKCS8,
                encryption_algorithm=serialization.NoEncryption()
            ))

        # Create certificate
        subject = x509.Name([
            x509.NameAttribute(NameOID.COMMON_NAME, "Remote Desktop Server"),
            x509.NameAttribute(NameOID.ORGANIZATION_NAME, "Remote Desktop"),
            x509.NameAttribute(NameOID.COUNTRY_NAME, "US")
        ])

        # Create certificate builder
        builder = x509.CertificateBuilder()
        builder = builder.subject_name(subject)
        builder = builder.issuer_name(ca_cert.subject)
        builder = builder.not_valid_before(datetime.now() - timedelta(days=1))
        builder = builder.not_valid_after(datetime.now() + timedelta(days=365*10))
        builder = builder.serial_number(x509.random_serial_number())
        builder = builder.public_key(server_key.public_key())

        # Add extensions
        builder = builder.add_extension(
            x509.BasicConstraints(ca=False, path_length=None),
            critical=True
        )

        # Add Subject Key Identifier
        builder = builder.add_extension(
            x509.SubjectKeyIdentifier.from_public_key(server_key.public_key()),
            critical=False
        )

        # Add Authority Key Identifier
        builder = builder.add_extension(
            x509.AuthorityKeyIdentifier.from_issuer_public_key(ca_key.public_key()),
            critical=False
        )

        # Add Subject Alternative Names
        san = [
            x509.DNSName("localhost"),
            x509.DNSName("*"),  # Wildcard for all hostnames
            x509.IPAddress(ipaddress.IPv4Address("127.0.0.1")),
            x509.IPAddress(ipaddress.IPv4Address(local_ip))
        ]
        
        # Try to add public IP if available
        try:
            import socket
            import urllib.request
            
            # Get public IP
            public_ip = urllib.request.urlopen('https://api.ipify.org').read().decode('utf8')
            if public_ip:
                san.append(x509.IPAddress(ipaddress.IPv4Address(public_ip)))
                
            # Get all local IPs
            hostname = socket.gethostname()
            all_ips = socket.gethostbyname_ex(hostname)[2]
            for ip in all_ips:
                if ip != local_ip and ip != "127.0.0.1":
                    san.append(x509.IPAddress(ipaddress.IPv4Address(ip)))
                    
        except Exception as e:
            logger.warning(f"Failed to add additional IPs to certificate: {e}")
            
        builder = builder.add_extension(
            x509.SubjectAlternativeName(san),
            critical=False
        )

        # Add Extended Key Usage
        builder = builder.add_extension(
            x509.ExtendedKeyUsage([
                ExtendedKeyUsageOID.SERVER_AUTH,
                ExtendedKeyUsageOID.CLIENT_AUTH
            ]),
            critical=False
        )

        # Add Key Usage
        builder = builder.add_extension(
            x509.KeyUsage(
                digital_signature=True,
                key_encipherment=True,
                key_cert_sign=False,
                crl_sign=False,
                content_commitment=False,
                data_encipherment=False,
                key_agreement=False,
                encipher_only=False,
                decipher_only=False
            ),
            critical=True
        )

        # Sign the certificate with CA key
        certificate = builder.sign(
            private_key=ca_key,
            algorithm=hashes.SHA256()
        )

        # Save certificate with full chain
        with open(self.server_cert_path, "wb") as f:
            # Write server certificate
            f.write(certificate.public_bytes(serialization.Encoding.PEM))
            # Write CA certificate to complete the chain
            f.write(ca_cert.public_bytes(serialization.Encoding.PEM))

    def get_ca_cert_path(self) -> str:
        """Get the path to the CA certificate"""
        return self.ca_cert_path

    def get_ca_key_path(self) -> str:
        """Get the path to the CA private key"""
        return self.ca_key_path

    def get_server_cert_path(self) -> str:
        """Get the path to the server certificate"""
        return self.server_cert_path

    def get_server_key_path(self) -> str:
        """Get the path to the server private key"""
        return self.server_key_path
