#pragma once
#include <QByteArray>
#include <QJsonDocument>
#include <QMap>
#include <QUrl>
#include <chrono>
#include <memory>
#include <stop_token>
namespace kirox {
struct HttpRequest {
    QByteArray method = "GET";
    QUrl url;
    QMap<QByteArray, QByteArray> headers;
    QByteArray body;
    std::chrono::milliseconds timeout{60000};
    bool followRedirects = false;
};
struct HttpResponse {
    int status = 0;
    QByteArray body;
    QMultiMap<QByteArray, QByteArray> headers;
    [[nodiscard]] QJsonDocument json() const;
    void requireSuccess() const;
};
struct TransportOptions {
    QString proxy;
    QString userAgent;
    QString browserProfile;
};
class ITransport {
  public:
    virtual ~ITransport() = default;
    [[nodiscard]] virtual HttpResponse send(const HttpRequest &request, std::stop_token stop = {}) = 0;
    virtual void setCookie(const QUrl &origin, const QByteArray &name, const QByteArray &value) = 0;
    [[nodiscard]] virtual QByteArray cookie(const QUrl &origin, const QByteArray &name) const = 0;
};
class ITransportFactory {
  public:
    virtual ~ITransportFactory() = default;
    [[nodiscard]] virtual std::unique_ptr<ITransport> create(const TransportOptions &options) const = 0;
};
} // namespace kirox
