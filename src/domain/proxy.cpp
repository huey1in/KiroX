#include "kirox/domain/proxy.hpp"
#include "kirox/domain/error.hpp"
#include <QRegularExpression>
#include <QStringList>
#include <QUrl>
#include <algorithm>
namespace kirox {
QString normalizeProxy(const QString &input) {
    auto value = input.trimmed();
    if (value.isEmpty())
        return {};
    if (!value.contains("://")) {
        const auto parts = value.split(':');
        if (parts.size() == 4 && !value.contains('@')) {
            QUrl url;
            url.setScheme("http");
            url.setHost(parts[0]);
            bool valid = false;
            const auto port = parts[1].toInt(&valid);
            if (!valid)
                throw Error(ErrorCode::InvalidInput, "Invalid proxy port");
            url.setPort(port);
            url.setUserName(parts[2]);
            url.setPassword(parts[3]);
            value = url.toString(QUrl::FullyEncoded);
        } else
            value.prepend("http://");
    }
    QUrl url(value, QUrl::StrictMode);
    const auto scheme = url.scheme().toLower();
    if (!url.isValid() || url.host().isEmpty() || !QStringList{"http", "https", "socks5", "socks5h"}.contains(scheme))
        throw Error(ErrorCode::InvalidInput, "Expected an HTTP, HTTPS or SOCKS5 proxy URL");
    if (url.port() == 0 || url.port() > 65535)
        throw Error(ErrorCode::InvalidInput, "Invalid proxy port");
    if (url.path() != "" && url.path() != "/")
        throw Error(ErrorCode::InvalidInput, "Proxy URL must not contain a path");
    if (url.hasQuery() || url.hasFragment())
        throw Error(ErrorCode::InvalidInput, "Proxy URL must not contain query or fragment");
    url.setScheme(scheme);
    url.setPath({});
    if (url.port() < 0)
        url.setPort(scheme == "https" ? 443 : scheme.startsWith("socks") ? 1080 : 80);
    return url.toString(QUrl::FullyEncoded);
}
QString redactProxy(const QString &input) {
    QUrl url(input);
    url.setUserName({});
    url.setPassword({});
    return url.toString();
}
ProxyEntry ProxyEntry::fromJson(const QJsonObject &o) {
    ProxyEntry p;
    p.id = o.value("id").toString();
    p.name = o.value("name").toString();
    p.url = o.value("url").toString();
    p.weight = std::clamp(o.value("weight").toInt(50), 1, 100);
    p.enabled = o.value("enabled").toBool(true);
    p.probe = o;
    return p;
}
QJsonObject ProxyEntry::toJson() const {
    auto o = probe;
    o["id"] = id;
    o["name"] = name;
    o["url"] = url;
    o["weight"] = weight;
    o["enabled"] = enabled;
    return o;
}
} // namespace kirox
