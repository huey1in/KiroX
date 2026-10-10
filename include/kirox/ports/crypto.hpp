#pragma once
#include <QByteArray>
#include <QJsonObject>

namespace kirox {
struct SealedMessage {
    QByteArray ciphertext, tag;
};
struct EphemeralSignature {
    QByteArray signature;
    QJsonObject publicKey;
};
class ICryptography {
  public:
    virtual ~ICryptography() = default;
    virtual QByteArray randomBytes(int size) const = 0;
    virtual EphemeralSignature signEs256(const QByteArray &message) const = 0;
    virtual QByteArray rsaOaep256(const QJsonObject &publicKey, const QByteArray &plaintext) const = 0;
    virtual SealedMessage aes256Gcm(const QByteArray &key, const QByteArray &nonce, const QByteArray &plaintext,
                                    const QByteArray &additionalData) const = 0;
};
} // namespace kirox
