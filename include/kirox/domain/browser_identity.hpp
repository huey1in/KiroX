#pragma once
#include <QJsonObject>
#include <QString>

namespace kirox {
// The persisted field names also accept identities written by the legacy client.
class BrowserIdentity {
  public:
    explicit BrowserIdentity(QJsonObject properties = {}) : properties_(std::move(properties)) {}
    static BrowserIdentity generate();
    [[nodiscard]] bool valid() const;
    [[nodiscard]] QString majorVersion() const;
    [[nodiscard]] QString userAgent() const;
    [[nodiscard]] QString securityUserAgent() const;
    [[nodiscard]] const QJsonObject &json() const {
        return properties_;
    }
    [[nodiscard]] BrowserIdentity newSession(bool resampleCanvas = false) const;

  private:
    QJsonObject properties_;
};
QString browserIdentityKey(const QString &proxy);
} // namespace kirox
