#include "kirox/domain/error.hpp"
#include "kirox/infrastructure/curl_transport.hpp"
#include <QElapsedTimer>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTest>
#include <atomic>
#include <thread>

using namespace kirox;
class CurlTests : public QObject {
    Q_OBJECT
  private slots:
    void realHttpCookiesAndRequestMethods_data() {
        QTest::addColumn<QString>("profile");
        QTest::newRow("standard") << QString{};
        QTest::newRow("chrome131") << QString("chrome131");
        QTest::newRow("chrome133") << QString("chrome133");
        QTest::newRow("chrome144") << QString("chrome144");
    }
    void tlsClientHelloProfiles_data() {
        QTest::addColumn<QString>("profile");
        QTest::addColumn<QString>("expectedExtensions");
        QTest::newRow("131") << QString("chrome131")
                             << QString("13-65037-65281-18-27-16-5-10-17513-11-51-35-43-45-0-23");
        QTest::newRow("133") << QString("chrome133") << QString("13-17613-51-18-11-43-5-16-0-65037-27-10-45-23-65281");
        QTest::newRow("144") << QString("chrome144")
                             << QString("51-0-17613-65281-10-27-35-5-23-43-13-18-11-65037-16-45");
    }
    void tlsClientHelloProfiles() {
        QFETCH(QString, profile);
        QFETCH(QString, expectedExtensions);
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        QByteArray hello;
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            auto *socket = server.nextPendingConnection();
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            connect(socket, &QTcpSocket::readyRead, socket, [&, socket, pending = QByteArray{}]() mutable {
                pending += socket->readAll();
                if (pending.size() < 5)
                    return;
                const auto length = (static_cast<quint8>(pending[3]) << 8) | static_cast<quint8>(pending[4]);
                if (pending.size() < length + 5)
                    return;
                hello = pending.left(length + 5);
                socket->write(QByteArray::fromHex("15030300020228"));
                socket->disconnectFromHost();
            });
        });
        std::atomic<bool> done{false};
        QString unexpectedError;
        std::jthread worker([&] {
            try {
                CurlTransport session(TransportOptions{{}, {}, profile});
                HttpRequest request;
                request.url = QUrl("https://localhost:" + QString::number(server.serverPort()));
                request.timeout = std::chrono::seconds(2);
                (void)session.send(request);
            } catch (const Error &error) {
                if (error.code() != ErrorCode::Network)
                    unexpectedError = error.message();
            }
            done = true;
        });
        QTRY_VERIFY_WITH_TIMEOUT(done.load(), 4000);
        worker.join();
        QVERIFY2(unexpectedError.isEmpty(), qPrintable(unexpectedError));
        QVERIFY(hello.size() > 44);
        QCOMPARE(static_cast<quint8>(hello[0]), quint8(22));
        QCOMPARE(static_cast<quint8>(hello[5]), quint8(1));
        qsizetype position = 43;
        const auto byte = [&] {
            if (position >= hello.size())
                throw Error(ErrorCode::Protocol, "Truncated ClientHello");
            return static_cast<quint8>(hello[position++]);
        };
        const auto number = [&] {
            const auto high = byte();
            return (high << 8) | byte();
        };
        const auto grease = [](int value) { return (value & 0x0f0f) == 0x0a0a && (value >> 8) == (value & 255); };
        position += byte();
        const auto cipherLength = number();
        const auto cipherEnd = position + cipherLength;
        QStringList ciphers;
        while (position < cipherEnd) {
            const auto cipher = number();
            if (!grease(cipher))
                ciphers.append(QString::number(cipher));
        }
        QCOMPARE(ciphers.join('-'),
                 QString("4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53"));
        position += byte();
        const auto extensionLength = number();
        const auto extensionEnd = position + extensionLength;
        QStringList extensions;
        QMap<int, QByteArray> payloads;
        while (position < extensionEnd) {
            const auto extension = number();
            const auto length = number();
            QVERIFY(position + length <= hello.size());
            if (!grease(extension)) {
                extensions.append(QString::number(extension));
                payloads[extension] = hello.mid(position, length);
            }
            position += length;
        }
        QCOMPARE(extensions.join('-'), expectedExtensions);
        QCOMPARE(payloads[16], QByteArray::fromHex("000c02683208687474702f312e31"));
        QCOMPARE(payloads[10].mid(2, 2).size(), qsizetype(2));
        // Supported groups retain ML-KEM/X25519/P-256/P-384 after GREASE.
        QCOMPARE(payloads[10].mid(4).toHex(), QByteArray("11ec001d00170018"));
    }
    void realHttpCookiesAndRequestMethods() {
        QFETCH(QString, profile);
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        QByteArray captured;
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            while (server.hasPendingConnections()) {
                auto *socket = server.nextPendingConnection();
                connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
                connect(socket, &QTcpSocket::readyRead, socket, [&, socket, buffer = QByteArray{}]() mutable {
                    buffer += socket->readAll();
                    const auto end = buffer.indexOf("\r\n\r\n");
                    if (end < 0)
                        return;
                    if (buffer.startsWith("POST") && buffer.size() < end + 4 + 7)
                        return;
                    captured = buffer;
                    socket->write("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nSet-Cookie: server=value; "
                                  "Path=/\r\nConnection: close\r\n\r\n{}");
                    socket->disconnectFromHost();
                });
            }
        });
        const QUrl origin("http://127.0.0.1:" + QString::number(server.serverPort()));
        std::atomic<bool> done{false};
        QString failure;
        int status = 0;
        QByteArray initialCookie, receivedCookie, otherSessionCookie;
        std::jthread worker([&] {
            try {
                CurlTransport session(TransportOptions{{}, "KiroX test", profile});
                session.setCookie(origin, "manual", "present");
                initialCookie = session.cookie(origin, "manual");
                HttpRequest request;
                request.url = origin;
                request.method = "POST";
                request.body = "payload";
                request.headers["Content-Type"] = "text/plain";
                request.timeout = std::chrono::seconds(2);
                status = session.send(request).status;
                receivedCookie = session.cookie(origin, "server");
                CurlTransport independent({});
                otherSessionCookie = independent.cookie(origin, "server");
            } catch (const std::exception &e) {
                failure = QString::fromUtf8(e.what());
            }
            done = true;
        });
        QTRY_VERIFY_WITH_TIMEOUT(done.load(), 4000);
        worker.join();
        QVERIFY2(failure.isEmpty(), qPrintable(failure));
        QCOMPARE(status, 200);
        QCOMPARE(initialCookie, QByteArray("present"));
        QCOMPARE(receivedCookie, QByteArray("value"));
        QVERIFY(otherSessionCookie.isEmpty());
        QVERIFY(captured.startsWith("POST / HTTP/1.1"));
        QVERIFY(captured.contains("manual=present"));
        QVERIFY(captured.endsWith("payload"));
    }
    void cancellationAndTimeout_data() {
        QTest::addColumn<bool>("cancel");
        QTest::newRow("cancel") << true;
        QTest::newRow("timeout") << false;
    }
    void cancellationAndTimeout() {
        QFETCH(bool, cancel);
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        std::atomic<bool> done{false};
        ErrorCode result = ErrorCode::InvalidInput;
        const QUrl url("http://127.0.0.1:" + QString::number(server.serverPort()));
        QElapsedTimer elapsed;
        elapsed.start();
        std::jthread worker([&](std::stop_token stop) {
            try {
                CurlTransport session({});
                HttpRequest request;
                request.url = url;
                request.timeout = cancel ? std::chrono::seconds(10) : std::chrono::milliseconds(120);
                (void)session.send(request, stop);
            } catch (const Error &error) {
                result = error.code();
            }
            done = true;
        });
        QTRY_VERIFY_WITH_TIMEOUT(server.hasPendingConnections(), 1000);
        if (cancel)
            worker.request_stop();
        QTRY_VERIFY_WITH_TIMEOUT(done.load(), 1200);
        worker.join();
        QCOMPARE(result, cancel ? ErrorCode::Cancelled : ErrorCode::Timeout);
        QVERIFY(elapsed.elapsed() < 1500);
    }
    void secureCookiesAreScopedAndHeaderInjectionRejected() {
        CurlTransport session({});
        const QUrl origin("https://mail.test/path");
        session.setCookie(origin, "secret", "value");
        QCOMPARE(session.cookie(origin, "secret"), QByteArray("value"));
        QVERIFY(session.cookie(QUrl("http://mail.test/path"), "secret").isEmpty());
        QVERIFY(session.cookie(QUrl("https://other.test/path"), "secret").isEmpty());
        HttpRequest request;
        request.url = origin;
        request.headers["bad"] = "value\r\nInjected: true";
        QVERIFY_THROWS_EXCEPTION(Error, (void)(session.send(request)));
        QVERIFY_THROWS_EXCEPTION(Error, (void)(CurlTransport(TransportOptions{{}, {}, "unimplemented"})));
    }
};
QTEST_GUILESS_MAIN(CurlTests)
#include "curl_tests.moc"
