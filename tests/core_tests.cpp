#include "kirox/application/account_service.hpp"
#include "kirox/application/proxy_service.hpp"
#include "kirox/application/settings_service.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/error.hpp"
#include "kirox/infrastructure/json_repository.hpp"
#include "kirox/infrastructure/qt_transport.hpp"
#include <QFile>
#include <QJsonArray>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <thread>

using namespace kirox;
class CoreTests : public QObject {
    Q_OBJECT
  private slots:
    void legacyStorageConfigMigratesWithoutRemovingSources() {
        QTemporaryDir home, legacy;
        const auto put = [](const QString &path, const QByteArray &bytes) {
            QFile file(path);
            if (!file.open(QIODevice::WriteOnly))
                return false;
            return file.write(bytes) == bytes.size();
        };
        QVERIFY(put(legacy.filePath("storage.conf"),
                    "language=ja\nproxy=http://proxy.test:80\nresult_output_dir=C:/results\n"));
        const QByteArray accounts = "[{\"email\":\"legacy@test\",\"clientId\":\"client\",\"refreshToken\":\"token\"}]";
        QVERIFY(put(legacy.filePath("accounts.dat"), accounts));
        QVERIFY(put(legacy.filePath("identities.dat"), "{}"));
        JsonRepository repository(home.path(), legacy.path());
        QCOMPARE(repository.read(Document::Accounts).array().size(), qsizetype(1));
        QCOMPARE(SettingsService(repository).get().language, QString("ja"));
        QVERIFY(QFileInfo::exists(legacy.filePath("accounts.dat")));
        QVERIFY(QFileInfo::exists(home.filePath("cache/identities.json")));
        // Once settings exist, a legacy change cannot replace migrated data.
        QVERIFY(put(legacy.filePath("accounts.dat"), "[]"));
        JsonRepository restarted(home.path(), legacy.path());
        QCOMPARE(restarted.read(Document::Accounts).array().size(), qsizetype(1));
    }
    void directoryResetRefreshesOnlyUnmodifiedRollbackCopies() {
        QTemporaryDir home, destination;
        JsonRepository repository(home.path());
        AccountService accounts(repository);
        (void)accounts.import(MailboxKind::ICloud, "first@test----https://mail.test/messages/key");
        repository.relocateData(destination.path());
        (void)accounts.import(MailboxKind::ICloud, "second@test----https://mail.test/messages/key");
        repository.relocateData({});
        QCOMPARE(accounts.list(MailboxKind::ICloud).size(), std::size_t(2));
        repository.relocateData(destination.path());
        QFile independentlyChanged(home.filePath("data/accounts.json"));
        QVERIFY(independentlyChanged.open(QIODevice::WriteOnly));
        independentlyChanged.write("[]");
        independentlyChanged.close();
        QVERIFY_THROWS_EXCEPTION(Error, (void)(repository.relocateData({})));
        QCOMPARE(repository.paths().data, destination.path());
    }
    void settingsPatchesPreserveIndependentChanges() {
        QTemporaryDir dir;
        JsonRepository repository(dir.path());
        SettingsService service(repository);
        (void)service.patch({{"soundVolume", 25}, {"extensionField", "retained"}});
        (void)service.patch({{"theme", "dark"}});
        QCOMPARE(service.get().soundVolume, 25);
        QCOMPARE(service.get().theme, QString("dark"));
        QCOMPARE(repository.read(Document::Settings).object()["runtime"].toObject()["extensionField"].toString(),
                 QString("retained"));
    }
    void settingsValidationAndProxyPolicy() {
        auto settings = Settings::fromJson(
            {{"otpTimeoutSeconds", -1}, {"soundVolume", 200}, {"theme", "bad"}, {"language", "unknown"}});
        QCOMPARE(settings.otpTimeoutSeconds, 120);
        QCOMPARE(settings.soundVolume, 70);
        QCOMPARE(settings.theme, QString("system"));
        QVERIFY(settings.language.isEmpty());
        QCOMPARE(settings.mailboxProxy("http://example:80"), QString("http://example:80"));
        settings.emailProxyMode = "direct";
        QVERIFY(settings.mailboxProxy("http://example:80").isEmpty());
        settings.emailProxyMode = "custom";
        settings.emailProxy = "http://other:80";
        QCOMPARE(settings.mailboxProxy("http://example:80"), QString("http://other:80"));
    }
    void mixedProvidersAreIsolated() {
        QTemporaryDir dir;
        JsonRepository repository(dir.path());
        AccountService service(repository);
        QCOMPARE(service.import(MailboxKind::Outlook, "same@example.com----password----client----token----graph").added,
                 1);
        QCOMPARE(service.import(MailboxKind::ICloud, "same@example.com----https://mail.example/messages/key").added, 1);
        QCOMPARE(service.import(MailboxKind::Outlook, "same@example.com----password----client----token").duplicates, 1);
        service.mark(MailboxKind::Outlook, "same@example.com", true);
        QVERIFY(service.list(MailboxKind::Outlook).front().registered);
        QVERIFY(!service.list(MailboxKind::ICloud).front().registered);
        QCOMPARE(service.clear(MailboxKind::Outlook, true), 1);
        QCOMPARE(service.list(MailboxKind::ICloud).size(), std::size_t{1});
        service.remove(MailboxKind::ICloud, "same@example.com");
        QVERIFY(repository.read(Document::Accounts).array().isEmpty());
    }
    void legacyOutlookAndUnknownFieldsSurvive() {
        QTemporaryDir dir;
        JsonRepository repository(dir.path());
        AccountService service(repository);
        repository.update(Document::Accounts, [](QJsonDocument &d) {
            d.setArray(QJsonArray{QJsonObject{{"email", "legacy@example.com"},
                                              {"clientId", "client"},
                                              {"refreshToken", "token"},
                                              {"extension", 42}}});
        });
        QCOMPARE(service.list(MailboxKind::Outlook).size(), std::size_t{1});
        service.mark(MailboxKind::Outlook, "legacy@example.com", false);
        QCOMPARE(repository.read(Document::Accounts).array()[0].toObject()["extension"].toInt(), 42);
    }
    void malformedStorageIsPreserved() {
        QTemporaryDir dir;
        JsonRepository repository(dir.path());
        const auto path = dir.filePath("data/accounts.json");
        QFile file(path);
        QVERIFY(file.open(QIODevice::WriteOnly));
        file.write("malformed");
        file.close();
        QVERIFY_THROWS_EXCEPTION(Error, (void)(repository.read(Document::Accounts)));
        QVERIFY(file.open(QIODevice::ReadOnly));
        QCOMPARE(file.readAll(), QByteArray("malformed"));
    }
    void relocationCommitsAllDataAndRejectsConflicts() {
        QTemporaryDir dir, destination, conflict;
        JsonRepository repository(dir.path());
        AccountService service(repository);
        (void)service.import(MailboxKind::Outlook, "one@example.com----password----client----token");
        repository.relocateData(destination.path());
        QCOMPARE(repository.paths().data, destination.path());
        QCOMPARE(service.list(MailboxKind::Outlook).size(), std::size_t{1});
        QFile conflictFile(conflict.filePath("accounts.json"));
        QVERIFY(conflictFile.open(QIODevice::WriteOnly));
        conflictFile.write("[]");
        conflictFile.close();
        QVERIFY_THROWS_EXCEPTION(Error, (void)(repository.relocateData(conflict.path())));
        QCOMPARE(repository.paths().data, destination.path());
        QVERIFY(QFile::exists(dir.filePath("data/accounts.json")));
    }
    void concurrentSettingsAndAccountWritesAreNotLost() {
        QTemporaryDir dir;
        JsonRepository repository(dir.path());
        std::vector<std::jthread> workers;
        for (int i = 0; i < 12; ++i)
            workers.emplace_back([&, i] {
                SettingsService settings(repository);
                AccountService accounts(repository);
                for (int j = 0; j < 5; ++j) {
                    (void)settings.patch({{QString("extension%1").arg(i), j}});
                    (void)accounts.import(
                        MailboxKind::Outlook,
                        QString("user%1-%2@example.com----password----client----token").arg(i).arg(j));
                }
            });
        workers.clear();
        QCOMPARE(repository.read(Document::Accounts).array().size(), 60);
        const auto runtime = repository.read(Document::Settings).object()["runtime"].toObject();
        for (int i = 0; i < 12; ++i)
            QCOMPARE(runtime[QString("extension%1").arg(i)].toInt(), 4);
    }
    void resultExportOnlyKeepsSuccessAndReplacesSameEmail() {
        QTemporaryDir dir, output;
        JsonRepository repository(dir.path());
        repository.setResultsDirectory(output.path());
        repository.saveResult({{"status", "failed"}, {"email", "same@example.com"}});
        QVERIFY(!QFile::exists(output.filePath("accounts.json")));
        for (const auto &token : {"first", "second"})
            repository.saveResult({{"status", "success"},
                                   {"email", "same@example.com"},
                                   {"aws_token", QJsonObject{{"refreshToken", token}}}});
        QFile file(output.filePath("accounts.json"));
        QVERIFY(file.open(QIODevice::ReadOnly));
        const auto values = QJsonDocument::fromJson(file.readAll()).array();
        QCOMPARE(values.size(), 1);
        QCOMPARE(values[0].toObject()["refreshToken"].toString(), QString("second"));
    }
    void proxyNormalization_data() {
        QTest::addColumn<QString>("input");
        QTest::addColumn<QString>("expected");
        QTest::newRow("plain") << QString("host:8080") << QString("http://host:8080");
        QTest::newRow("credentials") << QString("host:8080:user:password") << QString("http://user:password@host:8080");
        QTest::newRow("socks") << QString("socks5://host") << QString("socks5://host:1080");
        QTest::newRow("ipv6") << QString("http://[::1]:3128") << QString("http://[::1]:3128");
        QTest::newRow("tls") << QString("https://host") << QString("https://host:443");
    }
    void proxyNormalization() {
        QFETCH(QString, input);
        QFETCH(QString, expected);
        QCOMPARE(normalizeProxy(input), expected);
    }
    void invalidProxiesAreRejected() {
        for (const auto &input : {"ftp://host", "http://", "host:70000", "http://host/path", "http://host?secret=1"})
            QVERIFY_THROWS_EXCEPTION(Error, (void)(normalizeProxy(input)));
        QCOMPARE(redactProxy("http://secret:password@host:80"), QString("http://host:80"));
    }
    void messageParsingAvoidsCssAndDecodesBody() {
        QCOMPARE(extractVerificationCode("<style>color:#123456</style><p>Your code is 654321</p>"), QString("654321"));
        QVERIFY(extractVerificationCode("1234567").isEmpty());
        QCOMPARE(extractVerificationCode("&#56;&#55;&#54;&#53;&#52;&#51;"), QString("876543"));
        QCOMPARE(extractVerificationCode("000000 old, current 876543", true), QString("876543"));
        QCOMPARE(decodeMessageBody("data:text/html;base64,PGI+NjU0MzIxPC9iPg=="), QString("<b>654321</b>"));
        QCOMPARE(iCloudMessageIds("<a data-id='a'></a><a DATA-ID=\"b\"></a><a data-id='a'></a>"),
                 QStringList({"a", "b"}));
        QCOMPARE(iCloudDetailUrl("https://mail.example/messages/key?x=1", "a/b"),
                 QString("https://mail.example/message/a%2Fb/key?x=1"));
    }
    void transportReadsResponseAndCookies() {
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            auto *socket = server.nextPendingConnection();
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            connect(socket, &QTcpSocket::readyRead, socket, [socket] {
                socket->readAll();
                socket->write("HTTP/1.1 200 OK\r\nContent-Length: 11\r\nSet-Cookie: session=value; "
                              "Path=/\r\nConnection: close\r\n\r\n{\"ok\":true}");
                socket->disconnectFromHost();
            });
        });
        QtTransport transport({});
        HttpRequest request;
        request.url = QUrl(QString("http://127.0.0.1:%1/").arg(server.serverPort()));
        const auto response = transport.send(request);
        QCOMPARE(response.status, 200);
        QVERIFY(response.json().object()["ok"].toBool());
        QCOMPARE(transport.cookie(request.url, "session"), QByteArray("value"));
    }
    void cancellationInterruptsStalledNetwork() {
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        QtTransport transport({});
        HttpRequest request;
        request.url = QUrl(QString("http://127.0.0.1:%1/").arg(server.serverPort()));
        std::stop_source stop;
        std::jthread worker([&] {
            std::this_thread::sleep_for(std::chrono::milliseconds{80});
            stop.request_stop();
        });
        try {
            (void)transport.send(request, stop.get_token());
            QFAIL("Request did not cancel");
        } catch (const Error &error) {
            QVERIFY(error.code() == ErrorCode::Cancelled);
        }
    }
    void timeoutInterruptsStalledNetwork() {
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        QtTransport transport({});
        HttpRequest request;
        request.url = QUrl(QString("http://127.0.0.1:%1/").arg(server.serverPort()));
        request.timeout = std::chrono::milliseconds{80};
        try {
            (void)transport.send(request);
            QFAIL("Request did not time out");
        } catch (const Error &error) {
            QVERIFY(error.code() == ErrorCode::Timeout);
        }
    }
};
QTEST_GUILESS_MAIN(CoreTests)
#include "core_tests.moc"
