#pragma once
#include <QJsonObject>
#include <QString>

namespace kirox {
struct Settings {
    QString emailProxyMode = QStringLiteral("follow-task");
    QString emailProxy;
    int otpTimeoutSeconds = 120;
    QString retryProfile = QStringLiteral("standard");
    bool stopOnRisk = true;
    bool soundEnabled = true;
    bool desktopNotifications = true;
    int soundVolume = 70;
    bool autoCheckUpdates = true;
    QString theme = QStringLiteral("system");
    QString language;
    bool persistentLogs = false;
    int logRetentionDays = 7;
    bool autoProbeProxies = true;
    int moeMailExpiryMinutes = 60;
    bool reduceMotion = false;
    bool reduceTransparency = false;

    [[nodiscard]] static Settings fromJson(const QJsonObject &json);
    [[nodiscard]] QJsonObject toJson() const;
    void normalize();
    [[nodiscard]] QString mailboxProxy(const QString &taskProxy) const;
};
} // namespace kirox
