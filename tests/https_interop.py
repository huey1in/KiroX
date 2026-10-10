"""Verify nested HTTPS CONNECT, authentication and both TLS trust boundaries."""
import base64
import datetime
import ipaddress
from pathlib import Path
import socket
import ssl
import subprocess
import sys
import tempfile
import threading

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID


def certificate(root, label):
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, label)])
    now = datetime.datetime.now(datetime.timezone.utc)
    cert = (x509.CertificateBuilder().subject_name(name).issuer_name(name).public_key(key.public_key())
            .serial_number(x509.random_serial_number()).not_valid_before(now - datetime.timedelta(minutes=1))
            .not_valid_after(now + datetime.timedelta(days=1))
            .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
            .add_extension(x509.SubjectAlternativeName([x509.DNSName("localhost"), x509.IPAddress(ipaddress.ip_address("127.0.0.1"))]), critical=False)
            .sign(key, hashes.SHA256()))
    ca, private = root / f"{label}.pem", root / f"{label}.key"
    ca.write_bytes(cert.public_bytes(serialization.Encoding.PEM))
    private.write_bytes(key.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()))
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(ca, private)
    context.set_alpn_protocols(["http/1.1"])
    return ca, context


def headers(client):
    result = b""
    while b"\r\n\r\n" not in result:
        chunk = client.recv(4096)
        if not chunk:
            raise ConnectionError("Client disconnected")
        result += chunk
        if len(result) > 65536:
            raise AssertionError("Oversized fixture headers")
    return result


def main():
    with tempfile.TemporaryDirectory() as temporary:
        root = Path(temporary)
        target_ca, target_tls = certificate(root, "target")
        proxy_ca, proxy_tls = certificate(root, "proxy")
        stopped = threading.Event()
        connections, requests, errors = [], [], []

        def start(context, handler):
            listener = socket.socket()
            listener.bind(("127.0.0.1", 0))
            listener.listen(8)
            listener.settimeout(0.2)
            connections.append(listener)

            def serve(raw):
                try:
                    with context.wrap_socket(raw, server_side=True) as client:
                        client.settimeout(5)
                        handler(client)
                except (OSError, ssl.SSLError):
                    pass  # Negative trust tests intentionally abort the handshake.
                except Exception as error:
                    errors.append(repr(error))
                finally:
                    raw.close()

            def accept():
                while not stopped.is_set():
                    try:
                        client, _ = listener.accept()
                    except socket.timeout:
                        continue
                    except OSError:
                        break
                    threading.Thread(target=serve, args=(client,), daemon=True).start()

            threading.Thread(target=accept, daemon=True).start()
            return listener.getsockname()[1]

        def target(client):
            request = headers(client)
            assert request.startswith(b"GET /fixture HTTP/1.1\r\n"), request
            requests.append(request)
            client.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 8\r\nConnection: close\r\n\r\nverified")

        target_port = start(target_tls, target)

        def proxy(client):
            request = headers(client)
            assert request.startswith(f"CONNECT 127.0.0.1:{target_port} HTTP/1.1\r\n".encode()), request
            auth = b"Proxy-Authorization: Basic " + base64.b64encode(b"fixture:synthetic")
            assert auth in request, request
            client.sendall(b"HTTP/1.1 200 Connection established\r\n\r\n")
            with socket.create_connection(("127.0.0.1", target_port), timeout=5) as upstream:
                client.settimeout(0.03)
                upstream.settimeout(0.03)
                while not stopped.is_set():
                    for source, destination in [(client, upstream), (upstream, client)]:
                        try:
                            payload = source.recv(65536)
                        except socket.timeout:
                            continue
                        if not payload:
                            return
                        destination.sendall(payload)

        proxy_port = start(proxy_tls, proxy)
        url = f"https://127.0.0.1:{target_port}/fixture"
        proxy_url = f"https://fixture:synthetic@127.0.0.1:{proxy_port}"
        try:
            for profile in ["chrome131", "chrome133", "chrome144"]:
                result = subprocess.run([sys.argv[1], url, proxy_url, str(target_ca), str(proxy_ca), profile], capture_output=True, timeout=10)
                assert result.returncode == 0, result.stderr.decode(errors="replace")
            # A wrong trust root must reject the target and proxy independently.
            for target_trust, proxy_trust in [(proxy_ca, proxy_ca), (target_ca, target_ca)]:
                result = subprocess.run([sys.argv[1], url, proxy_url, str(target_trust), str(proxy_trust), "chrome131"], capture_output=True, timeout=10)
                assert result.returncode == 4, (result.returncode, result.stderr)
            assert len(requests) == 3 and not errors, (len(requests), errors)
            print("Native HTTPS: authenticated CONNECT, three browser profiles and independent target/proxy certificate rejection passed.")
        finally:
            stopped.set()
            for listener in connections:
                listener.close()


if __name__ == "__main__":
    main()
