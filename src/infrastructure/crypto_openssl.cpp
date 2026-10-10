#include "kirox/domain/error.hpp"
#include "kirox/infrastructure/native_crypto.hpp"
#include <climits>
#include <memory>
#include <openssl/core_names.h>
#include <openssl/ec.h>
#include <openssl/ecdsa.h>
#include <openssl/evp.h>
#include <openssl/param_build.h>
#include <openssl/rand.h>
#include <openssl/rsa.h>

namespace kirox {
namespace {
void require(bool okay) {
    if (!okay)
        throw Error(ErrorCode::Protocol, "Native cryptographic operation failed");
}
template <class T, auto Free> using Owner = std::unique_ptr<T, decltype(Free)>;
QByteArray decode(const QJsonObject &key, const char *name) {
    const auto value = QByteArray::fromBase64Encoding(
        key[name].toString().toLatin1(), QByteArray::Base64UrlEncoding | QByteArray::AbortOnBase64DecodingErrors);
    if (!value || value.decoded.isEmpty())
        throw Error(ErrorCode::InvalidInput, "Invalid RSA JWK");
    return value.decoded;
}
const unsigned char *data(const QByteArray &bytes) {
    return reinterpret_cast<const unsigned char *>(bytes.constData());
}
unsigned char *data(QByteArray &bytes) {
    return reinterpret_cast<unsigned char *>(bytes.data());
}
} // namespace
QByteArray NativeCryptography::randomBytes(int size) const {
    if (size <= 0 || size > 1024 * 1024)
        throw Error(ErrorCode::InvalidInput, "Invalid random byte count");
    QByteArray result(size, '\0');
    require(RAND_bytes(data(result), size) == 1);
    return result;
}
EphemeralSignature NativeCryptography::signEs256(const QByteArray &message) const {
    Owner<EVP_PKEY_CTX, EVP_PKEY_CTX_free> generator(EVP_PKEY_CTX_new_from_name(nullptr, "EC", nullptr),
                                                     EVP_PKEY_CTX_free);
    require(generator && EVP_PKEY_keygen_init(generator.get()) == 1);
    require(EVP_PKEY_CTX_set_group_name(generator.get(), "prime256v1") == 1);
    EVP_PKEY *rawKey = nullptr;
    require(EVP_PKEY_generate(generator.get(), &rawKey) == 1);
    Owner<EVP_PKEY, EVP_PKEY_free> key(rawKey, EVP_PKEY_free);
    Owner<EVP_MD_CTX, EVP_MD_CTX_free> signer(EVP_MD_CTX_new(), EVP_MD_CTX_free);
    require(signer && EVP_DigestSignInit(signer.get(), nullptr, EVP_sha256(), nullptr, key.get()) == 1);
    size_t size = 0;
    require(EVP_DigestSign(signer.get(), nullptr, &size, data(message), message.size()) == 1);
    QByteArray der(static_cast<qsizetype>(size), '\0');
    require(EVP_DigestSign(signer.get(), data(der), &size, data(message), message.size()) == 1);
    const auto *cursor = data(der);
    Owner<ECDSA_SIG, ECDSA_SIG_free> signature(d2i_ECDSA_SIG(nullptr, &cursor, static_cast<long>(size)),
                                               ECDSA_SIG_free);
    require(static_cast<bool>(signature));
    const BIGNUM *r = nullptr, *s = nullptr;
    ECDSA_SIG_get0(signature.get(), &r, &s);
    EphemeralSignature result{QByteArray(64, '\0'), {}};
    require(BN_bn2binpad(r, data(result.signature), 32) == 32 &&
            BN_bn2binpad(s, data(result.signature) + 32, 32) == 32);
    const auto coordinate = [&](const char *name) {
        BIGNUM *raw = nullptr;
        require(EVP_PKEY_get_bn_param(key.get(), name, &raw) == 1);
        Owner<BIGNUM, BN_free> value(raw, BN_free);
        QByteArray bytes(32, '\0');
        require(BN_bn2binpad(value.get(), data(bytes), 32) == 32);
        return QString::fromLatin1(bytes.toBase64(QByteArray::Base64UrlEncoding | QByteArray::OmitTrailingEquals));
    };
    result.publicKey = {{"kty", "EC"},
                        {"crv", "P-256"},
                        {"x", coordinate(OSSL_PKEY_PARAM_EC_PUB_X)},
                        {"y", coordinate(OSSL_PKEY_PARAM_EC_PUB_Y)}};
    return result;
}
QByteArray NativeCryptography::rsaOaep256(const QJsonObject &publicKey, const QByteArray &plaintext) const {
    const auto modulus = decode(publicKey, "n"), exponent = decode(publicKey, "e");
    if (modulus.size() < 256 || modulus.size() > 1024 || exponent.size() > 8)
        throw Error(ErrorCode::InvalidInput, "Unsupported RSA key size");
    Owner<BIGNUM, BN_free> n(BN_bin2bn(data(modulus), static_cast<int>(modulus.size()), nullptr), BN_free);
    Owner<BIGNUM, BN_free> e(BN_bin2bn(data(exponent), static_cast<int>(exponent.size()), nullptr), BN_free);
    Owner<OSSL_PARAM_BLD, OSSL_PARAM_BLD_free> builder(OSSL_PARAM_BLD_new(), OSSL_PARAM_BLD_free);
    require(n && e && builder);
    require(OSSL_PARAM_BLD_push_BN(builder.get(), OSSL_PKEY_PARAM_RSA_N, n.get()) == 1);
    require(OSSL_PARAM_BLD_push_BN(builder.get(), OSSL_PKEY_PARAM_RSA_E, e.get()) == 1);
    Owner<OSSL_PARAM, OSSL_PARAM_free> parameters(OSSL_PARAM_BLD_to_param(builder.get()), OSSL_PARAM_free);
    Owner<EVP_PKEY_CTX, EVP_PKEY_CTX_free> importer(EVP_PKEY_CTX_new_from_name(nullptr, "RSA", nullptr),
                                                    EVP_PKEY_CTX_free);
    require(parameters && importer);
    require(EVP_PKEY_fromdata_init(importer.get()) == 1);
    EVP_PKEY *rawKey = nullptr;
    require(EVP_PKEY_fromdata(importer.get(), &rawKey, EVP_PKEY_PUBLIC_KEY, parameters.get()) == 1);
    Owner<EVP_PKEY, EVP_PKEY_free> key(rawKey, EVP_PKEY_free);
    Owner<EVP_PKEY_CTX, EVP_PKEY_CTX_free> context(EVP_PKEY_CTX_new(key.get(), nullptr), EVP_PKEY_CTX_free);
    require(context && EVP_PKEY_encrypt_init(context.get()) == 1);
    require(EVP_PKEY_CTX_set_rsa_padding(context.get(), RSA_PKCS1_OAEP_PADDING) == 1);
    require(EVP_PKEY_CTX_set_rsa_oaep_md(context.get(), EVP_sha256()) == 1);
    require(EVP_PKEY_CTX_set_rsa_mgf1_md(context.get(), EVP_sha256()) == 1);
    size_t size = 0;
    require(EVP_PKEY_encrypt(context.get(), nullptr, &size, data(plaintext), plaintext.size()) == 1);
    QByteArray result(static_cast<qsizetype>(size), '\0');
    require(EVP_PKEY_encrypt(context.get(), data(result), &size, data(plaintext), plaintext.size()) == 1);
    result.resize(static_cast<qsizetype>(size));
    return result;
}
SealedMessage NativeCryptography::aes256Gcm(const QByteArray &key, const QByteArray &nonce, const QByteArray &plaintext,
                                            const QByteArray &additionalData) const {
    if (key.size() != 32 || nonce.size() != 12 || plaintext.size() > INT_MAX || additionalData.size() > INT_MAX)
        throw Error(ErrorCode::InvalidInput, "Invalid AES-256-GCM input");
    Owner<EVP_CIPHER_CTX, EVP_CIPHER_CTX_free> context(EVP_CIPHER_CTX_new(), EVP_CIPHER_CTX_free);
    require(static_cast<bool>(context));
    require(EVP_EncryptInit_ex(context.get(), EVP_aes_256_gcm(), nullptr, data(key), data(nonce)) == 1);
    int size = 0;
    require(EVP_EncryptUpdate(context.get(), nullptr, &size, data(additionalData),
                              static_cast<int>(additionalData.size())) == 1);
    SealedMessage result{QByteArray(plaintext.size() + 16, '\0'), QByteArray(16, '\0')};
    require(EVP_EncryptUpdate(context.get(), data(result.ciphertext), &size, data(plaintext),
                              static_cast<int>(plaintext.size())) == 1);
    int finalSize = 0;
    require(EVP_EncryptFinal_ex(context.get(), data(result.ciphertext) + size, &finalSize) == 1);
    result.ciphertext.resize(size + finalSize);
    require(EVP_CIPHER_CTX_ctrl(context.get(), EVP_CTRL_GCM_GET_TAG, 16, data(result.tag)) == 1);
    return result;
}
} // namespace kirox
