package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

// JWEEncryptor JWE 加密器
type JWEEncryptor struct{}

// b64url Base64 URL 编码 (无填充)
func b64url(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// jwkToPublicKey 将 JWK 转为 RSA 公钥
func jwkToPublicKey(jwk map[string]string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk["n"])
	if err != nil {
		return nil, fmt.Errorf("解码 n 失败: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk["e"])
	if err != nil {
		return nil, fmt.Errorf("解码 e 失败: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

// Encrypt 加密密码
func (j *JWEEncryptor) Encrypt(
	password string,
	publicKey map[string]string,
	issuer, audience, region string,
) (string, error) {
	// Header
	header := map[string]string{
		"alg": "RSA-OAEP-256",
		"kid": publicKey["kid"],
		"enc": "A256GCM",
		"cty": "enc",
		"typ": "application/aws+signin+jwe",
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := b64url(headerJSON)

	// CEK (内容加密密钥)
	cek := make([]byte, 32)
	if _, err := rand.Read(cek); err != nil {
		return "", err
	}

	// RSA-OAEP-256 加密 CEK
	pubKey, err := jwkToPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	encryptedCEK, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, cek, nil)
	if err != nil {
		return "", err
	}

	// Claims
	now := time.Now().Unix()
	claims := map[string]interface{}{
		"iss":      fmt.Sprintf("%s.%s", region, issuer),
		"iat":      now,
		"nbf":      now,
		"jti":      genUUID(),
		"exp":      now + 300,
		"aud":      fmt.Sprintf("%s.%s", region, audience),
		"password": password,
	}
	plaintext, _ := json.Marshal(claims)

	// AES-256-GCM 加密
	iv := make([]byte, 12)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	// AAD = headerB64
	ciphertext := gcm.Seal(nil, iv, plaintext, []byte(headerB64))

	// 分离密文和 tag (最后 16 字节)
	ct := ciphertext[:len(ciphertext)-16]
	tag := ciphertext[len(ciphertext)-16:]

	// JWE Compact: header.encKey.iv.ciphertext.tag
	return fmt.Sprintf("%s.%s.%s.%s.%s",
		headerB64,
		b64url(encryptedCEK),
		b64url(iv),
		b64url(ct),
		b64url(tag),
	), nil
}

// genUUID mirrors the UUID v4 value used by the AWS browser encryptor for jti.
func genUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
