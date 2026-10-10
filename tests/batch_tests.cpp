#include "kirox/application/batch_service.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/error.hpp"
#include "kirox/infrastructure/json_repository.hpp"
#include <QDir>
#include <QFile>
#include <QJsonArray>
#include <QTemporaryDir>
#include <QTest>
#include <atomic>
#include <thread>

using namespace kirox;
namespace {
class Registrar final : public IRegistrationService {
  public:
    mutable std::atomic<int> calls{0}, active{0}, peak{0};
    std::atomic<bool> release{false};
    bool risk = false, alreadyRegistered = false, consumedFailure = false;
    mutable std::mutex mutex;
    mutable QStringList emails;
    RegistrationResult run(const RegistrationRequest &request, std::stop_token stop,
                           const RegistrationProgress &progress) const override {
        const auto call = calls.fetch_add(1);
        const auto concurrent = ++active;
        int previous = peak;
        while (previous < concurrent && !peak.compare_exchange_weak(previous, concurrent)) {
        }
        struct Leave {
            std::atomic<int> &value;
            ~Leave() {
                --value;
            }
        } leave{active};
        {
            std::scoped_lock lock(mutex);
            emails.append(request.mailbox.account.email);
        }
        if (progress)
            progress("GetOTP");
        while (!release)
            interruptibleWait(stop, std::chrono::milliseconds(2));
        checkCancelled(stop);
        if (risk && call == 0)
            return {{{"status", "failed"},
                     {"risk", true},
                     {"errorCode", "BLOCKED"},
                     {"email", request.mailbox.account.email}}};
        if (alreadyRegistered && call == 0)
            return {{{"status", "failed"},
                     {"errorCode", "EMAIL_ALREADY_REGISTERED"},
                     {"email", request.mailbox.account.email}}};
        if (consumedFailure)
            return {{{"status", "failed"},
                     {"passwordSet", true},
                     {"errorCode", "TIMEOUT"},
                     {"email", request.mailbox.account.email}}};
        return {{{"status", "success"},
                 {"passwordSet", true},
                 {"email", request.mailbox.account.email},
                 {"password", "synthetic-password"},
                 {"aws_token", QJsonObject{{"refreshToken", "synthetic-refresh"}}}}};
    }
};
void populate(IRepository &repository, int count) {
    repository.setResultsDirectory(QDir(repository.paths().root).filePath("results"));
    QStringList entries;
    for (int i = 0; i < count; ++i)
        entries.append(QString("account%1@example.test----https://mail.example.test/messages/key%1").arg(i));
    (void)AccountService(repository).import(MailboxKind::ICloud, entries.join('\n'));
}
BatchRequest request(int count, int concurrency = 1) {
    BatchRequest result;
    result.provider = MailboxKind::ICloud;
    result.count = count;
    result.concurrency = concurrency;
    result.delaySeconds = 0;
    return result;
}
} // namespace
class BatchTests : public QObject {
    Q_OBJECT
  private slots:
    void concurrentWorkHonorsLimitAndNeverDuplicatesMailboxes() {
        QTemporaryDir home, output;
        JsonRepository repository(home.path());
        populate(repository, 12);
        Registrar registrar;
        BatchService batch(repository, registrar);
        auto options = request(12, 3);
        options.outputDirectory = output.path();
        batch.start(options, Settings{});
        QTRY_COMPARE_WITH_TIMEOUT(registrar.active.load(), 3, 1000);
        QVERIFY_EXCEPTION_THROWN(batch.start(options, Settings{}), Error);
        registrar.release = true;
        batch.wait();
        const auto state = batch.status();
        QCOMPARE(state.value("success").toInt(), 12);
        QCOMPARE(state.value("completed").toInt(), 12);
        QCOMPARE(state.value("cancelled").toInt(), 0);
        QVERIFY(!state.value("running").toBool());
        QCOMPARE(registrar.peak.load(), 3);
        QCOMPARE(QSet<QString>(registrar.emails.begin(), registrar.emails.end()).size(), qsizetype(12));
        QCOMPARE(AccountService(repository).list(MailboxKind::ICloud, true).size(), size_t(0));
        QVERIFY(!QDir(output.path()).entryList({"*.json"}, QDir::Files).isEmpty());
        for (const auto value : batch.tasks())
            QCOMPARE(value.toObject().value("status").toString(), QString("success"));
    }
    void stopCancelsActiveAndQueuedWorkAndAllowsRestart() {
        QTemporaryDir home;
        JsonRepository repository(home.path());
        populate(repository, 20);
        Registrar registrar;
        BatchService batch(repository, registrar);
        batch.start(request(20, 3), Settings{});
        QTRY_COMPARE_WITH_TIMEOUT(registrar.active.load(), 3, 1000);
        QVERIFY(batch.stop());
        batch.wait();
        QCOMPARE(registrar.calls.load(), 3);
        QCOMPARE(batch.status().value("cancelled").toInt(), 20);
        QCOMPARE(batch.status().value("completed").toInt(), 20);
        QCOMPARE(AccountService(repository).list(MailboxKind::ICloud, true).size(), size_t(20));
        registrar.release = true;
        batch.start(request(1), Settings{});
        batch.wait();
        QCOMPARE(batch.status().value("success").toInt(), 1);
    }
    void immediateStopCannotLoseCancellation() {
        QTemporaryDir home;
        JsonRepository repository(home.path());
        populate(repository, 3);
        Registrar registrar;
        BatchService batch(repository, registrar);
        for (int i = 0; i < 20; ++i) {
            batch.start(request(3, 3), Settings{});
            batch.stop();
            batch.wait();
            QCOMPARE(batch.status().value("completed").toInt(), 3);
            QCOMPARE(batch.status().value("success").toInt(), 0);
        }
    }
    void riskStopsQueueAndRetainsTheFailure() {
        QTemporaryDir home;
        JsonRepository repository(home.path());
        populate(repository, 10);
        Registrar registrar;
        registrar.release = true;
        registrar.risk = true;
        BatchService batch(repository, registrar);
        batch.start(request(10), Settings{});
        batch.wait();
        QCOMPARE(registrar.calls.load(), 1);
        QVERIFY(batch.status().value("riskStopped").toBool());
        QCOMPARE(batch.status().value("failed").toInt(), 1);
        QCOMPARE(batch.status().value("cancelled").toInt(), 9);
    }
    void consumedMailboxIsNeverRetried() {
        QTemporaryDir home;
        JsonRepository repository(home.path());
        populate(repository, 2);
        Registrar registrar;
        registrar.release = true;
        registrar.consumedFailure = true;
        BatchService batch(repository, registrar);
        batch.start(request(1), Settings{});
        batch.wait();
        QCOMPARE(registrar.calls.load(), 1);
        QCOMPARE(AccountService(repository).list(MailboxKind::ICloud, true).size(), size_t(1));
    }
    void alreadyRegisteredMailboxIsReplacedUsingSparePool() {
        QTemporaryDir home, output;
        JsonRepository repository(home.path());
        populate(repository, 2);
        Registrar registrar;
        registrar.release = true;
        registrar.alreadyRegistered = true;
        BatchService batch(repository, registrar);
        auto options = request(1);
        options.outputDirectory = output.path();
        batch.start(options, Settings{});
        batch.wait();
        QCOMPARE(registrar.calls.load(), 2);
        QCOMPARE(batch.status().value("success").toInt(), 1);
        QCOMPARE(AccountService(repository).list(MailboxKind::ICloud, true).size(), size_t(0));
    }
    void invalidAdmissionNeverStartsWorkers() {
        QTemporaryDir home;
        JsonRepository repository(home.path());
        populate(repository, 1);
        Registrar registrar;
        BatchService batch(repository, registrar);
        QVERIFY_EXCEPTION_THROWN(batch.start(request(2), Settings{}), Error);
        auto options = request(1);
        options.proxyMode = "selected";
        options.proxyId = "removed";
        QVERIFY_EXCEPTION_THROWN(batch.start(options, Settings{}), Error);
        QCOMPARE(registrar.calls.load(), 0);
        QVERIFY(!batch.status().value("running").toBool());
    }
};
QTEST_GUILESS_MAIN(BatchTests)
#include "batch_tests.moc"
