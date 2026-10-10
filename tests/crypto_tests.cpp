#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/fingerprint_crypto.hpp"
#include "kirox/infrastructure/app_script_cache.hpp"
#include "kirox/infrastructure/native_crypto.hpp"
#include <QTest>
#include <atomic>
#include <thread>

using namespace kirox;
namespace {
class DelayedTransport final : public ITransport {
  public:
    DelayedTransport(std::atomic<bool> &release, std::atomic<int> &calls) : release_(release), calls_(calls) {}
    HttpResponse send(const HttpRequest &, std::stop_token stop) override {
        ++calls_;
        while (!release_)
            interruptibleWait(stop, std::chrono::milliseconds(5));
        return {
            200,
            "var "
            "a=[2,1,3,\"NewKey\",4];return{identifier:a[3],material:[a[1],a[0],a[2],a[4]]};x.FWCIM_VERSION=\"5.1.2\"",
            {}};
    }
    void setCookie(const QUrl &, const QByteArray &, const QByteArray &) override {}
    QByteArray cookie(const QUrl &, const QByteArray &) const override {
        return {};
    }

  private:
    std::atomic<bool> &release_;
    std::atomic<int> &calls_;
};
class DelayedFactory final : public ITransportFactory {
  public:
    mutable std::atomic<int> calls{0};
    mutable std::atomic<bool> release{false};
    std::unique_ptr<ITransport> create(const TransportOptions &) const override {
        return std::make_unique<DelayedTransport>(release, calls);
    }
};
} // namespace
class CryptoTests : public QObject {
    Q_OBJECT
  private slots:
    void crcAndXxteaMatchLegacyGoFixtures() {
        QCOMPARE(crc32("123456789"), 0xcbf43926U);
        const FingerprintCryptoConfig config;
        QCOMPARE(encryptFingerprint("{}", config), QString("ECdITeCs:pHPTX0YPfTU1Op7h"));
        QCOMPARE(encryptFingerprint("{\"event\":\"PageLoad\",\"value\":42}", config),
                 QString("ECdITeCs:Ec2jReK3FyciisWaZnYgvlmJh/peTgMHxgvQx9AG+0lIm9qq62N54g=="));
        QCOMPARE(encryptFingerprint(QStringLiteral("{\"message\":\"测试\"}").toUtf8(), config),
                 QString("ECdITeCs:sjMiZnt7UJihA87m6VoRVegL112yPaI0M2mhqLjqxKk="));
        QVERIFY(xxteaEncrypt({}, config.key).isEmpty());
    }
    void scriptKeyExtractionRejectsUnrelatedArrays() {
        const auto script = QString(
            "var unrelated=[10,20,30,\"Wrong\",40];var key=[2576816180,1888420705,2347232058,\"ECdITeCs\",874813317];"
            "return{identifier:key[3],material:[key[1],key[0],key[2],key[4]]};module[\"FWCIM_VERSION\"]=\"4.5.6\";");
        const auto config = fingerprintCryptoFromScript(script);
        QCOMPARE(config.key, FingerprintCryptoConfig{}.key);
        QCOMPARE(config.identifier, QString("ECdITeCs"));
        QCOMPARE(config.version, QString("4.5.6"));
        const auto unrelated = fingerprintCryptoFromScript("var data=[1,2,3,\"Wrong\",4];");
        QCOMPARE(unrelated.key, FingerprintCryptoConfig{}.key);
        QCOMPARE(unrelated.identifier, QString("ECdITeCs"));
    }
    void aes256GcmMatchesKnownAnswer() {
        NativeCryptography crypto;
        const auto result = crypto.aes256Gcm(QByteArray(32, 0), QByteArray(12, 0), QByteArray(16, 0), {});
        QCOMPARE(result.ciphertext.toHex(), QByteArray("cea7403d4d606b6e074ec5d3baf39d18"));
        QCOMPARE(result.tag.toHex(), QByteArray("d0d1c8a799996bf0265b98b5d48ab919"));
        QVERIFY_EXCEPTION_THROWN(crypto.aes256Gcm(QByteArray(16, 0), QByteArray(12, 0), {}, {}), Error);
        QCOMPARE(crypto.randomBytes(32).size(), qsizetype(32));
        QVERIFY(crypto.randomBytes(32) != crypto.randomBytes(32));
    }
    void cancelledWaiterDoesNotCancelSharedCache() {
        DelayedFactory transport;
        AppScriptCache cache(transport);
        cache.warm({}, "agent");
        cache.warm({}, "another-agent");
        std::atomic<bool> cancelled{false};
        std::jthread waiter([&](std::stop_token stop) {
            try {
                cache.wait(stop);
            } catch (const Error &e) {
                cancelled = e.code() == ErrorCode::Cancelled;
            }
        });
        QTRY_COMPARE_WITH_TIMEOUT(transport.calls.load(), 1, 1000);
        waiter.request_stop();
        waiter.join();
        QVERIFY(cancelled);
        QCOMPARE(cache.snapshot()->version, QString("4.0.0"));
        transport.release = true;
        cache.wait();
        QCOMPARE(cache.snapshot()->version, QString("5.1.2"));
        QCOMPARE(cache.snapshot()->identifier, QString("NewKey"));
        QCOMPARE(transport.calls.load(), 1);
    }
};
QTEST_GUILESS_MAIN(CryptoTests)
#include "crypto_tests.moc"
