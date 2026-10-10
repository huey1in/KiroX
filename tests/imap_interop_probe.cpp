#include "kirox/domain/error.hpp"
#include "kirox/infrastructure/imap_mailbox.hpp"
#include <QCoreApplication>
#include <QElapsedTimer>
#include <QJsonDocument>
#include <QJsonObject>
#include <cstdio>
#include <thread>

int main(int argc, char **argv) {
    QCoreApplication app(argc, argv);
    if (argc != 3)
        return 2;
    try {
        kirox::NativeImapFactory factory(QString::fromLocal8Bit(argv[1]), QString::fromLocal8Bit(argv[2]));
        auto connection = factory.create("fixture@example.test", "synthetic-token", {});
        const auto status =
            connection->command("INBOX", "STATUS INBOX (UIDNEXT UIDVALIDITY)", {}, std::chrono::seconds(3));
        const auto search = connection->command("Junk", "UID SEARCH UID 42:*", {}, std::chrono::seconds(3));
        const auto message = connection->message("Junk", 42, {}, std::chrono::seconds(3));
        std::stop_source cancellation;
        std::jthread cancel([&] {
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
            cancellation.request_stop();
        });
        bool cancelled = false, timedOut = false;
        QElapsedTimer elapsed;
        elapsed.start();
        try {
            (void)connection->command("INBOX", "NOOP", cancellation.get_token(), std::chrono::seconds(5));
        } catch (const kirox::Error &error) {
            cancelled = error.code() == kirox::ErrorCode::Cancelled;
        }
        const auto cancellationMs = elapsed.elapsed();
        connection = factory.create("fixture@example.test", "synthetic-token", {});
        try {
            (void)connection->command("INBOX", "NOOP", {}, std::chrono::milliseconds(200));
        } catch (const kirox::Error &error) {
            timedOut = error.code() == kirox::ErrorCode::Timeout;
        }
        const auto result = QJsonDocument(QJsonObject{{"status", QString::fromUtf8(status)},
                                                      {"search", QString::fromUtf8(search)},
                                                      {"message", QString::fromUtf8(message)},
                                                      {"cancelled", cancelled},
                                                      {"timedOut", timedOut},
                                                      {"cancellationMs", cancellationMs}})
                                .toJson(QJsonDocument::Compact);
        std::fwrite(result.constData(), 1, static_cast<size_t>(result.size()), stdout);
        return 0;
    } catch (const std::exception &error) {
        std::fprintf(stderr, "%s\n", error.what());
        return 1;
    }
}
