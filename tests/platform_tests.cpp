#include "../src/desktop/desktop_instance.hpp"
#include "kirox/application/update_service.hpp"
#include "kirox/domain/error.hpp"
#include "kirox/domain/version.hpp"
#include "kirox/infrastructure/activity_log.hpp"
#include <QDir>
#include <QFile>
#include <QJsonDocument>
#include <QProcess>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

using namespace kirox;
namespace {
class Transport final : public ITransport {
  public:
    explicit Transport(HttpResponse response) : response(std::move(response)) {}
    HttpResponse send(const HttpRequest &, std::stop_token) override {
        return response;
    }
    void setCookie(const QUrl &, const QByteArray &, const QByteArray &) override {}
    QByteArray cookie(const QUrl &, const QByteArray &) const override {
        return {};
    }
    HttpResponse response;
};
class Factory final : public ITransportFactory {
  public:
    HttpResponse response;
    std::unique_ptr<ITransport> create(const TransportOptions &) const override {
        return std::make_unique<Transport>(response);
    }
};
} // namespace
class PlatformTests : public QObject {
    Q_OBJECT
  private slots:
    void semanticVersionsHonorPrereleaseOrdering() {
        const QStringList versions{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta",
                                   "1.0.0-beta",  "1.0.0-beta.2",  "1.0.0-beta.11",
                                   "1.0.0-rc.1",  "1.0.0",         "1.0.1",
                                   "1.1.0",       "2.0.0"};
        for (qsizetype i = 1; i < versions.size(); ++i) {
            QVERIFY(newerVersion(versions.at(i), versions.at(i - 1)));
            QVERIFY(!newerVersion(versions.at(i - 1), versions.at(i)));
        }
        QVERIFY(!newerVersion("v2.0.0+build.17", "2.0.0+build.18"));
        QVERIFY(newerVersion("99999999999999999999999999.0.0", "9999999999999999999999999.0.0"));
        QVERIFY_EXCEPTION_THROWN(newerVersion("1.0.0-01", "1.0.0"), Error);
        QVERIFY_EXCEPTION_THROWN(newerVersion("1.0junk", "1.0.0"), Error);
    }
    void updaterHasBoundedMetadataAndSafeReleaseLink() {
        Factory factory;
        factory.response = {200,
                            QJsonDocument(QJsonObject{{"tag_name", "v2.1.0"},
                                                      {"html_url", "https://untrusted.test/download"},
                                                      {"body", "Release notes"},
                                                      {"published_at", "2026-10-10T10:00:00Z"}})
                                .toJson(),
                            {}};
        const auto result = UpdateService(factory).check("2.0.0");
        QVERIFY(result.value("hasUpdate").toBool());
        QCOMPARE(result.value("releaseURL").toString(), QString("https://github.com/huey1in/KiroX/releases/latest"));
        factory.response = {404, {}, {}};
        QVERIFY(UpdateService(factory).check("2.0.0").value("noRelease").toBool());
        factory.response = {503, {}, {}};
        QVERIFY_EXCEPTION_THROWN(UpdateService(factory).check("2.0.0"), Error);
    }
    void persistentLogsExcludeCredentialsAndBoundMemory() {
        QTemporaryDir home;
        ActivityLog log(home.path());
        log.append({{"step", "GetOTP"},
                    {"status", "running"},
                    {"password", "secret"},
                    {"refreshToken", "token"},
                    {"email", "private@example.test"},
                    {"body", "sensitive"}},
                   true, 7);
        for (int i = 0; i < 520; ++i)
            log.append({{"index", i}, {"status", "success"}}, false, 7);
        QCOMPARE(log.entries().size(), qsizetype(500));
        const auto files = QDir(home.path()).entryList({"*.jsonl"}, QDir::Files);
        QCOMPARE(files.size(), qsizetype(1));
        QFile file(QDir(home.path()).filePath(files.first()));
        QVERIFY(file.open(QIODevice::ReadOnly));
        const auto bytes = file.readAll();
        QVERIFY(!bytes.contains("secret"));
        QVERIFY(!bytes.contains("token"));
        QVERIFY(!bytes.contains("private@"));
        ActivityLog restarted(home.path());
        QCOMPARE(restarted.entries().size(), qsizetype(1));
        QCOMPARE(restarted.entries().first().toObject().value("step").toString(), QString("GetOTP"));
        restarted.clear();
        QCOMPARE(restarted.entries().size(), qsizetype(0));
        QVERIFY(file.exists());
    }
    void retentionDeletesOnlyOwnedExpiredLogs() {
        QTemporaryDir home;
        const auto old = QString("kirox-native-%1.jsonl").arg(QDate::currentDate().addDays(-20).toString(Qt::ISODate));
        for (const auto &name : QStringList{old, "unrelated.jsonl"}) {
            QFile file(home.filePath(name));
            QVERIFY(file.open(QIODevice::WriteOnly));
            file.write("{}\n");
        }
        ActivityLog log(home.path());
        log.append({{"status", "success"}}, true, 7);
        QVERIFY(!QFile::exists(home.filePath(old)));
        QVERIFY(QFile::exists(home.filePath("unrelated.jsonl")));
    }
    void secondInstanceActivatesFirstAndRootsRemainIsolated() {
        QTemporaryDir firstRoot, secondRoot;
        DesktopInstance first(firstRoot.path());
        QVERIFY(first.primary());
        QSignalSpy activated(&first, &DesktopInstance::activationRequested);
        QProcess second;
        second.start(QCoreApplication::applicationFilePath(), {"--activate", firstRoot.path()});
        QTRY_COMPARE_WITH_TIMEOUT(activated.size(), 1, 3000);
        QTRY_COMPARE_WITH_TIMEOUT(second.state(), QProcess::NotRunning, 3000);
        QCOMPARE(second.exitCode(), 0);
        DesktopInstance isolated(secondRoot.path());
        QVERIFY(isolated.primary());
    }
};
int main(int argc, char *argv[]) {
    QCoreApplication app(argc, argv);
    if (app.arguments().size() == 3 && app.arguments().at(1) == "--activate") {
        DesktopInstance instance(app.arguments().at(2));
        return instance.primary() ? 2 : 0;
    }
    PlatformTests tests;
    return QTest::qExec(&tests, argc, argv);
}
#include "platform_tests.moc"
