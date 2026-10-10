#include "kirox/infrastructure/http_mailbox.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/infrastructure/imap_mailbox.hpp"
#include <QJsonArray>
#include <QRandomGenerator>
#include <QRegularExpression>
#include <QSet>
#include <QUrlQuery>
#include <algorithm>

namespace kirox {
namespace {
class HttpMailboxSession final : public IMailboxSession {
  public:
    HttpMailboxSession(MailboxRequest request, std::unique_ptr<ITransport> transport, MailboxEndpoints endpoints)
        : request_(std::move(request)), transport_(std::move(transport)), endpoints_(std::move(endpoints)) {}
    QString open(std::stop_token stop) override {
        stop_ = stop;
        deadline_ = std::chrono::steady_clock::now() + std::chrono::seconds(60);
        checkCancelled(stop);
        if (opened_)
            return address_;
        const auto kind = request_.provider;
        if (kind == MailboxKind::ICloud) {
            address_ = request_.account.email;
            for (const auto &id : cloudIds())
                seen_.insert(id);
        } else if (kind == MailboxKind::Outlook) {
            if (request_.account.mode != "graph")
                throw Error(ErrorCode::InvalidInput, "Use the IMAP mailbox adapter for this account");
            address_ = request_.account.email;
            refreshGraphToken();
            inboxBaseline_ = graphCount("inbox");
            try {
                junkBaseline_ = graphCount("junkemail");
            } catch (const Error &) {
                checkCancelled(stop);
                junkBaseline_ = -1;
            }
        } else if (kind == MailboxKind::MoeMail) {
            auto domain = request_.domain;
            const auto available = moeDomains();
            if (!available.contains(domain))
                domain = chooseDomain(available);
            const auto value = object(
                send("POST", base() + "/api/emails/generate",
                     {{"name", request_.name}, {"expiryTime", request_.expiryMilliseconds}, {"domain", domain}}));
            id_ = value["id"].toString();
            address_ = value["email"].toString(value["address"].toString());
            if (id_.isEmpty())
                throw Error(ErrorCode::Protocol, "MoeMail returned no mailbox ID");
            moeBaseline_ = moeMessages().size();
        } else if (kind == MailboxKind::CloudMail) {
            auto domain = request_.domain;
            if (domain.isEmpty()) {
                auto domains = strings(request_.configuration["domains"]);
                if (domains.isEmpty())
                    domains = cloudDomains();
                if (domains.isEmpty())
                    throw Error(ErrorCode::Protocol, "Cloud-Mail returned no domains");
                domain = chooseDomain(domains);
            }
            address_ = request_.name + "@" + domain;
            (void)cloudAuthorized("/api/public/addUser", {{"list", QJsonArray{QJsonObject{{"email", address_}}}}});
            for (const auto &entry : cloudMessages(address_, 5))
                cloudBaseline_ = std::max(cloudBaseline_, entry.toObject()["emailId"].toInteger());
        } else {
            const auto entries = array(nest("POST", "/api/v1/email/temporary/buy",
                                            {{"count", 1}, {"project_code", request_.configuration["projectCode"]}}));
            if (entries.isEmpty())
                throw Error(ErrorCode::Protocol, "MailNest returned no mailbox");
            address_ = entries[0].toObject()["email"].toString();
        }
        if (address_.isEmpty())
            throw Error(ErrorCode::Protocol, "Provider returned no mailbox address");
        opened_ = true;
        return address_;
    }
    QString poll(std::stop_token stop, std::chrono::milliseconds budget) override {
        if (!opened_)
            throw Error(ErrorCode::Conflict, "Mailbox baseline has not been established");
        stop_ = stop;
        deadline_ = std::chrono::steady_clock::now() + budget;
        checkCancelled(stop);
        if (request_.provider == MailboxKind::ICloud) {
            for (const auto &id : cloudIds()) {
                if (seen_.contains(id))
                    continue;
                // Only successful detail reads are marked seen; transient errors retry.
                const auto value = object(send("GET", iCloudDetailUrl(request_.account.messagesUrl, id)));
                const auto code = extractVerificationCode(
                    value["subject"].toString() + " " + decodeMessageBody(value["body"].toString()), true);
                seen_.insert(id);
                if (!code.isEmpty())
                    return code;
            }
        } else if (request_.provider == MailboxKind::MoeMail) {
            const auto entries = moeMessages();
            const auto count = std::max<qsizetype>(0, entries.size() - moeBaseline_);
            for (qsizetype i = 0; i < count; ++i) {
                const auto code = messageCode(entries[i].toObject(), {"content", "html", "subject"});
                if (!code.isEmpty())
                    return code;
            }
        } else if (request_.provider == MailboxKind::CloudMail) {
            for (const auto &entry : cloudMessages(address_, 20)) {
                const auto value = entry.toObject();
                if (value["emailId"].toInteger() <= cloudBaseline_)
                    continue;
                const auto code = messageCode(value, {"text", "content", "subject"});
                if (!code.isEmpty())
                    return code;
            }
        } else if (request_.provider == MailboxKind::MailNest) {
            for (const auto &entry : array(nest("POST", "/api/v1/email/receive", {{"email", address_}}))) {
                const auto code = entry.toObject()["code_match"].toString();
                if (QRegularExpression("^[0-9]{6}$").match(code).hasMatch())
                    return code;
            }
        } else {
            for (const auto &folder : {QString("inbox"), QString("junkemail")}) {
                const auto baseline = folder == "inbox" ? inboxBaseline_ : junkBaseline_;
                if (baseline < 0)
                    continue;
                const auto count = graphCount(folder);
                if (count <= baseline)
                    continue;
                const auto limit = std::min(count - baseline, 10);
                const auto entries = array(graph(
                    "/me/mailFolders/" + folder + "/messages?$top=" + QString::number(limit) +
                    "&$orderby=receivedDateTime%20desc&$select=subject,bodyPreview,body,receivedDateTime")["value"]);
                for (const auto &entry : entries) {
                    auto value = entry.toObject();
                    value["content"] = value["body"].toObject()["content"];
                    const auto code = messageCode(value, {"bodyPreview", "subject", "content"});
                    if (!code.isEmpty())
                        return code;
                }
            }
        }
        return {};
    }
    QJsonObject inspect(std::stop_token stop) {
        stop_ = stop;
        deadline_ = std::chrono::steady_clock::now() + std::chrono::seconds(60);
        if (request_.provider == MailboxKind::MoeMail)
            return {{"domains", QJsonArray::fromStringList(moeDomains())}};
        if (request_.provider == MailboxKind::CloudMail) {
            cloudToken();
            auto domains = cloudDomains();
            (void)cloudMessages({}, 1);
            if (domains.isEmpty())
                domains = strings(request_.configuration["domains"]);
            return {{"domains", QJsonArray::fromStringList(domains)}};
        }
        if (request_.provider == MailboxKind::MailNest)
            return {{"balance", object(nest("GET", "/api/v1/balance"))["balance"]}};
        throw Error(ErrorCode::InvalidInput, "Connection inspection only accepts API providers");
    }

