#pragma once
#include "kirox/ports/crypto.hpp"
#include <QString>

namespace kirox {
class PasswordEncryptor {
  public:
    explicit PasswordEncryptor(const ICryptography &crypto) : crypto_(crypto) {}
    QString encrypt(const QString &password, const QJsonObject &publicKey, const QString &issuer,
                    const QString &audience, const QString &region, qint64 now = 0) const;

  private:
    const ICryptography &crypto_;
};
} // namespace kirox
