#pragma once
#include <QByteArray>
#include <QString>
#include <array>

namespace kirox {
struct FingerprintCryptoConfig {
    std::array<quint32, 4> key{1888420705U, 2576816180U, 2347232058U, 874813317U};
    QString version = "4.0.0";
    QString identifier = "ECdITeCs";
};
quint32 crc32(const QByteArray &data);
QByteArray xxteaEncrypt(const QByteArray &plaintext, const std::array<quint32, 4> &key);
QString encryptFingerprint(const QByteArray &json, const FingerprintCryptoConfig &configuration);
FingerprintCryptoConfig fingerprintCryptoFromScript(const QString &javascript);
} // namespace kirox
