#pragma once
#include <QJsonArray>
#include <QJsonObject>
#include <QString>
#include <vector>

namespace kirox {
enum class MailboxKind { Outlook, ICloud, MoeMail, CloudMail, MailNest };
[[nodiscard]] QString providerId(MailboxKind kind);
[[nodiscard]] MailboxKind providerKind(const QString &id);
struct MailboxAccount {
    MailboxKind provider = MailboxKind::Outlook;
    QString email, password, clientId, refreshToken, messagesUrl;
    QString mode = QStringLiteral("imap");
    bool registered = false;
    bool success = false;
    QJsonObject original;
    [[nodiscard]] static MailboxAccount fromJson(QJsonObject object);
    [[nodiscard]] QJsonObject toJson() const;
};
struct ImportResult {
    int added = 0;
    int duplicates = 0;
    int invalid = 0;
};
struct ParsedAccounts {
    std::vector<MailboxAccount> accounts;
    int invalid = 0;
};
[[nodiscard]] ParsedAccounts parseAccounts(MailboxKind kind, const QString &input);
[[nodiscard]] QString extractVerificationCode(QString content, bool rejectPlaceholders = false);
[[nodiscard]] QString decodeMessageBody(const QString &content);
[[nodiscard]] QStringList iCloudMessageIds(const QString &html);
[[nodiscard]] QString iCloudDetailUrl(const QString &listUrl, const QString &id);
} // namespace kirox
