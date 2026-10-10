#include "kirox/domain/settings.hpp"
#include "kirox/domain/proxy.hpp"
#include <QStringList>

namespace kirox {
Settings Settings::fromJson(const QJsonObject &o) {
    Settings s;
#define TEXT(field) s.field = o.value(QStringLiteral(#field)).toString(s.field)
#define NUMBER(field) s.field = o.value(QStringLiteral(#field)).toInt(s.field)
#define FLAG(field) s.field = o.value(QStringLiteral(#field)).toBool(s.field)
    TEXT(emailProxyMode);
    TEXT(emailProxy);
    TEXT(retryProfile);
    TEXT(theme);
    TEXT(language);
    NUMBER(otpTimeoutSeconds);
    NUMBER(soundVolume);
    NUMBER(logRetentionDays);
    NUMBER(moeMailExpiryMinutes);
    FLAG(stopOnRisk);
    FLAG(soundEnabled);
    FLAG(desktopNotifications);
    FLAG(autoCheckUpdates);
    FLAG(persistentLogs);
    FLAG(autoProbeProxies);
    FLAG(reduceMotion);
    FLAG(reduceTransparency);
#undef TEXT
#undef NUMBER
#undef FLAG
    s.normalize();
    return s;
}
QJsonObject Settings::toJson() const {
    QJsonObject o;
#define FIELD(field) o.insert(QStringLiteral(#field), field)
    FIELD(emailProxyMode);
    FIELD(emailProxy);
    FIELD(otpTimeoutSeconds);
    FIELD(retryProfile);
    FIELD(stopOnRisk);
    FIELD(soundEnabled);
    FIELD(desktopNotifications);
    FIELD(soundVolume);
    FIELD(autoCheckUpdates);
    FIELD(theme);
    FIELD(language);
    FIELD(persistentLogs);
    FIELD(logRetentionDays);
    FIELD(autoProbeProxies);
    FIELD(moeMailExpiryMinutes);
    FIELD(reduceMotion);
    FIELD(reduceTransparency);
#undef FIELD
    return o;
}
void Settings::normalize() {
    if (!QStringList{"direct", "follow-task", "custom"}.contains(emailProxyMode))
        emailProxyMode = "follow-task";
    emailProxy = emailProxy.trimmed();
    if (emailProxyMode != "custom")
        emailProxy.clear();
    else if (!emailProxy.isEmpty())
        emailProxy = normalizeProxy(emailProxy);
    if (otpTimeoutSeconds != 60 && otpTimeoutSeconds != 120 && otpTimeoutSeconds != 180 && otpTimeoutSeconds != 300)
        otpTimeoutSeconds = 120;
    if (!QStringList{"fast", "standard", "stable"}.contains(retryProfile))
        retryProfile = "standard";
    if (soundVolume < 0 || soundVolume > 100)
        soundVolume = 70;
    if (!QStringList{"system", "light", "dark"}.contains(theme))
        theme = "system";
    if (!QStringList{"", "zh", "en", "ja"}.contains(language))
        language.clear();
    if (logRetentionDays < 1 || logRetentionDays > 90)
        logRetentionDays = 7;
    if (moeMailExpiryMinutes < 10 || moeMailExpiryMinutes > 1440)
        moeMailExpiryMinutes = 60;
}
QString Settings::mailboxProxy(const QString &taskProxy) const {
    if (emailProxyMode == "custom")
        return emailProxy;
    return emailProxyMode == "follow-task" ? taskProxy : QString{};
}
} // namespace kirox
