#pragma once
#include "kirox/domain/browser_identity.hpp"
#include "kirox/ports/repository.hpp"
#include <QDateTime>

namespace kirox {
class IdentityService {
  public:
    explicit IdentityService(IRepository &repository) : repository_(repository) {}
    BrowserIdentity forProxy(const QString &proxy, bool registration = true,
                             qint64 nowSeconds = QDateTime::currentSecsSinceEpoch());

  private:
    IRepository &repository_;
};
} // namespace kirox