  private:
    using Clock = std::chrono::steady_clock;
    QString chooseDomain(const QStringList &domains) const {
        return domains.at(request_.randomDomains ? QRandomGenerator::system()->bounded(int(domains.size()))
                                                 : std::max(0, request_.domainIndex) % domains.size());
    }
    static QJsonObject object(const QJsonValue &value) {
        if (!value.isObject())
            throw Error(ErrorCode::Protocol, "Expected JSON object from mailbox provider");
        return value.toObject();
    }
    static QJsonArray array(const QJsonValue &value) {
        if (!value.isArray())
            throw Error(ErrorCode::Protocol, "Expected JSON array from mailbox provider");
        return value.toArray();
    }
    static QStringList strings(const QJsonValue &value) {
        QStringList result;
        if (!value.isArray())
            return result;
        for (const auto &entry : value.toArray()) {
            auto domain = entry.toString().trimmed();
            if (domain.startsWith('@'))
                domain.remove(0, 1);
            if (!domain.isEmpty() && !result.contains(domain))
                result.append(domain);
        }
        return result;
    }
    static QString messageCode(const QJsonObject &value, std::initializer_list<const char *> keys) {
        for (const auto *key : keys) {
            const auto code = extractVerificationCode(value[key].toString());
            if (!code.isEmpty())
                return code;
        }
        return {};
    }
    QString base() const {
        auto url = request_.configuration["url"].toString();
        while (url.endsWith('/'))
            url.chop(1);
        return url;
    }
    HttpResponse raw(QByteArray method, const QString &url, const QByteArray &body = {},
                     QMap<QByteArray, QByteArray> headers = {}) {
        checkCancelled(stop_);
        const auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(deadline_ - Clock::now());
        if (remaining.count() <= 0)
            throw Error(ErrorCode::Timeout, "Mailbox request deadline exceeded");
        HttpRequest request;
        request.method = std::move(method);
        request.url = QUrl(url);
        request.body = body;
        request.headers = std::move(headers);
        request.timeout = std::min(remaining, std::chrono::milliseconds(15000));
        if (!request.headers.contains("Accept"))
            request.headers["Accept"] = "application/json";
        if (request_.provider == MailboxKind::MoeMail)
            request.headers["X-API-Key"] = request_.configuration["apiKey"].toString().toUtf8();
        if (request_.provider == MailboxKind::MailNest)
            request.headers["Authorization"] = "Bearer " + request_.configuration["apiKey"].toString().toUtf8();
        return transport_->send(request, stop_);
    }
    QJsonValue send(const QByteArray &method, const QString &url, const QJsonObject &payload = {},
                    QMap<QByteArray, QByteArray> headers = {}) {
        QByteArray body;
        if (method != "GET") {
            body = QJsonDocument(payload).toJson(QJsonDocument::Compact);
            headers["Content-Type"] = "application/json";
        }
        const auto response = raw(method, url, body, std::move(headers));
        response.requireSuccess();
        const auto json = response.json();
        return json.isObject() ? QJsonValue(json.object()) : QJsonValue(json.array());
    }
    static QJsonValue unwrapCloud(const QJsonValue &value) {
        const auto wrap = object(value);
        if (wrap["code"].toInt() != 200)
            throw Error(ErrorCode::Protocol, "Cloud-Mail rejected the request");
        return wrap["data"];
    }
    void cloudToken() {
        const auto data = object(unwrapCloud(
            send("POST", base() + "/api/public/genToken",
                 {{"email", request_.configuration["email"]}, {"password", request_.configuration["password"]}})));
        token_ = data["token"].toString();
        if (token_.isEmpty())
            throw Error(ErrorCode::Protocol, "Cloud-Mail returned no token");
    }
    QJsonValue cloudAuthorized(const QString &path, const QJsonObject &payload) {
        if (token_.isEmpty())
            cloudToken();
        const auto body = QJsonDocument(payload).toJson(QJsonDocument::Compact);
        auto response = raw("POST", base() + path, body,
                            {{"Authorization", token_.toUtf8()}, {"Content-Type", "application/json"}});
        if (response.status == 401) {
            cloudToken();
            response = raw("POST", base() + path, body,
                           {{"Authorization", token_.toUtf8()}, {"Content-Type", "application/json"}});
        }
        response.requireSuccess();
        return unwrapCloud(response.json().object());
    }
    QJsonArray cloudMessages(const QString &address, int size) {
        QJsonObject payload{{"timeSort", "desc"}, {"size", size}, {"num", 1}, {"type", 0}, {"isDel", 0}};
        if (!address.isEmpty())
            payload["toEmail"] = address;
        return array(cloudAuthorized("/api/public/emailList", payload));
    }
    QStringList cloudDomains() {
        auto value = object(unwrapCloud(send("GET", base() + "/api/setting/websiteConfig")));
        auto domains = strings(value["domainList"]);
        if (domains.isEmpty()) {
            if (token_.isEmpty())
                cloudToken();
            value = object(unwrapCloud(
                send("GET", base() + "/api/setting/websiteConfig", {}, {{"Authorization", token_.toUtf8()}})));
            domains = strings(value["domainList"]);
        }
        return domains;
    }
    QStringList moeDomains() {
        QStringList domains;
        for (const auto &entry : object(send("GET", base() + "/api/config"))["emailDomains"].toString().split(',')) {
            const auto domain = entry.trimmed();
            if (!domain.isEmpty() && !domains.contains(domain))
                domains.append(domain);
        }
        if (domains.isEmpty())
            throw Error(ErrorCode::Protocol, "MoeMail returned no domains");
        return domains;
    }
    QJsonArray moeMessages() {
        return array(
            object(send("GET", base() + "/api/emails/" + QString::fromUtf8(QUrl::toPercentEncoding(id_))))["messages"]);
    }
    QJsonValue nest(const QByteArray &method, const QString &path, const QJsonObject &payload = {}) {
        const auto wrap = object(send(method, endpoints_.mailNest + path, payload));
        if (wrap["code"].toString() != "00000")
            throw Error(ErrorCode::Protocol, "MailNest rejected the request");
        return wrap["data"];
    }
    QStringList cloudIds() {
        const auto response =
            raw("GET", request_.account.messagesUrl, {}, {{"Accept", "text/html,application/xhtml+xml"}});
        response.requireSuccess();
        return iCloudMessageIds(QString::fromUtf8(response.body));
    }
    void refreshGraphToken() {
        QUrlQuery form;
        form.addQueryItem("grant_type", "refresh_token");
        form.addQueryItem("client_id", request_.account.clientId);
        form.addQueryItem("refresh_token", request_.account.refreshToken);
        form.addQueryItem("scope", "https://graph.microsoft.com/Mail.Read offline_access");
        // QUrlQuery's FullyEncoded representation leaves '+', which form decoders
        // interpret as a space. Encode every individual form value instead.
        QByteArray body;
        for (const auto &entry : form.queryItems()) {
            if (!body.isEmpty())
                body += '&';
            body += QUrl::toPercentEncoding(entry.first) + '=' + QUrl::toPercentEncoding(entry.second);
        }
        const auto response = raw("POST", endpoints_.outlookOAuth + "/common/oauth2/v2.0/token", body,
                                  {{"Content-Type", "application/x-www-form-urlencoded"}});
        response.requireSuccess();
        token_ = response.json().object()["access_token"].toString();
        if (token_.isEmpty())
            throw Error(ErrorCode::Protocol, "Outlook returned no access token");
    }
    QJsonObject graph(const QString &path) {
        auto response =
            raw("GET", endpoints_.outlookGraph + path, {},
                {{"Authorization", "Bearer " + token_.toUtf8()}, {"Prefer", "outlook.body-content-type=\"text\""}});
        if (response.status == 401) {
            refreshGraphToken();
            response =
                raw("GET", endpoints_.outlookGraph + path, {},
                    {{"Authorization", "Bearer " + token_.toUtf8()}, {"Prefer", "outlook.body-content-type=\"text\""}});
        }
        response.requireSuccess();
        if (!response.json().isObject())
            throw Error(ErrorCode::Protocol, "Invalid Graph response");
        return response.json().object();
    }
    int graphCount(const QString &folder) {
        const auto value = graph("/me/mailFolders/" + folder + "?$select=totalItemCount").value("totalItemCount");
        if (!value.isDouble() || value.toInt(-1) < 0)
            throw Error(ErrorCode::Protocol, "Graph returned no message count");
        return value.toInt();
    }
    MailboxRequest request_;
    std::unique_ptr<ITransport> transport_;
    MailboxEndpoints endpoints_;
    std::stop_token stop_;
    Clock::time_point deadline_;
    QString address_, id_, token_;
    QSet<QString> seen_;
    qsizetype moeBaseline_ = 0;
    qint64 cloudBaseline_ = 0;
    int inboxBaseline_ = 0, junkBaseline_ = -1;
    bool opened_ = false;
};
} // namespace
std::unique_ptr<IMailboxSession> HttpMailboxFactory::create(const MailboxRequest &request) const {
    if (request.provider == MailboxKind::Outlook && request.account.mode != "graph") {
        if (!imap_)
            throw Error(ErrorCode::InvalidInput, "IMAP transport has not been configured");
        return createImapMailbox(request, transports_, *imap_, endpoints_.outlookOAuth);
    }
    return std::make_unique<HttpMailboxSession>(request, transports_.create(request.transport), endpoints_);
}
QJsonObject HttpMailboxFactory::inspect(MailboxKind provider, const QJsonObject &configuration,
                                        const TransportOptions &transport, std::stop_token stop) const {
    MailboxRequest request;
    request.provider = provider;
    request.configuration = configuration;
    HttpMailboxSession session(request, transports_.create(transport), endpoints_);
    return session.inspect(stop);
}
} // namespace kirox
