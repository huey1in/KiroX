"""Verify native JWE using an independent RSA/AES implementation (test only)."""
import base64
import json
import subprocess
import sys
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, padding, rsa, utils
from cryptography.hazmat.primitives.ciphers.aead import AESGCM


def encode_integer(value):
    return base64.urlsafe_b64encode(value.to_bytes((value.bit_length() + 7) // 8, "big")).decode().rstrip("=")


def decode(value):
    return base64.urlsafe_b64decode(value + "=" * (-len(value) % 4))


key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
public = key.public_key().public_numbers()
password = "KiroX-密码-Password!7"
request = {"publicKey": {"n": encode_integer(public.n), "e": encode_integer(public.e), "kid": "interop"}, "password": password}
result = subprocess.run([sys.argv[1]], input=json.dumps(request).encode(), capture_output=True, timeout=10, check=True)
output = json.loads(result.stdout)
jwe = output["jwe"]
header, encrypted_key, nonce, ciphertext, tag = jwe.split(".")
cek = key.decrypt(decode(encrypted_key), padding.OAEP(mgf=padding.MGF1(hashes.SHA256()), algorithm=hashes.SHA256(), label=None))
claims = json.loads(AESGCM(cek).decrypt(decode(nonce), decode(ciphertext) + decode(tag), header.encode()))
assert json.loads(decode(header)) == {"alg": "RSA-OAEP-256", "kid": "interop", "enc": "A256GCM", "cty": "enc", "typ": "application/aws+signin+jwe"}
assert claims["password"] == password
assert claims["iss"] == "us-east-1.issuer" and claims["aud"] == "us-east-1.audience"
assert claims["iat"] == claims["nbf"] == 1700000000 and claims["exp"] == 1700000300
import uuid
assert uuid.UUID(claims["jti"]).version == 4
print("Native RSA-OAEP-256 / AES-256-GCM JWE decrypted and validated independently.")
signing_key = output["signingKey"]
public_key = ec.EllipticCurvePublicNumbers(int.from_bytes(decode(signing_key["x"]), "big"), int.from_bytes(decode(signing_key["y"]), "big"), ec.SECP256R1()).public_key()
signature = base64.b64decode(output["signature"])
assert len(signature) == 64
der = utils.encode_dss_signature(int.from_bytes(signature[:32], "big"), int.from_bytes(signature[32:], "big"))
public_key.verify(der, b"independent-es256-verification", ec.ECDSA(hashes.SHA256()))
print("Native P-256 ES256 signature independently verified.")
