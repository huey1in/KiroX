#include <windows.h>
#include "kirox/domain/error.hpp"
#include "kirox/infrastructure/native_crypto.hpp"
#include <QCryptographicHash>
#include <bcrypt.h>
#include <cstring>
#include <limits>

namespace kirox {
namespace {
void require(NTSTATUS status) {
    if (!BCRYPT_SUCCESS(status))
        throw Error(ErrorCode::Protocol, "Native cryptographic operation failed (" +
                                             QString::number(static_cast<quint32>(status), 16) + ")");
}
struct Algorithm {
    BCRYPT_ALG_HANDLE handle = nullptr;
    explicit Algorithm(LPCWSTR name) {
        require(BCryptOpenAlgorithmProvider(&handle, name, nullptr, 0));
    }
    ~Algorithm() {
        BCryptCloseAlgorithmProvider(handle, 0);
    }
    Algorithm(const Algorithm &) = delete;
};
struct Key {
    BCRYPT_KEY_HANDLE handle = nullptr;
    ~Key() {
        if (handle)
            BCryptDestroyKey(handle);
    }
};
PUCHAR bytes(const QByteArray &data) {
    return reinterpret_cast<PUCHAR>(const_cast<char *>(data.constData()));
}
ULONG length(const QByteArray &data) {
    if (data.size() > std::numeric_limits<ULONG>::max())
        throw Error(ErrorCode::InvalidInput, "Cryptographic input too large");
    return static_cast<ULONG>(data.size());
}
QByteArray decode(const QJsonObject &publicKey, const char *name) {
    const auto result = QByteArray::fromBase64Encoding(
        publicKey[name].toString().toLatin1(), QByteArray::Base64UrlEncoding | QByteArray::AbortOnBase64DecodingErrors);
    if (!result || result.decoded.isEmpty())
        throw Error(ErrorCode::InvalidInput, "Invalid RSA JWK");
    return result.decoded;
}
} // namespace
QByteArray NativeCryptography::randomBytes(int size) const {
    if (size <= 0 || size > 1024 * 1024)
        throw Error(ErrorCode::InvalidInput, "Invalid random byte count");
    QByteArray result(size, '\0');
    require(BCryptGenRandom(nullptr, bytes(result), length(result), BCRYPT_USE_SYSTEM_PREFERRED_RNG));
    return result;
}
EphemeralSignature NativeCryptography::signEs256(const QByteArray &message) const {
    Algorithm algorithm(BCRYPT_ECDSA_P256_ALGORITHM);
    Key key;
    require(BCryptGenerateKeyPair(algorithm.handle, &key.handle, 256, 0));
    require(BCryptFinalizeKeyPair(key.handle, 0));
    const auto digest = QCryptographicHash::hash(message, QCryptographicHash::Sha256);
    ULONG size = 0;
    require(BCryptSignHash(key.handle, nullptr, bytes(digest), length(digest), nullptr, 0, &size, 0));
    EphemeralSignature result{QByteArray(size, '\0'), {}};
    require(BCryptSignHash(key.handle, nullptr, bytes(digest), length(digest), bytes(result.signature),
                           length(result.signature), &size, 0));
    result.signature.resize(size);
    if (result.signature.size() != 64)
        throw Error(ErrorCode::Protocol, "Invalid ES256 signature length");
    require(BCryptExportKey(key.handle, nullptr, BCRYPT_ECCPUBLIC_BLOB, nullptr, 0, &size, 0));
    QByteArray blob(size, '\0');
    require(BCryptExportKey(key.handle, nullptr, BCRYPT_ECCPUBLIC_BLOB, bytes(blob), length(blob), &size, 0));
    if (size != sizeof(BCRYPT_ECCKEY_BLOB) + 64)
        throw Error(ErrorCode::Protocol, "Invalid EC public key");
    const auto encode = [&](int offset) {
        return QString::fromLatin1(blob.mid(sizeof(BCRYPT_ECCKEY_BLOB) + offset, 32)
                                       .toBase64(QByteArray::Base64UrlEncoding | QByteArray::OmitTrailingEquals));
    };
    result.publicKey = {{"kty", "EC"}, {"crv", "P-256"}, {"x", encode(0)}, {"y", encode(32)}};
    return result;
}
QByteArray NativeCryptography::rsaOaep256(const QJsonObject &publicKey, const QByteArray &plaintext) const {
    auto modulus = decode(publicKey, "n");
    auto exponent = decode(publicKey, "e");
    while (modulus.size() > 1 && modulus[0] == 0)
        modulus.remove(0, 1);
    if (modulus.size() < 256 || modulus.size() > 1024 || exponent.size() > 8)
        throw Error(ErrorCode::InvalidInput, "Unsupported RSA key size");
    const BCRYPT_RSAKEY_BLOB header{
        BCRYPT_RSAPUBLIC_MAGIC, static_cast<ULONG>(modulus.size() * 8), length(exponent), length(modulus), 0, 0};
    QByteArray blob(reinterpret_cast<const char *>(&header), sizeof(header));
    blob += exponent;
    blob += modulus;
    Algorithm algorithm(BCRYPT_RSA_ALGORITHM);
    Key key;
    require(BCryptImportKeyPair(algorithm.handle, nullptr, BCRYPT_RSAPUBLIC_BLOB, &key.handle, bytes(blob),
                                length(blob), 0));
    BCRYPT_OAEP_PADDING_INFO padding{BCRYPT_SHA256_ALGORITHM, nullptr, 0};
    ULONG required = 0;
    require(BCryptEncrypt(key.handle, bytes(plaintext), length(plaintext), &padding, nullptr, 0, nullptr, 0, &required,
                          BCRYPT_PAD_OAEP));
    QByteArray result(required, '\0');
    require(BCryptEncrypt(key.handle, bytes(plaintext), length(plaintext), &padding, nullptr, 0, bytes(result),
                          length(result), &required, BCRYPT_PAD_OAEP));
    result.resize(required);
    return result;
}
SealedMessage NativeCryptography::aes256Gcm(const QByteArray &keyBytes, const QByteArray &nonce,
                                            const QByteArray &plaintext, const QByteArray &additionalData) const {
    if (keyBytes.size() != 32 || nonce.size() != 12)
        throw Error(ErrorCode::InvalidInput, "Invalid AES-256-GCM key or nonce");
    Algorithm algorithm(BCRYPT_AES_ALGORITHM);
    Key key;
    require(BCryptSetProperty(algorithm.handle, BCRYPT_CHAINING_MODE,
                              reinterpret_cast<PUCHAR>(const_cast<wchar_t *>(BCRYPT_CHAIN_MODE_GCM)),
                              sizeof(BCRYPT_CHAIN_MODE_GCM), 0));
    require(
        BCryptGenerateSymmetricKey(algorithm.handle, &key.handle, nullptr, 0, bytes(keyBytes), length(keyBytes), 0));
    SealedMessage result{QByteArray(plaintext.size(), '\0'), QByteArray(16, '\0')};
    BCRYPT_AUTHENTICATED_CIPHER_MODE_INFO info;
    BCRYPT_INIT_AUTH_MODE_INFO(info);
    info.pbNonce = bytes(nonce);
    info.cbNonce = length(nonce);
    info.pbAuthData = bytes(additionalData);
    info.cbAuthData = length(additionalData);
    info.pbTag = bytes(result.tag);
    info.cbTag = length(result.tag);
    ULONG written = 0;
    require(BCryptEncrypt(key.handle, bytes(plaintext), length(plaintext), &info, nullptr, 0, bytes(result.ciphertext),
                          length(result.ciphertext), &written, 0));
    result.ciphertext.resize(written);
    return result;
}
} // namespace kirox
