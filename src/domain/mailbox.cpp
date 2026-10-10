#include "kirox/domain/mailbox.hpp"
#include "kirox/domain/error.hpp"
#include <QRegularExpression>
#include <QSet>
#include <QUrl>

namespace kirox {
QString providerId(MailboxKind kind) {
    switch (kind) {
    case MailboxKind::Outlook:
        return "outlook";
    case MailboxKind::ICloud:
        return "icloud";
    case MailboxKind::MoeMail:
        return "moemail";
    case MailboxKind::CloudMail:
        return "cloudmail";
    case MailboxKind::MailNest:
        return "mailnest";
    }
    throw Error(ErrorCode::InvalidInput, "Unknown mailbox provider");
}
MailboxKind providerKind(const QString &id) {
    const auto value = id.trimmed().toLower();
    for (auto kind : {MailboxKind::Outlook, MailboxKind::ICloud, MailboxKind::MoeMail, MailboxKind::CloudMail,
                      MailboxKind::MailNest})
        if (providerId(kind) == value)
            return kind;
    if (value.isEmpty())
        return MailboxKind::Outlook;
    throw Error(ErrorCode::InvalidInput, "Unknown mailbox provider: " + id);
}
MailboxAccount MailboxAccount::fromJson(QJsonObject object) {
    MailboxAccount a;
    a.provider = providerKind(object.value("provider").toString());
    a.email = object.value("email").toString();
    a.password = object.value("password").toString();
    a.clientId = object.value("clientId").toString();
    a.refreshToken = object.value("refreshToken").toString();
    a.messagesUrl = object.value("messagesURL").toString();
    a.mode = object.value("mode").toString().toLower() == "graph" ? "graph" : "imap";
    a.registered = object.value("registered").toBool();
    a.success = object.value("success").toBool();
    a.original = std::move(object);
    return a;
}
QJsonObject MailboxAccount::toJson() const {
    auto o = original;
    o["provider"] = providerId(provider);
    o["email"] = email;
    o["registered"] = registered;
    o["success"] = success;
    if (provider == MailboxKind::Outlook) {
        o["password"] = password;
        o["clientId"] = clientId;
        o["refreshToken"] = refreshToken;
        o["mode"] = mode;
    } else if (provider == MailboxKind::ICloud)
        o["messagesURL"] = messagesUrl;
    return o;
}
ParsedAccounts parseAccounts(MailboxKind kind, const QString &input) {
    if (kind != MailboxKind::Outlook && kind != MailboxKind::ICloud)
        throw Error(ErrorCode::InvalidInput, "Only mailbox pools accept account imports");
    ParsedAccounts result;
    auto lines = input.trimmed().split('\n');
    if (kind == MailboxKind::Outlook && lines.size() == 1)
        lines = input.split(QRegularExpression("\\s+"), Qt::SkipEmptyParts);
    for (auto line : lines) {
        line = line.trimmed();
        if (line.isEmpty() || line.startsWith('#'))
            continue;
        const auto parts = line.split("----");
        MailboxAccount a;
        a.provider = kind;
        if (kind == MailboxKind::Outlook && (parts.size() == 4 || parts.size() == 5)) {
            a.email = parts[0].trimmed();
            a.password = parts[1].trimmed();
            a.clientId = parts[2].trimmed();
            a.refreshToken = parts[3].trimmed();
            if (parts.size() == 5 && parts[4].trimmed().toLower() == "graph")
                a.mode = "graph";
            if (a.email.isEmpty() || a.clientId.isEmpty() || a.refreshToken.isEmpty()) {
                ++result.invalid;
                continue;
            }
        } else if (kind == MailboxKind::ICloud && parts.size() >= 2) {
            a.email = parts[0].trimmed();
            a.messagesUrl = parts.mid(1).join("----").trimmed();
            const QUrl url(a.messagesUrl);
            if (a.email.isEmpty() || url.host().isEmpty() || (url.scheme() != "http" && url.scheme() != "https")) {
                ++result.invalid;
                continue;
            }
        } else {
            ++result.invalid;
            continue;
        }
        result.accounts.push_back(std::move(a));
    }
    return result;
}
QString extractVerificationCode(QString content, bool rejectPlaceholders) {
    content.remove(
        QRegularExpression("<(?:style|script)\\b[^>]*>.*?</(?:style|script)\\s*>",
                           QRegularExpression::CaseInsensitiveOption | QRegularExpression::DotMatchesEverythingOption));
    content.replace(QRegularExpression("<[^>]*>"), " ");
    const QRegularExpression entity("&#(x[0-9a-f]+|[0-9]+);", QRegularExpression::CaseInsensitiveOption);
    auto entities = entity.globalMatch(content);
    QString decoded;
    qsizetype offset = 0;
    while (entities.hasNext()) {
        const auto match = entities.next();
        auto digits = match.captured(1);
        const bool hex = digits.startsWith('x', Qt::CaseInsensitive);
        if (hex)
            digits.remove(0, 1);
        bool valid = false;
        const auto number = digits.toUInt(&valid, hex ? 16 : 10);
        decoded += content.mid(offset, match.capturedStart() - offset);
        if (valid && number <= 0x10ffff && !(number >= 0xd800 && number <= 0xdfff)) {
            const auto codepoint = static_cast<char32_t>(number);
            decoded += QString::fromUcs4(&codepoint, 1);
        } else
            decoded += match.captured();
        offset = match.capturedEnd();
    }
    content = decoded + content.mid(offset);
    content.replace("&nbsp;", " ")
        .replace("&quot;", "\"")
        .replace("&apos;", "'")
        .replace("&lt;", "<")
        .replace("&gt;", ">")
        .replace("&amp;", "&");
    auto matches = QRegularExpression("(?<![0-9])([0-9]{6})(?![0-9])").globalMatch(content);
    while (matches.hasNext()) {
        const auto code = matches.next().captured(1);
        if (!rejectPlaceholders || (code != "000000" && code != "111111" && code != "123456"))
            return code;
    }
    return {};
}
QString decodeMessageBody(const QString &content) {
    if (!content.startsWith("data:", Qt::CaseInsensitive))
        return content;
    const auto comma = content.indexOf(',');
    if (comma < 0)
        throw Error(ErrorCode::Protocol, "Malformed message data URL");
    const auto payload = content.mid(comma + 1).toUtf8();
    if (content.left(comma).contains(";base64", Qt::CaseInsensitive)) {
        const auto decoded = QByteArray::fromBase64Encoding(payload, QByteArray::AbortOnBase64DecodingErrors);
        if (!decoded)
            throw Error(ErrorCode::Protocol, "Invalid message base64");
        return QString::fromUtf8(decoded.decoded);
    }
    return QUrl::fromPercentEncoding(payload);
}
QStringList iCloudMessageIds(const QString &html) {
    QStringList result;
    QSet<QString> seen;
    auto matches = QRegularExpression("data-id\\s*=\\s*[\"']([^\"']+)[\"']", QRegularExpression::CaseInsensitiveOption)
                       .globalMatch(html);
    while (matches.hasNext()) {
        const auto id = matches.next().captured(1);
        if (!seen.contains(id)) {
            seen.insert(id);
            result.append(id);
        }
    }
    return result;
}
QString iCloudDetailUrl(const QString &listUrl, const QString &id) {
    auto bytes = QUrl(listUrl).toEncoded();
    const auto marker = bytes.indexOf("/messages/");
    if (marker < 0 || id.isEmpty())
        throw Error(ErrorCode::InvalidInput, "Invalid iCloud message list URL");
    bytes.replace(marker, 10, "/message/" + QUrl::toPercentEncoding(id) + "/");
    return QString::fromUtf8(bytes);
}
} // namespace kirox
