#include "kirox/application/password_encryptor.hpp"
#include <QDateTime>
#include <QJsonDocument>
#include <QUuid>

namespace kirox {
QString PasswordEncryptor::encrypt(const QString &password, const QJsonObject &publicKey, const QString &issuer,
                                   const QString &audience, const QString &region, qint64 now) const {
    const auto encode = [](const QByteArray &value) {
        return value.toBase64(QByteArray::Base64UrlEncoding | QByteArray::OmitTrailingEquals);
    };
    const QJsonObject header{{"alg", "RSA-OAEP-256"},
                             {"kid", publicKey["kid"]},
                             {"enc", "A256GCM"},
                             {"cty", "enc"},
                             {"typ", "application/aws+signin+jwe"}};
    const auto aad = encode(QJsonDocument(header).toJson(QJsonDocument::Compact));
    auto key = crypto_.randomBytes(32);
    struct Wipe {
        QByteArray &bytes;
        ~Wipe() {
            volatile char *memory = bytes.data();
            for (qsizetype i = 0; i < bytes.size(); ++i)
                memory[i] = 0;
        }
    } wipe{key};
    const auto encryptedKey = crypto_.rsaOaep256(publicKey, key);
    if (now == 0)
        now = QDateTime::currentSecsSinceEpoch();
    const QJsonObject claims{{"iss", region + "." + issuer},
                             {"aud", region + "." + audience},
                             {"iat", now},
                             {"nbf", now},
                             {"exp", now + 300},
                             {"jti", QUuid::createUuid().toString(QUuid::WithoutBraces)},
                             {"password", password}};
    const auto nonce = crypto_.randomBytes(12);
    const auto result = crypto_.aes256Gcm(key, nonce, QJsonDocument(claims).toJson(QJsonDocument::Compact), aad);
    return QString::fromLatin1(aad + '.' + encode(encryptedKey) + '.' + encode(nonce) + '.' +
                               encode(result.ciphertext) + '.' + encode(result.tag));
}
} // namespace kirox
