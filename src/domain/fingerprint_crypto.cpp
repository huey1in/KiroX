#include "kirox/domain/fingerprint_crypto.hpp"
#include <QRegularExpression>
#include <QtEndian>
#include <vector>

namespace kirox {
quint32 crc32(const QByteArray &data) {
    quint32 crc = 0xffffffffU;
    for (const auto byte : data) {
        crc ^= static_cast<quint8>(byte);
        for (int bit = 0; bit < 8; ++bit)
            crc = (crc >> 1) ^ (0xedb88320U & (0U - (crc & 1U)));
    }
    return ~crc;
}
QByteArray xxteaEncrypt(const QByteArray &plaintext, const std::array<quint32, 4> &key) {
    if (plaintext.isEmpty())
        return {};
    const auto size = static_cast<size_t>((plaintext.size() + 3) / 4);
    std::vector<quint32> words(size, 0);
    for (qsizetype i = 0; i < plaintext.size(); ++i)
        words[static_cast<size_t>(i) / 4] |= static_cast<quint32>(static_cast<quint8>(plaintext[i])) << ((i % 4) * 8);
    const auto rounds = 6 + 52 / size;
    quint32 z = words.back(), sum = 0;
    for (size_t round = 0; round < rounds; ++round) {
        sum += 0x9e3779b9U;
        const auto e = (sum >> 2) & 3U;
        for (size_t p = 0; p < size; ++p) {
            const auto y = words[(p + 1) % size];
            const auto mx = ((z >> 5 ^ y << 2) + (y >> 3 ^ z << 4)) ^ ((sum ^ y) + (key[(p & 3U) ^ e] ^ z));
            words[p] += mx;
            z = words[p];
        }
    }
    QByteArray result(static_cast<qsizetype>(size * 4), '\0');
    for (size_t i = 0; i < size; ++i)
        qToLittleEndian(words[i], result.data() + i * 4);
    return result;
}
QString encryptFingerprint(const QByteArray &json, const FingerprintCryptoConfig &configuration) {
    const auto checksum = QByteArray::number(crc32(json), 16).rightJustified(8, '0').toUpper();
    return configuration.identifier + ":" +
           QString::fromLatin1(xxteaEncrypt(checksum + '#' + json, configuration.key).toBase64());
}
FingerprintCryptoConfig fingerprintCryptoFromScript(const QString &javascript) {
    FingerprintCryptoConfig configuration;
    const QRegularExpression candidate(
        R"re(var\s+([A-Za-z_$][\w$]*)\s*=\s*\[\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*"([A-Za-z0-9]+)"\s*,\s*(\d+)[^\]]*\])re");
    auto matches = candidate.globalMatch(javascript);
    while (matches.hasNext()) {
        const auto match = matches.next();
        const auto name = QRegularExpression::escape(match.captured(1));
        const QRegularExpression provider(
            "return\\s*\\{\\s*identifier\\s*:\\s*" + name + "\\[3\\]\\s*,\\s*material\\s*:\\s*\\[\\s*" + name +
            "\\[1\\]\\s*,\\s*" + name + "\\[0\\]\\s*,\\s*" + name + "\\[2\\]\\s*,\\s*" + name + "\\[4\\]\\s*\\]");
        if (!provider.match(javascript).hasMatch())
            continue;
        std::array<quint32, 4> key{};
        bool valid = true;
        constexpr std::array<int, 4> order{3, 2, 4, 6};
        for (size_t i = 0; i < key.size(); ++i) {
            bool parsed = false;
            key[i] = match.captured(order[i]).toUInt(&parsed);
            valid &= parsed;
        }
        if (valid) {
            configuration.key = key;
            configuration.identifier = match.captured(5);
            break;
        }
    }
    const auto version =
        QRegularExpression(R"re(FWCIM_VERSION(?:"\s*\]|)\s*=\s*"(\d+\.\d+\.\d+)")re").match(javascript);
    if (version.hasMatch())
        configuration.version = version.captured(1);
    return configuration;
}
} // namespace kirox
