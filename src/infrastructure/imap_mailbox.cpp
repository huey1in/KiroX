#include "kirox/infrastructure/imap_mailbox.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/mime.hpp"
#include <QJsonObject>
#include <QRegularExpression>

namespace kirox {
namespace {
class ImapMailboxSession final : public IMailboxSession {
  public:
    ImapMailboxSession(MailboxRequest request, std::unique_ptr<ITransport> http, const IImapFactory &factory,
                       QString oauth)
        : request_(std::move(request)), http_(std::move(http)), factory_(factory), oauth_(std::move(oauth)) {}
    QString open(std::stop_token stop) override {
        checkCancelled(stop);
        stop_ = stop;
        deadline_ = Clock::now() + std::chrono::seconds(60);
        if (opened_)
            return request_.account.email;
        QByteArray form;
        for (const auto &item : std::initializer_list<std::pair<QString, QString>>{
                 {"grant_type", "refresh_token"},
                 {"client_id", request_.account.clientId},
                 {"refresh_token", request_.account.refreshToken},
                 {"scope", "https://outlook.office.com/IMAP.AccessAsUser.All offline_access"}}) {
            if (!form.isEmpty())
                form += '&';
            form += QUrl::toPercentEncoding(item.first) + '=' + QUrl::toPercentEncoding(item.second);
        }
        HttpRequest token;
        token.url = QUrl(oauth_ + "/consumers/oauth2/v2.0/token");
        token.method = "POST";
        token.headers = {{"Content-Type", "application/x-www-form-urlencoded"}};
        token.body = form;
        token.timeout = remaining();
        const auto response = http_->send(token, stop);
        response.requireSuccess();
        const auto bearer = response.json().object()["access_token"].toString();
        if (bearer.isEmpty())
            throw Error(ErrorCode::Protocol, "Outlook returned no IMAP token");
        connection_ = factory_.create(request_.account.email, bearer, request_.transport);
        folders_.clear();
        folders_.push_back(baseline("INBOX"));
        QString listing;
        try {
            listing = QString::fromUtf8(connection_->command({}, "LIST \"\" \"*\"", stop, remaining()));
        } catch (const Error &) {
            checkCancelled(stop);
        }
        auto lines = listing.split('\n');
        for (const auto &line : lines) {
            const auto match =
                QRegularExpression(
                    "\\* LIST \\(([^)]*)\\) (?:\"[^\"]*\"|NIL) (?:\"((?:[^\"\\\\]|\\\\.)*)\"|([^\\r\\n]+))",
                    QRegularExpression::CaseInsensitiveOption)
                    .match(line);
            if (!match.hasMatch())
                continue;
            auto folder = match.captured(2).isEmpty() ? match.captured(3).trimmed() : match.captured(2);
            folder.replace("\\\"", "\"").replace("\\\\", "\\");
            if (match.captured(1).contains("\\Junk", Qt::CaseInsensitive) ||
                folder.compare("Junk", Qt::CaseInsensitive) == 0 ||
                folder.compare("Junk Email", Qt::CaseInsensitive) == 0 ||
                folder.compare("Spam", Qt::CaseInsensitive) == 0) {
                try {
                    folders_.push_back(baseline(folder));
                } catch (const Error &) {
                    checkCancelled(stop);
                }
                break;
            }
        }
        opened_ = true;
        return request_.account.email;
    }
    QString poll(std::stop_token stop, std::chrono::milliseconds budget) override {
        if (!opened_)
            throw Error(ErrorCode::Conflict, "IMAP mailbox baseline has not been established");
        stop_ = stop;
        deadline_ = Clock::now() + budget;
        for (const auto &folder : folders_) {
            const auto current = baseline(folder.name);
            if (folder.validity != current.validity)
                throw Error(ErrorCode::Conflict,
                            "IMAP folder UIDVALIDITY changed; reopen the mailbox before sending another code");
            if (current.next <= folder.next)
                continue;
            const auto response = QString::fromUtf8(connection_->command(
                folder.name, "UID SEARCH UID " + QByteArray::number(folder.next) + ":*", stop, remaining()));
            const auto match =
                QRegularExpression("(?:^|\\n)\\* SEARCH([^\\r\\n]*)", QRegularExpression::CaseInsensitiveOption)
                    .match(response);
            auto ids = match.captured(1).split(QRegularExpression("\\s+"), Qt::SkipEmptyParts);
            int inspected = 0;
            for (auto it = ids.crbegin(); it != ids.crend() && inspected < 10; ++it) {
                bool valid = false;
                const auto uid = it->toUInt(&valid);
                if (!valid || uid < folder.next)
                    continue;
                ++inspected;
                const auto content = connection_->message(folder.name, uid, stop, remaining());
                const auto code = extractVerificationCode(mailMessageText(content));
                if (!code.isEmpty())
                    return code;
            }
        }
        return {};
    }

  private:
    using Clock = std::chrono::steady_clock;
    struct Folder {
        QString name;
        quint32 next, validity;
    };
    std::chrono::milliseconds remaining() const {
        checkCancelled(stop_);
        const auto left = std::chrono::duration_cast<std::chrono::milliseconds>(deadline_ - Clock::now());
        if (left.count() <= 0)
            throw Error(ErrorCode::Timeout, "IMAP mailbox deadline exceeded");
        return std::min(left, std::chrono::milliseconds(15000));
    }
    Folder baseline(const QString &folder) {
        auto quoted = folder.toUtf8();
        quoted.replace("\\", "\\\\").replace("\"", "\\\"");
        if (quoted.contains('\r') || quoted.contains('\n'))
            throw Error(ErrorCode::InvalidInput, "Invalid IMAP folder name");
        const auto response = QString::fromUtf8(
            connection_->command({}, "STATUS \"" + quoted + "\" (UIDNEXT UIDVALIDITY)", stop_, remaining()));
        const auto next = QRegularExpression("UIDNEXT\\s+([0-9]+)", QRegularExpression::CaseInsensitiveOption)
                              .match(response)
                              .captured(1)
                              .toUInt();
        const auto validity = QRegularExpression("UIDVALIDITY\\s+([0-9]+)", QRegularExpression::CaseInsensitiveOption)
                                  .match(response)
                                  .captured(1)
                                  .toUInt();
        if (next == 0 || validity == 0)
            throw Error(ErrorCode::Protocol, "IMAP returned no UID baseline");
        return {folder, next, validity};
    }
    MailboxRequest request_;
    std::unique_ptr<ITransport> http_;
    const IImapFactory &factory_;
    QString oauth_;
    std::unique_ptr<IImapConnection> connection_;
    std::vector<Folder> folders_;
    std::stop_token stop_;
    Clock::time_point deadline_;
    bool opened_ = false;
};
} // namespace
std::unique_ptr<IMailboxSession> createImapMailbox(const MailboxRequest &request, const ITransportFactory &transports,
                                                   const IImapFactory &imap, const QString &oauthEndpoint) {
    return std::make_unique<ImapMailboxSession>(request, transports.create(request.transport), imap, oauthEndpoint);
}
} // namespace kirox
