#include "kirox/domain/error.hpp"
#include "kirox/infrastructure/curl_transport.hpp"
#include <QCoreApplication>
#include <cstdio>

int main(int argc, char **argv) {
    QCoreApplication app(argc, argv);
    if (argc != 6)
        return 2;
    try {
        kirox::CurlTransport transport({QString::fromLocal8Bit(argv[2]), {}, QString::fromLocal8Bit(argv[5])},
                                       QString::fromLocal8Bit(argv[3]), QString::fromLocal8Bit(argv[4]));
        kirox::HttpRequest request;
        request.url = QUrl(QString::fromLocal8Bit(argv[1]));
        request.timeout = std::chrono::seconds(5);
        const auto response = transport.send(request);
        if (response.status != 200 || response.body != "verified")
            return 3;
        return 0;
    } catch (const kirox::Error &error) {
        std::fprintf(stderr, "%s\n", error.what());
        return error.code() == kirox::ErrorCode::Network ? 4 : 5;
    }
}
