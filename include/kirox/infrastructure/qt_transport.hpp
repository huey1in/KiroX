#pragma once
#include "kirox/ports/transport.hpp"
#include <QNetworkAccessManager>
#include <QThread>
namespace kirox {
// One session belongs to one worker thread; each task receives its own cookies.
class QtTransport final : public ITransport {
  public:
    explicit QtTransport(const TransportOptions &options);
    [[nodiscard]] HttpResponse send(const HttpRequest &request, std::stop_token stop = {}) override;
    void setCookie(const QUrl &origin, const QByteArray &name, const QByteArray &value) override;
    [[nodiscard]] QByteArray cookie(const QUrl &origin, const QByteArray &name) const override;

  private:
    void assertThread() const;
    QNetworkAccessManager manager_;
    QThread *owner_;
    QString userAgent_;
};
class QtTransportFactory final : public ITransportFactory {
  public:
    [[nodiscard]] std::unique_ptr<ITransport> create(const TransportOptions &options) const override;
};
} // namespace kirox
