#include "kirox/application/mailbox_service.hpp"
#include "kirox/domain/cancellation.hpp"
#include <QRandomGenerator>
#include <QSet>

namespace kirox {
namespace {
Document configurationDocument(MailboxKind provider) {
    if (provider == MailboxKind::MoeMail)
        return Document::MoeMail;
    if (provider == MailboxKind::CloudMail)
        return Document::CloudMail;
    if (provider == MailboxKind::MailNest)
        return Document::MailNest;
    throw Error(ErrorCode::InvalidInput, "Mailbox pools have no service configuration");
}
} // namespace
void MailboxConfigurationService::validate(MailboxKind provider, const QJsonObject &configuration) {
    const auto require = [&](const char *key) {
        if (configuration[key].toString().trimmed().isEmpty())
            throw Error(ErrorCode::InvalidInput, QString("Missing mailbox configuration field: ") + key);
    };
    if (provider == MailboxKind::MailNest) {
        require("apiKey");
        require("projectCode");
        return;
    }
    require("name");
    require("url");
    const QUrl url(configuration["url"].toString());
    if (url.host().isEmpty() || (url.scheme() != "http" && url.scheme() != "https") || !url.userInfo().isEmpty() ||
        !url.query().isEmpty() || !url.fragment().isEmpty())
        throw Error(ErrorCode::InvalidInput, "Invalid mailbox API base URL");
    if (provider == MailboxKind::MoeMail)
        require("apiKey");
    else {
        require("email");
        require("password");
    }
}
QJsonDocument MailboxConfigurationService::get(MailboxKind provider) const {
    return repository_.read(configurationDocument(provider));
}
void MailboxConfigurationService::save(MailboxKind provider, const QJsonDocument &configuration) {
    const auto document = configurationDocument(provider);
    if (provider == MailboxKind::MailNest) {
        if (!configuration.isObject())
            throw Error(ErrorCode::InvalidInput, "MailNest configuration must be an object");
        // An empty object explicitly disables the provider.
        if (!configuration.object().isEmpty())
            validate(provider, configuration.object());
    } else {
        if (!configuration.isArray())
            throw Error(ErrorCode::InvalidInput, "Mailbox configurations must be an array");
        QSet<QString> names;
        for (const auto &entry : configuration.array()) {
            if (!entry.isObject())
                throw Error(ErrorCode::InvalidInput, "Invalid mailbox configuration entry");
            validate(provider, entry.toObject());
            const auto name = entry.toObject()["name"].toString().trimmed();
            if (names.contains(name))
                throw Error(ErrorCode::Conflict, "Duplicate mailbox configuration name");
            names.insert(name);
        }
    }
    repository_.update(document, [&](QJsonDocument &stored) { stored = configuration; });
}
QString generateMailboxName() {
    constexpr auto alphabet = "abcdefghijklmnopqrstuvwxyz0123456789";
    auto *random = QRandomGenerator::system();
    QString name;
    const int length = random->bounded(10, 16);
    name.reserve(length);
    for (int i = 0; i < length; ++i)
        name += QLatin1Char(alphabet[random->bounded(36)]);
    return name;
}
void MailboxConfigurationService::upsert(MailboxKind provider, const QJsonObject &configuration,
                                         const QString &oldName) {
    validate(provider, configuration);
    repository_.update(configurationDocument(provider), [&](QJsonDocument &stored) {
        if (provider == MailboxKind::MailNest) {
            auto object = stored.object();
            for (auto it = configuration.begin(); it != configuration.end(); ++it)
                object[it.key()] = it.value();
            stored.setObject(object);
            return;
        }
        const auto name = configuration["name"].toString().trimmed();
        auto entries = stored.array();
        int index = -1;
        for (qsizetype i = 0; i < entries.size(); ++i) {
            const auto existing = entries[i].toObject()["name"].toString();
            if (!oldName.isEmpty() && existing == oldName)
                index = static_cast<int>(i);
            else if (existing == name)
                throw Error(ErrorCode::Conflict, "Duplicate mailbox configuration name");
        }
        if (!oldName.isEmpty() && index < 0)
            throw Error(ErrorCode::NotFound, "Mailbox configuration was removed");
        auto object = index < 0 ? QJsonObject{} : entries[index].toObject();
        for (auto it = configuration.begin(); it != configuration.end(); ++it)
            object[it.key()] = it.value();
        object["name"] = name;
        if (index < 0)
            entries.append(object);
        else
            entries[index] = object;
        stored.setArray(entries);
    });
}
void MailboxConfigurationService::remove(MailboxKind provider, const QString &name) {
    repository_.update(configurationDocument(provider), [&](QJsonDocument &stored) {
        if (provider == MailboxKind::MailNest) {
            stored.setObject({});
            return;
        }
        auto entries = stored.array();
        for (qsizetype i = entries.size(); i > 0; --i)
            if (entries[i - 1].toObject()["name"].toString() == name)
                entries.removeAt(i - 1);
        stored.setArray(entries);
    });
}
QString waitForVerificationCode(IMailboxSession &session, std::chrono::milliseconds timeout,
                                std::chrono::milliseconds interval, std::stop_token stop,
                                const std::function<void(const QString &)> &onRetry) {
    checkCancelled(stop);
    if (timeout.count() <= 0 || interval.count() <= 0)
        throw Error(ErrorCode::InvalidInput, "Invalid OTP polling timing");
    const auto deadline = std::chrono::steady_clock::now() + timeout;
    for (;;) {
        checkCancelled(stop);
        auto remaining =
            std::chrono::duration_cast<std::chrono::milliseconds>(deadline - std::chrono::steady_clock::now());
        if (remaining.count() <= 0)
            throw Error(ErrorCode::Timeout, "Verification code timed out");
        try {
            const auto code = session.poll(stop, remaining);
            checkCancelled(stop);
            if (!code.isEmpty())
                return code;
        } catch (const Error &error) {
            checkCancelled(stop);
            if (error.code() != ErrorCode::Network && error.code() != ErrorCode::Timeout &&
                error.code() != ErrorCode::Protocol)
                throw;
            if (onRetry)
                onRetry(QString::fromUtf8(error.what()));
        }
        remaining = std::chrono::duration_cast<std::chrono::milliseconds>(deadline - std::chrono::steady_clock::now());
        if (remaining.count() <= 0)
            throw Error(ErrorCode::Timeout, "Verification code timed out");
        interruptibleWait(stop, std::min(interval, remaining));
    }
}
} // namespace kirox
