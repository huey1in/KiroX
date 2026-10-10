#pragma once
#include "kirox/ports/transport.hpp"
#include <QJsonObject>
namespace kirox {
class UpdateService {
  public:
    explicit UpdateService(const ITransportFactory &transport,
                           QUrl endpoint = QUrl("https://api.github.com/repos/huey1in/KiroX/releases/latest"))
        : transport_(transport), endpoint_(std::move(endpoint)) {}
    QJsonObject check(const QString &current, std::stop_token stop = {}) const;

  private:
    const ITransportFactory &transport_;
    QUrl endpoint_;
};
} // namespace kirox
