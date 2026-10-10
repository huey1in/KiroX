#include "kirox/infrastructure/qt_transport.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/proxy.hpp"
#include <QEventLoop>
#include <QJsonParseError>
#include <QNetworkCookie>
#include <QNetworkCookieJar>
#include <QNetworkProxy>
#include <QNetworkReply>
#include <QPointer>
#include <QTimer>

namespace kirox {
QJsonDocument HttpResponse::json() const {
    QJsonParseError error;
    const auto value = QJsonDocument::fromJson(body, &error);
    if (error.error != QJsonParseError::NoError)
        throw Error(ErrorCode::Protocol, "Response is not valid JSON: " + error.errorString());
    return value;
}
void HttpResponse::requireSuccess() const {
    if (status < 200 || status >= 300)
        throw Error(ErrorCode::Protocol, "HTTP " + QString::number(status));
}
QtTransport::QtTransport(const TransportOptions &options)
    : owner_(QThread::currentThread()), userAgent_(options.userAgent) {
    if (options.proxy.isEmpty())
        manager_.setProxy(QNetworkProxy::NoProxy);
    else {
        const QUrl proxy(normalizeProxy(options.proxy));
        if (proxy.scheme() == "https")
            throw Error(ErrorCode::InvalidInput, "HTTPS proxy transport requires the TLS tunnel adapter");
        manager_.setProxy(
            QNetworkProxy(proxy.scheme().startsWith("socks") ? QNetworkProxy::Socks5Proxy : QNetworkProxy::HttpProxy,
                          proxy.host(), static_cast<quint16>(proxy.port()), proxy.userName(), proxy.password()));
    }
}
void QtTransport::assertThread() const {
    if (owner_ != QThread::currentThread())
        throw Error(ErrorCode::Conflict, "HTTP session used from another thread");
}
HttpResponse QtTransport::send(const HttpRequest &request, std::stop_token stop) {
    assertThread();
    checkCancelled(stop);
    if (!request.url.isValid() || request.url.host().isEmpty())
        throw Error(ErrorCode::InvalidInput, "Invalid request URL");
    QNetworkRequest networkRequest(request.url);
    networkRequest.setAttribute(QNetworkRequest::RedirectPolicyAttribute,
                                request.followRedirects ? QNetworkRequest::NoLessSafeRedirectPolicy
                                                        : QNetworkRequest::ManualRedirectPolicy);
    if (!userAgent_.isEmpty())
        networkRequest.setHeader(QNetworkRequest::UserAgentHeader, userAgent_);
    for (auto it = request.headers.constBegin(); it != request.headers.constEnd(); ++it)
        networkRequest.setRawHeader(it.key(), it.value());
    QNetworkReply *reply = manager_.sendCustomRequest(networkRequest, request.method, request.body);
    QEventLoop loop;
    QTimer timeout;
    timeout.setSingleShot(true);
    bool expired = false;
    QObject::connect(&timeout, &QTimer::timeout, reply, [&] {
        expired = true;
        reply->abort();
    });
    QObject::connect(reply, &QNetworkReply::finished, &loop, &QEventLoop::quit);
    // The callback posts into the reply's thread. It never touches a QObject off-thread.
    std::stop_callback cancel(
        stop, [reply] { QMetaObject::invokeMethod(reply, &QNetworkReply::abort, Qt::QueuedConnection); });
    timeout.start(request.timeout);
    if (!reply->isFinished())
        loop.exec();
    timeout.stop();
    HttpResponse result;
    result.status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
    result.body = reply->readAll();
    for (const auto &header : reply->rawHeaderPairs())
        result.headers.insert(header.first.toLower(), header.second);
    const auto networkError = reply->error();
    const auto errorMessage = reply->errorString();
    reply->deleteLater();
    checkCancelled(stop);
    if (expired)
        throw Error(ErrorCode::Timeout, "Request timed out");
    if (networkError != QNetworkReply::NoError && result.status == 0)
        throw Error(ErrorCode::Network, errorMessage);
    return result;
}
void QtTransport::setCookie(const QUrl &origin, const QByteArray &name, const QByteArray &value) {
    assertThread();
    QNetworkCookie cookie(name, value);
    cookie.setPath("/");
    manager_.cookieJar()->setCookiesFromUrl({cookie}, origin);
}
QByteArray QtTransport::cookie(const QUrl &origin, const QByteArray &name) const {
    assertThread();
    for (const auto &c : manager_.cookieJar()->cookiesForUrl(origin))
        if (c.name() == name)
            return c.value();
    return {};
}
std::unique_ptr<ITransport> QtTransportFactory::create(const TransportOptions &options) const {
    return std::make_unique<QtTransport>(options);
}
} // namespace kirox
