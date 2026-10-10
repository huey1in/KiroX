#pragma once
#include "kirox/domain/browser_identity.hpp"
#include <QDateTime>
#include <QJsonObject>
#include <optional>

namespace kirox {
struct FingerprintEvent {
    QString location, referrer, page = "signin", event = "PageLoad", email;
    int timeOnPage = 0;
};
// One context belongs to one registration. Hardware remains stable across pages.
class FingerprintContext {
  public:
    explicit FingerprintContext(BrowserIdentity identity);
    void resetPage();
    void setBundleHash(const QString &hash);
    [[nodiscard]] const BrowserIdentity &identity() const {
        return identity_;
    }
    QByteArray serialize(const FingerprintEvent &event, const QString &version,
                         qint64 nowMilliseconds = QDateTime::currentMSecsSinceEpoch());

  private:
    BrowserIdentity identity_;
    QJsonObject timing_;
    std::optional<qint64> start_;
    QString signinId_, profileId_;
};
} // namespace kirox
