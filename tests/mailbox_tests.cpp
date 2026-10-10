#include "kirox/application/mailbox_service.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/mime.hpp"
#include "kirox/infrastructure/http_mailbox.hpp"
#include "kirox/infrastructure/imap_mailbox.hpp"
#include "kirox/infrastructure/json_repository.hpp"
#include <QJsonArray>
#include <QTemporaryDir>
#include <QTest>
#include <atomic>
#include <thread>

using namespace kirox;
namespace {
HttpResponse json(const QJsonValue &value, int status = 200) {
    const QJsonDocument document = value.isObject() ? QJsonDocument(value.toObject()) : QJsonDocument(value.toArray());
    return {status, document.toJson(QJsonDocument::Compact), {}};
}
QJsonObject cloud(const QJsonValue &data) {
    return {{"code", 200}, {"data", data}};
}
QJsonObject nest(const QJsonValue &data) {
    return {{"code", "00000"}, {"data", data}};
}
struct Script {
    std::function<HttpResponse(const HttpRequest &)> send;
    int requests = 0;
    TransportOptions options;
};
class FakeTransport final : public ITransport {
  public:
    explicit FakeTransport(std::shared_ptr<Script> script) : script_(std::move(script)) {}
    HttpResponse send(const HttpRequest &request, std::stop_token stop) override {
        checkCancelled(stop);
        ++script_->requests;
        return script_->send(request);
    }
    void setCookie(const QUrl &, const QByteArray &, const QByteArray &) override {}
    QByteArray cookie(const QUrl &, const QByteArray &) const override {
        return {};
    }

