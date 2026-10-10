#include "../src/desktop/controller.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/infrastructure/activity_log.hpp"
#include "kirox/infrastructure/json_repository.hpp"
#include <QApplication>
#include <QDir>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>
#include <atomic>

using namespace kirox;
namespace {
class Transport final : public ITransport {
  public:
    HttpResponse send(const HttpRequest &, std::stop_token stop) override {
        checkCancelled(stop);
        return {200, R"({"tag_name":"v2.1.0","html_url":"https://github.com/huey1in/KiroX/releases/tag/v2.1.0"})", {}};
    }
    void setCookie(const QUrl &, const QByteArray &, const QByteArray &) override {}
    QByteArray cookie(const QUrl &, const QByteArray &) const override {
        return {};
    }
};
class Transports final : public ITransportFactory {
  public:
    std::unique_ptr<ITransport> create(const TransportOptions &) const override {
        return std::make_unique<Transport>();
    }
};
class Mailboxes final : public IMailboxFactory {
  public:
    std::unique_ptr<IMailboxSession> create(const MailboxRequest &) const override {
        return {};
    }
    QJsonObject inspect(MailboxKind, const QJsonObject &, const TransportOptions &, std::stop_token) const override {
        return {};
    }
};
class Registration final : public IRegistrationService {
  public:
    mutable std::atomic<bool> entered{false};
    std::atomic<bool> release{false};
    RegistrationResult run(const RegistrationRequest &request, std::stop_token stop,
                           const RegistrationProgress &progress) const override {
        entered = true;
        progress("GetOTP");
        while (!release)
            interruptibleWait(stop, std::chrono::milliseconds(2));
        return {{{"status", "success"},
                 {"passwordSet", true},
                 {"email", request.mailbox.account.email},
                 {"password", "synthetic-password"},
                 {"aws_token", QJsonObject{{"refreshToken", "synthetic-refresh"}}}}};
    }
};
void prepare(JsonRepository &repository) {
    repository.setResultsDirectory(QDir(repository.paths().root).filePath("results"));
    (void)SettingsService(repository)
        .patch({{"language", "zh"}, {"autoCheckUpdates", false}, {"autoProbeProxies", false}});
}
} // namespace
class DesktopTests final : public QObject {
    Q_OBJECT
  private slots:
    void modelsKeepSecretsOutAndEditingPreservesCredentials() {
        QTemporaryDir root;
        JsonRepository repository(root.path());
        prepare(repository);
        Transports transports;
        Mailboxes mailboxes;
        Registration registration;
        ActivityLog log(repository.paths().logs);
        Controller controller(repository, transports, mailboxes, registration, log);
        controller.importAccounts("outlook",
                                  "fixture@example.test----synthetic-password----client----synthetic-refresh");
        const auto row = controller.mailboxes().first().toMap();
        QVERIFY(!row.contains("password"));
        QVERIFY(!row.contains("refreshToken"));
        controller.addProxy("Residential", "http://user:synthetic@localhost:8888", 50);
        const auto proxy = controller.proxies().first().toMap();
        QVERIFY(!proxy.value("url").toString().contains("synthetic"));
        const auto id = proxy.value("id").toString();
        const auto saved = controller.proxyConfiguration(id);
        QCOMPARE(saved.value("url").toString(), QString("http://user:synthetic@localhost:8888"));
        QVERIFY(controller.saveProxy(id, "Edited", saved.value("url").toString(), 20));
        QCOMPARE(controller.proxyConfiguration(id).value("weight").toInt(), 20);
        QSignalSpy errors(&controller, &Controller::operationFailed);
        QVERIFY(!controller.saveProxy(id, "Edited", "http://", 20));
        QCOMPARE(errors.count(), 1);
        QVERIFY(!errors.first().first().toString().contains("synthetic"));
        QCOMPARE(controller.proxyConfiguration(id).value("url"), saved.value("url"));
    }
    void queuedBatchUpdatesPersistAndProtectRunningMailboxes() {
        QTemporaryDir root;
        JsonRepository repository(root.path());
        prepare(repository);
        Transports transports;
        Mailboxes mailboxes;
        Registration registration;
        ActivityLog log(repository.paths().logs);
        Controller controller(repository, transports, mailboxes, registration, log);
        controller.importAccounts("icloud", "fixture@example.test----https://mail.example.test/messages/key");
        QSignalSpy finished(&controller, &Controller::batchFinished), errors(&controller, &Controller::operationFailed);
        QVERIFY(controller.startBatch({{"provider", "icloud"}, {"count", 1}, {"delaySeconds", 0}}));
        QTRY_VERIFY(registration.entered.load());
        controller.clearAccounts("icloud", false);
        QCOMPARE(controller.mailboxes().size(), 1);
        QCOMPARE(errors.count(), 1);
        QCOMPARE(errors.first().first().toString(), QString::fromUtf8("请先停止任务再清空邮箱"));
        registration.release = true;
        QTRY_COMPARE(finished.count(), 1);
        QCOMPARE(controller.batchStatus().value("success").toInt(), 1);
        QVERIFY(controller.mailboxes().first().toMap().value("registered").toBool());
        QVERIFY(!controller.activity().isEmpty());
        const auto activity = QJsonDocument(QJsonArray::fromVariantList(controller.activity())).toJson();
        QVERIFY(!activity.contains("synthetic"));
        QVERIFY(!activity.contains("fixture@example.test"));
        controller.clearActivity();
        QVERIFY(controller.activity().isEmpty());
    }
    void updateCheckUsesItsOwnCompletionState() {
        QTemporaryDir root;
        JsonRepository repository(root.path());
        prepare(repository);
        Transports transports;
        Mailboxes mailboxes;
        Registration registration;
        ActivityLog log(repository.paths().logs);
        Controller controller(repository, transports, mailboxes, registration, log);
        controller.checkUpdates();
        QVERIFY(controller.updateChecking());
        QVERIFY(!controller.busy());
        QTRY_VERIFY(!controller.updateChecking());
        QVERIFY(controller.updateInfo().value("hasUpdate").toBool());
        QCOMPARE(controller.updateInfo().value("latestVersion").toString(), QString("v2.1.0"));
    }
};
int main(int argc, char **argv) {
    QApplication app(argc, argv);
    app.setApplicationVersion("2.0.0");
    DesktopTests tests;
    return QTest::qExec(&tests, argc, argv);
}
#include "desktop_tests.moc"
