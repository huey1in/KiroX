#pragma once
#include "kirox/ports/transport.hpp"

namespace kirox {
// Independent native session, including connection cache, cookies and TLS profile.
class CurlTransport final : public ITransport {
  public:
    explicit CurlTransport(const TransportOptions &options);
    ~CurlTransport() override;
    CurlTransport(const CurlTransport &) = delete;
    CurlTransport &operator=(const CurlTransport &) = delete;
    HttpResponse send(const HttpRequest &request, std::stop_token stop = {}) override;
    void setCookie(const QUrl &origin, const QByteArray &name, const QByteArray &value) override;
    QByteArray cookie(const QUrl &origin, const QByteArray &name) const override;

  private:
    struct Impl;
    std::unique_ptr<Impl> impl_;
};
class CurlTransportFactory final : public ITransportFactory {
  public:
    std::unique_ptr<ITransport> create(const TransportOptions &options) const override;
};
} // namespace kirox
