#include "kirox/application/identity_service.hpp"
#include "kirox/domain/fingerprint.hpp"
#include "kirox/domain/fingerprint_crypto.hpp"
#include "kirox/infrastructure/json_repository.hpp"
#include <QJsonArray>
#include <QTemporaryDir>
#include <QTest>
#include <thread>

using namespace kirox;
class BrowserTests : public QObject {
    Q_OBJECT
  private slots:
    void generatedSurfaceHasConsistentHardware() {
        QSet<QString> versions;
        for (int i = 0; i < 400; ++i) {
            const auto identity = BrowserIdentity::generate();
            QVERIFY(identity.valid());
            versions.insert(identity.majorVersion());
            const auto &data = identity.json();
            const auto screen = data.value("Screen").toObject();
            QVERIFY((
                QList<int>{32, 40, 48}.contains(screen.value("Height").toInt() - screen.value("AvailHeight").toInt())));
            const auto ext = data.value("WebGLExts").toArray();
            QVERIFY(ext.size() >= 26 && ext.size() <= 30);
            const auto refreshed = identity.newSession();
            for (const auto key : {"UA", "SecUA", "GPUModel", "Screen", "HistogramBase", "CanvasHash", "Plugins",
                                   "DeviceMemory", "HardwareConcurrency", "TimezoneHours"})
                QCOMPARE(refreshed.json().value(key), data.value(key));
            QVERIFY(refreshed.newSession(true).valid());
        }
        QCOMPARE(versions.size(), qsizetype(3));
    }
    void proxyKeyNeverPersistsCredentials() {
        QCOMPARE(browserIdentityKey(""), QString("direct"));
        QCOMPARE(browserIdentityKey("http://user:password@PROXY.test:8080"), QString("proxy.test:8080"));
        QCOMPARE(browserIdentityKey("socks5://other:secret@proxy.test:8080"), QString("proxy.test:8080"));
        QCOMPARE(browserIdentityKey("https://user:secret@[2001:db8::1]:443"), QString("[2001:db8::1]:443"));
        QVERIFY(browserIdentityKey("user:secret@invalid").startsWith("raw:"));
        QVERIFY(!browserIdentityKey("user:secret@invalid").contains("secret"));
    }
    void cacheSurvivesRestartAndRefreshesAtTtl() {
        QTemporaryDir home;
        JsonRepository repository(home.path());
        IdentityService service(repository);
        const auto initial = service.forProxy("http://u:p@proxy.test:8080", false, 100000);
        auto original = repository.read(Document::Identities).object();
        const auto persisted = original.value("proxy.test:8080").toObject().value("identity").toObject();
        QCOMPARE(persisted.value("HistogramBase"), initial.json().value("HistogramBase"));
        JsonRepository restarted(home.path());
        const auto next = IdentityService(restarted).forProxy("socks5://x:y@proxy.test:8080", false, 121599);
        QCOMPARE(next.json().value("GPUModel"), initial.json().value("GPUModel"));
        QCOMPARE(next.json().value("HistogramBase"), initial.json().value("HistogramBase"));
        (void)service.forProxy("http://proxy.test:8080", false, 121600);
        QCOMPARE(repository.read(Document::Identities)
                     .object()
                     .value("proxy.test:8080")
                     .toObject()
                     .value("createdAt")
                     .toInteger(),
                 qint64(121600));
        // Old unsupported profiles and malformed histogram caches are regenerated.
        repository.update(Document::Identities, [](QJsonDocument &document) {
            auto entries = document.object(), entry = entries.value("proxy.test:8080").toObject(),
                 identity = entry.value("identity").toObject();
            identity.insert("ChromeVer", "120.0.0.0");
            entry.insert("identity", identity);
            entries.insert("proxy.test:8080", entry);
            document = QJsonDocument(entries);
        });
        QVERIFY(service.forProxy("http://proxy.test:8080", false, 121601).valid());
    }
    void concurrentCacheUpdatesRetainEveryProxy() {
        QTemporaryDir home;
        JsonRepository repository(home.path());
        IdentityService service(repository);
        std::vector<std::jthread> workers;
        for (int i = 0; i < 20; ++i)
            workers.emplace_back(
                [&, i] { (void)service.forProxy(QString("http://proxy%1.test:8080").arg(i), false, 100000); });
        workers.clear();
        QCOMPARE(repository.read(Document::Identities).object().size(), qsizetype(20));
    }
    void fingerprintMatchesPageContract_data() {
        QTest::addColumn<QString>("page");
        QTest::addColumn<QString>("event");
        QTest::addColumn<int>("history");
        QTest::addColumn<int>("clicks");
        QTest::addColumn<int>("keys");
        QTest::newRow("signin-load") << "signin" << "first_load" << 5 << 0 << 0;
        QTest::newRow("signin-submit") << "signin" << "PageSubmit" << 5 << 2 << 2;
        QTest::newRow("signin-signup") << "signin" << "SignupStart" << 5 << 2 << 2;
        QTest::newRow("profile-load") << "profile" << "PageLoad" << 6 << 0 << 0;
        QTest::newRow("profile-submit") << "profile" << "PageSubmit" << 8 << 2 << 2;
        QTest::newRow("profile-code") << "profile" << "EmailVerification" << 8 << 1 << 2;
        QTest::newRow("signup-load") << "signup" << "first_load" << 9 << 0 << 0;
        QTest::newRow("signup-submit") << "signup" << "PageSubmit" << 9 << 5 << 16;
    }
    void fingerprintMatchesPageContract() {
        QFETCH(QString, page);
        QFETCH(QString, event);
        QFETCH(int, history);
        QFETCH(int, clicks);
        QFETCH(int, keys);
        auto identity = BrowserIdentity::generate();
        FingerprintContext context(identity);
        const auto bytes = context.serialize(
            {"https://example.test/form", "https://example.test", page, event, "test@example.test", 2500}, "4.5.6",
            100000000);
        QJsonParseError error;
        const auto fp = QJsonDocument::fromJson(bytes, &error).object();
        QCOMPARE(error.error, QJsonParseError::NoError);
        QCOMPARE(fp.value("history").toObject().value("length").toInt(), history);
        const auto actions = fp.value("interaction").toObject();
        QCOMPARE(actions.value("clicks").toInt(), clicks);
        QCOMPARE(actions.value("keyPresses").toInt(), keys);
        QCOMPARE(actions.value("mouseClickPositions").toArray().size(), qsizetype(clicks));
        QCOMPARE(fp.value("canvas").toObject().value("histogramBins"), identity.json().value("HistogramBase"));
        QCOMPARE(fp.value("canvas").toObject().value("hash"), identity.json().value("CanvasHash"));
        QCOMPARE(fp.value("version").toString(), QString("4.5.6"));
        QVERIFY(!fp.value("webDriver").toBool());
        QCOMPARE(fp.value("token").toObject().value("isCompatible").toBool(), page != "signin");
        QVERIFY(bytes.startsWith("{\"metrics\":"));
        QVERIFY(bytes.indexOf("\"scripts\":") < bytes.indexOf("\"history\":"));
        QVERIFY(bytes.indexOf("\"screenInfo\":") < bytes.indexOf("\"lsUbid\":"));
        QVERIFY(bytes.indexOf("\"canvas\":{") < bytes.indexOf("\"token\":"));
        if (page == "profile" && event != "PageLoad") {
            const auto field = fp.value("form").toObject().begin().value().toObject();
            QCOMPARE(field.value("checksum").toString(),
                     QString("%1").arg(crc32("test@example.test"), 8, 16, QChar('0')).toUpper());
            QCOMPARE(field.value("keyPresses").toInt(), keys);
        }
    }
    void pageContextKeepsHardwareAndIdsButResetsTiming() {
        FingerprintContext first(BrowserIdentity::generate()), second(first.identity());
        const auto generate = [&](const FingerprintEvent &event, qint64 now) {
            return QJsonDocument::fromJson(first.serialize(event, "4.0.0", now)).object();
        };
        const auto load = generate({{}, {}, "signin", "first_load", {}, 0}, 100000000);
        QCOMPARE(load.value("performance").toObject().value("timing").toObject().value("loadEventEnd").toInteger(),
                 qint64(0));
        const auto submit = generate({{}, {}, "signin", "PageSubmit", {}, 0}, 100002000);
        const auto timing = submit.value("performance").toObject().value("timing").toObject();
        QVERIFY(timing.value("loadEventEnd").toInteger() > 0);
        QCOMPARE(load.value("lsUbid"), submit.value("lsUbid"));
        QCOMPARE(load.value("start"), submit.value("start"));
        first.resetPage();
        const auto profile = generate({{}, {}, "profile", "PageLoad", {}, 0}, 100010000);
        const auto profileSubmit = generate({{}, {}, "profile", "PageSubmit", {}, 0}, 100012000);
        QVERIFY(profile.value("lsUbid") != load.value("lsUbid"));
        QCOMPARE(profile.value("lsUbid"), profileSubmit.value("lsUbid"));
        QVERIFY(profile.value("performance").toObject().value("timing") != timing);
        QCOMPARE(profile.value("canvas"), submit.value("canvas"));
        const auto other =
            QJsonDocument::fromJson(second.serialize({{}, {}, "signin", "first_load", {}, 0}, "4.0.0", 100000000))
                .object();
        QVERIFY(other.value("lsUbid") != load.value("lsUbid"));
    }
};
QTEST_GUILESS_MAIN(BrowserTests)
#include "browser_tests.moc"
