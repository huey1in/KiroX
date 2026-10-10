#pragma once
#include <QJsonObject>
#include <QString>
namespace kirox {
[[nodiscard]] QString normalizeProxy(const QString &input);
[[nodiscard]] QString redactProxy(const QString &input);
struct ProxyEntry {
    QString id, name, url;
    int weight = 50;
    bool enabled = true;
    QJsonObject probe;
    [[nodiscard]] static ProxyEntry fromJson(const QJsonObject &object);
    [[nodiscard]] QJsonObject toJson() const;
};
} // namespace kirox