  private:
    std::shared_ptr<Script> script_;
};
class FakeFactory final : public ITransportFactory {
  public:
    std::shared_ptr<Script> script = std::make_shared<Script>();
    std::unique_ptr<ITransport> create(const TransportOptions &options) const override {
        script->options = options;
        return std::make_unique<FakeTransport>(script);
    }
};
} // namespace
class MailboxTests : public QObject {
    Q_OBJECT
  private slots:
    void configurationValidationPreservesStoredData() {
        QTemporaryDir dir;
        JsonRepository repository(dir.path());
        MailboxConfigurationService service(repository);
        const QJsonDocument valid(QJsonArray{
            QJsonObject{{"name", "primary"}, {"url", "https://mail.example"}, {"apiKey", "key"}, {"extension", true}}});
        service.save(MailboxKind::MoeMail, valid);
        QVERIFY_EXCEPTION_THROWN(
            service.save(MailboxKind::MoeMail, QJsonDocument(QJsonArray{QJsonObject{{"name", "invalid"}}})), Error);
        QCOMPARE(service.get(MailboxKind::MoeMail), valid);
        QVERIFY_EXCEPTION_THROWN(service.upsert(MailboxKind::MoeMail, valid.array()[0].toObject()), Error);
        QCOMPARE(service.get(MailboxKind::MoeMail), valid);
        auto renamed = valid.array()[0].toObject();
        renamed["name"] = "renamed";
        service.upsert(MailboxKind::MoeMail, renamed, "primary");
        QCOMPARE(service.get(MailboxKind::MoeMail).array()[0].toObject()["name"].toString(), QString("renamed"));
        QVERIFY(service.get(MailboxKind::MoeMail).array()[0].toObject()["extension"].toBool());
        QVERIFY_EXCEPTION_THROWN(
            service.save(MailboxKind::MoeMail, QJsonDocument(QJsonArray{valid.array()[0], valid.array()[0]})), Error);
        service.save(MailboxKind::MailNest, QJsonDocument(QJsonObject{{"apiKey", "key"}, {"projectCode", "project"}}));
        service.save(MailboxKind::MailNest, QJsonDocument(QJsonObject{}));
        QVERIFY(service.get(MailboxKind::MailNest).object().isEmpty());
    }
    void moeBaselineSkipsOldCodeAndFallsBackDomain() {
        FakeFactory transports;
        HttpMailboxFactory factory(transports);
        int lists = 0;
        bool domainCorrect = false;
        transports.script->send = [&](const HttpRequest &request) {
            if (request.url.path() == "/api/config")
                return json(QJsonObject{{"emailDomains", " first.test,second.test "}});
            if (request.url.path().endsWith("generate")) {
                domainCorrect = QJsonDocument::fromJson(request.body).object()["domain"] == "first.test";
                return json(QJsonObject{{"id", "mail/id"}, {"address", "test@first.test"}}, 201);
            }
            ++lists;
            if (lists == 1)
                return json(QJsonObject{{"messages", QJsonArray{QJsonObject{{"content", "old 654321"}}}}});
            return json(
                QJsonObject{{"messages", QJsonArray{QJsonObject{{"html", "<style>#555555</style><b>865204</b>"}},
                                                    QJsonObject{{"content", "old 654321"}}}}});
        };
        MailboxRequest request;
        request.provider = MailboxKind::MoeMail;
        request.name = "test";
        request.domain = "missing.test";
        request.configuration = {{"url", "https://mail.test"}, {"apiKey", "secret"}};
        request.transport.proxy = "http://proxy.test:80";
        auto session = factory.create(request);
        QCOMPARE(session->open(), QString("test@first.test"));
        QVERIFY(domainCorrect);
        QCOMPARE(session->poll(), QString("865204"));
        QCOMPARE(transports.script->options.proxy, request.transport.proxy);
    }
    void cloudUnauthorizedRefreshIsBoundedAndBaselineUsesId() {
        FakeFactory transports;
        HttpMailboxFactory factory(transports);
        int tokens = 0, lists = 0;
        bool usedRawToken = true;
        transports.script->send = [&](const HttpRequest &request) {
            if (request.url.path().endsWith("genToken"))
                return json(cloud(QJsonObject{{"token", "token" + QString::number(++tokens)}}));
            usedRawToken &= request.headers.value("Authorization").startsWith("token");
            if (request.url.path().endsWith("addUser"))
                return json(cloud(QJsonObject{}));
            ++lists;
            if (lists == 1)
                return json(cloud(QJsonArray{QJsonObject{{"emailId", 77}, {"text", "old 654321"}}}));
            if (lists == 2)
                return json(QJsonObject{}, 401);
            return json(cloud(QJsonArray{QJsonObject{{"emailId", 78}, {"text", "new 876543"}},
                                         QJsonObject{{"emailId", 77}, {"text", "old 654321"}}}));
        };
        MailboxRequest request;
        request.provider = MailboxKind::CloudMail;
        request.name = "test";
        request.domain = "mail.test";
        request.configuration = {{"url", "https://cloud.test"}, {"email", "admin@test"}, {"password", "secret"}};
        auto session = factory.create(request);
        QCOMPARE(session->open(), QString("test@mail.test"));
        QCOMPARE(session->poll(), QString("876543"));
        QCOMPARE(tokens, 2);
        QVERIFY(usedRawToken);
    }
    void mailNestChecksBusinessStatusAndCodeDigits() {
        FakeFactory transports;
        HttpMailboxFactory factory(transports);
        int reads = 0;
        transports.script->send = [&](const HttpRequest &request) {
            if (request.url.path().endsWith("buy"))
                return json(nest(QJsonArray{QJsonObject{{"email", "paid@test"}}}));
            if (++reads == 1)
                return json(
                    QJsonObject{{"code", "10001"}, {"data", QJsonArray{QJsonObject{{"code_match", "876543"}}}}});
            return json(nest(QJsonArray{QJsonObject{{"code_match", "abcdef"}}, QJsonObject{{"code_match", "987654"}}}));
        };
        MailboxRequest request;
        request.provider = MailboxKind::MailNest;
        request.configuration = {{"apiKey", "key"}, {"projectCode", "project"}};
        auto session = factory.create(request);
        QCOMPARE(session->open(), QString("paid@test"));
        QVERIFY_EXCEPTION_THROWN(session->poll(), Error);
        QCOMPARE(session->poll(), QString("987654"));
    }
    void iCloudFailedDetailsAreRetriedAndOldMessagesIgnored() {
        FakeFactory transports;
        HttpMailboxFactory factory(transports);
        int lists = 0, details = 0;
        transports.script->send = [&](const HttpRequest &request) -> HttpResponse {
            if (request.url.path().contains("/messages/")) {
                const auto html = ++lists == 1 ? "<i data-id='old'>" : "<i data-id='old'><i data-id='new'>";
                return {200, html, {}};
            }
            if (!request.url.path().contains("/message/new/"))
                throw Error(ErrorCode::Protocol, "Fetched historical message");
            if (++details == 1)
                return {503, {}, {}};
            return json(
                QJsonObject{{"body", "data:text/html;base64," +
                                         QString::fromLatin1(QByteArray("<style>#555555</style>987654").toBase64())}});
        };
        MailboxRequest request;
        request.provider = MailboxKind::ICloud;
        request.account.email = "icloud@test";
        request.account.messagesUrl = "https://icloud.test/messages/key";
        auto session = factory.create(request);
        QCOMPARE(session->open(), QString("icloud@test"));
        QVERIFY_EXCEPTION_THROWN(session->poll(), Error);
        QCOMPARE(session->poll(), QString("987654"));
        QCOMPARE(details, 2);
    }
    void graphRefreshTokenEncodingAndJunkFolderPolling() {
        FakeFactory transports;
        HttpMailboxFactory factory(transports);
        int inbox = 0, junk = 0, tokens = 0;
        bool correctForm = false;
        transports.script->send = [&](const HttpRequest &request) {
            const auto path = request.url.path();
            if (path.endsWith("/token")) {
                ++tokens;
                correctForm = request.body.contains("refresh_token=a%2Bb%26c");
                return json(QJsonObject{{"access_token", "token"}});
            }
            if (path.endsWith("/inbox")) {
                ++inbox;
                return json(QJsonObject{{"totalItemCount", 4}});
            }
            if (path.endsWith("/junkemail")) {
                ++junk;
                return json(QJsonObject{{"totalItemCount", junk == 1 ? 2 : 3}});
            }
            return json(QJsonObject{{"value", QJsonArray{QJsonObject{{"bodyPreview", "new code 786543"}}}}});
        };
        MailboxRequest request;
        request.provider = MailboxKind::Outlook;
        request.account.email = "outlook@test";
        request.account.mode = "graph";
        request.account.clientId = "client";
        request.account.refreshToken = "a+b&c";
        auto session = factory.create(request);
        QCOMPARE(session->open(), QString("outlook@test"));
        QCOMPARE(session->poll(), QString("786543"));
        QVERIFY(correctForm);
        QCOMPARE(inbox, 2);
        QCOMPARE(tokens, 1);
    }
    void otpPollingRetriesAndCancellationInterruptsWait() {
        class Poller final : public IMailboxSession {
          public:
            std::atomic<int> calls{0};
            bool returnCode = false;
            QString open(std::stop_token) override {
                return "mail@test";
            }
            QString poll(std::stop_token, std::chrono::milliseconds) override {
                const auto count = ++calls;
                if (count == 1)
                    throw Error(ErrorCode::Network, "Transient failure");
                return returnCode ? "876543" : QString{};
            }
        } session;
        session.returnCode = true;
        int retries = 0;
        QCOMPARE(waitForVerificationCode(session, std::chrono::seconds(1), std::chrono::milliseconds(1), {},
                                         [&](const QString &) { ++retries; }),
                 QString("876543"));
        QCOMPARE(retries, 1);
        session.returnCode = false;
        session.calls = 0;
        std::atomic<bool> cancelled{false};
        std::jthread worker([&](std::stop_token stop) {
            try {
                (void)waitForVerificationCode(session, std::chrono::seconds(20), std::chrono::seconds(10), stop);
            } catch (const Error &error) {
                cancelled = error.code() == ErrorCode::Cancelled;
            }
        });
        QTRY_VERIFY_WITH_TIMEOUT(session.calls.load() > 0, 1000);
        worker.request_stop();
        worker.join();
        QVERIFY(cancelled);
        QVERIFY_EXCEPTION_THROWN(
            waitForVerificationCode(session, std::chrono::milliseconds(40), std::chrono::milliseconds(10)), Error);
    }
    void imapUidBaselineSurvivesDeletedMessagesAndPollsJunk() {
        struct State {
            int inboxCalls = 0, junkCalls = 0, fetched = 0;
            bool validityChanged = false;
        };
        class Connection final : public IImapConnection {
          public:
            explicit Connection(std::shared_ptr<State> state) : state_(std::move(state)) {}
            QByteArray command(const QString &folder, const QByteArray &command, std::stop_token,
                               std::chrono::milliseconds) override {
                if (command.startsWith("LIST"))
                    return "* LIST (\\Junk) \"/\" \"Junk Email\"\r\n";
                if (command.startsWith("STATUS")) {
                    const bool junk = command.contains("Junk Email");
                    const auto count = junk ? ++state_->junkCalls : ++state_->inboxCalls;
                    const auto uid = junk ? (count == 1 ? 5 : 6) : 20;
                    return "* STATUS \"folder\" (UIDNEXT " + QByteArray::number(uid) + " UIDVALIDITY " +
                           (state_->validityChanged ? "8" : "7") + ")\r\n";
                }
                if (folder != "Junk Email")
                    throw Error(ErrorCode::Protocol, "Unexpected folder search");
                return "* SEARCH 4 5\r\n";
            }
            QByteArray message(const QString &folder, quint32 uid, std::stop_token,
                               std::chrono::milliseconds) override {
                if (folder != "Junk Email" || uid != 5)
                    throw Error(ErrorCode::Protocol, "Historical UID was fetched");
                ++state_->fetched;
                return "Content-Type: multipart/alternative; boundary=boundary\r\n\r\n"
                       "--boundary\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: "
                       "quoted-printable\r\n\r\nCode: =38=37=36=35=34=33\r\n--boundary--\r\n";
            }

          private:
            std::shared_ptr<State> state_;
        };
        class Imap final : public IImapFactory {
          public:
            std::shared_ptr<State> state = std::make_shared<State>();
            std::unique_ptr<IImapConnection> create(const QString &, const QString &,
                                                    const TransportOptions &) const override {
                return std::make_unique<Connection>(state);
            }
        } imap;
        FakeFactory transports;
        transports.script->send = [](const HttpRequest &) { return json(QJsonObject{{"access_token", "bearer"}}); };
        HttpMailboxFactory factory(transports, {}, &imap);
        MailboxRequest request;
        request.provider = MailboxKind::Outlook;
        request.account.email = "imap@test";
        request.account.mode = "imap";
        request.account.clientId = "client";
        request.account.refreshToken = "token";
        auto session = factory.create(request);
        QCOMPARE(session->open(), QString("imap@test"));
        QCOMPARE(session->poll(), QString("876543"));
        QCOMPARE(imap.state->fetched, 1);
        imap.state->validityChanged = true;
        QVERIFY_EXCEPTION_THROWN(session->poll(), Error);
    }
    void mimeMultipartDecodesTextAndSkipsAttachments() {
        const auto message =
            QByteArray("Subject: =?UTF-8?B?VmVyaWZpY2F0aW9u?=\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n"
                       "--x\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment\r\n\r\nwrong "
                       "654321\r\n"
                       "--x\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n") +
            QByteArray("<style>#555555</style><p>876543</p>").toBase64() + "\r\n--x--\r\n";
        const auto text = mailMessageText(message);
        QVERIFY(text.contains("Verification"));
        QVERIFY(!text.contains("654321"));
        QCOMPARE(extractVerificationCode(text), QString("876543"));
    }
};
QTEST_GUILESS_MAIN(MailboxTests)
#include "mailbox_tests.moc"
