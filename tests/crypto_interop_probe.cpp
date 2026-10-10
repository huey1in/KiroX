#include "kirox/application/password_encryptor.hpp"
#include "kirox/infrastructure/native_crypto.hpp"
#include <QCoreApplication>
#include <QFile>
#include <QJsonDocument>
#include <cstdio>

int main(int argc, char *argv[]) {
    QCoreApplication app(argc, argv);
    QFile input;
    input.open(stdin, QIODevice::ReadOnly);
    const auto request = QJsonDocument::fromJson(input.readAll()).object();
    try {
        kirox::NativeCryptography crypto;
        kirox::PasswordEncryptor encryptor(crypto);
        const auto jwe = encryptor.encrypt(request["password"].toString(), request["publicKey"].toObject(), "issuer",
                                           "audience", "us-east-1", 1700000000);
        const auto signature = crypto.signEs256("independent-es256-verification");
        const auto output =
            QJsonDocument(QJsonObject{{"jwe", jwe},
                                      {"signature", QString::fromLatin1(signature.signature.toBase64())},
                                      {"signingKey", signature.publicKey}})
                .toJson(QJsonDocument::Compact);
        std::fwrite(output.constData(), 1, output.size(), stdout);
        return 0;
    } catch (const std::exception &error) {
        std::fprintf(stderr, "%s\n", error.what());
        return 1;
    }
}
