"""Exercise the native curl IMAPS path against a trusted, isolated TLS server."""
import base64
import datetime
import ipaddress
import json
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


def main():
    with tempfile.TemporaryDirectory() as temporary:
        root = Path(temporary)
        key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "KiroX isolated fixture")])
        now = datetime.datetime.now(datetime.timezone.utc)
        cert = (x509.CertificateBuilder().subject_name(name).issuer_name(name).public_key(key.public_key())
                .serial_number(x509.random_serial_number()).not_valid_before(now - datetime.timedelta(minutes=1))
                .not_valid_after(now + datetime.timedelta(days=1))
                .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
                .add_extension(x509.SubjectAlternativeName([x509.DNSName("localhost"), x509.IPAddress(ipaddress.ip_address("127.0.0.1"))]), critical=False)
                .sign(key, hashes.SHA256()))
        ca = root / "ca.pem"
        ca.write_bytes(cert.public_bytes(serialization.Encoding.PEM))
        private = root / "key.pem"
        private.write_bytes(key.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()))
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(ca, private)
        listener = socket.socket()
        listener.bind(("127.0.0.1", 0))
        listener.listen(8)
        listener.settimeout(0.2)
        finished = threading.Event()
        commands, auth, errors = [], [], []
        message = b"From: AWS <noreply@example.test>\r\nSubject: Verification\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nYour verification code is 123456.\r\n"

        def serve(raw):
            try:
                with context.wrap_socket(raw, server_side=True) as client:
                    client.settimeout(5)
                    stream = client.makefile("rb")
                    client.sendall(b"* OK [CAPABILITY IMAP4rev1 AUTH=XOAUTH2 SASL-IR] ready\r\n")
                    while not finished.is_set():
                        line = stream.readline()
                        if not line:
                            break
                        tag, command = line.strip().split(b" ", 1)
                        commands.append(command)
                        if command == b"CAPABILITY":
                            body = b"* CAPABILITY IMAP4rev1 AUTH=XOAUTH2 SASL-IR\r\n"
                        elif command.startswith(b"AUTHENTICATE XOAUTH2"):
                            pieces = command.split()
                            if len(pieces) == 2:
                                client.sendall(b"+ \r\n")
                                token = stream.readline().strip()
                            else:
                                token = pieces[2]
                            auth.append(base64.b64decode(token))
                            body = b""
                        elif command.startswith((b"SELECT ", b"EXAMINE ")):
                            body = b"* 1 EXISTS\r\n* FLAGS (\\Seen)\r\n* OK [UIDVALIDITY 8] valid\r\n* OK [UIDNEXT 42] next\r\n"
                        elif command.startswith(b"STATUS "):
                            body = b"* STATUS INBOX (UIDNEXT 42 UIDVALIDITY 8)\r\n"
                        elif command.startswith(b"UID SEARCH"):
                            body = b"* SEARCH 42\r\n"
                        elif command.startswith(b"UID FETCH"):
                            body = b"* 1 FETCH (UID 42 BODY[] {" + str(len(message)).encode() + b"}\r\n" + message + b")\r\n"
                        elif command == b"NOOP":
                            finished.wait(2)
                            continue
                        elif command == b"LOGOUT":
                            client.sendall(b"* BYE goodbye\r\n" + tag + b" OK logout\r\n")
                            break
                        else:
                            raise AssertionError(f"Unexpected IMAP command: {command!r}")
                        client.sendall(body + tag + b" OK complete\r\n")
            except (OSError, ssl.SSLError):
                pass  # Client cancellation intentionally closes a TLS session.
            except Exception as error:
                errors.append(repr(error))

        def accept():
            while not finished.is_set():
                try:
                    client, _ = listener.accept()
                except socket.timeout:
                    continue
                except OSError:
                    break
                threading.Thread(target=serve, args=(client,), daemon=True).start()

        thread = threading.Thread(target=accept, daemon=True)
        thread.start()
        try:
            completed = subprocess.run([sys.argv[1], f"imaps://127.0.0.1:{listener.getsockname()[1]}", str(ca)], capture_output=True, timeout=15)
            assert completed.returncode == 0, completed.stderr.decode(errors="replace")
            result = json.loads(completed.stdout)
            assert "UIDNEXT 42" in result["status"], result
            assert "SEARCH 42" in result["search"], result
            assert result["message"].encode() == message, result
            assert result["cancelled"] and result["timedOut"] and result["cancellationMs"] < 1500, result
            assert auth and all(value == b"user=fixture@example.test\x01auth=Bearer synthetic-token\x01\x01" for value in auth), auth
            assert any(b"Junk" in value for value in commands), commands
            assert not errors, errors
            print("Native IMAPS: certificate verification, XOAUTH2, folders, UID search/fetch, cancellation and timeout passed.")
        finally:
            finished.set()
            listener.close()
            thread.join(timeout=2)


if __name__ == "__main__":
    main()
