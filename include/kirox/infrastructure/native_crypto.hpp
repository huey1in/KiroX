#pragma once
#include "kirox/ports/crypto.hpp"

namespace kirox {
class NativeCryptography final : public ICryptography {
  public:
    QByteArray randomBytes(int size) const override;
    EphemeralSignature signEs256(const QByteArray &message) const override;
    QByteArray rsaOaep256(const QJsonObject &publicKey, const QByteArray &plaintext) const override;
    SealedMessage aes256Gcm(const QByteArray &key, const QByteArray &nonce, const QByteArray &plaintext,
                            const QByteArray &additionalData) const override;
};
} // namespace kirox
