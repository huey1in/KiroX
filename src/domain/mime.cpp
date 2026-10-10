#include "kirox/domain/mime.hpp"
#include <QMap>
#include <QRegularExpression>
#include <QStringDecoder>

namespace kirox {
namespace {
QByteArray quotedPrintable(const QByteArray &input) {
    QByteArray result;
    result.reserve(input.size());
    for (qsizetype i = 0; i < input.size(); ++i) {
        if (input[i] != '=') {
            result += input[i];
            continue;
        }
        if (i + 1 < input.size() && input[i + 1] == '\n') {
            ++i;
            continue;
        }
        if (i + 2 < input.size() && input.mid(i + 1, 2) == "\r\n") {
            i += 2;
            continue;
        }
        if (i + 2 < input.size()) {
            bool valid = false;
            const auto byte = input.mid(i + 1, 2).toUInt(&valid, 16);
            if (valid) {
                result += static_cast<char>(byte);
                i += 2;
                continue;
            }
        }
        result += '=';
    }
    return result;
}
QString decode(const QByteArray &bytes, const QString &charset) {
    const auto encoding = QStringConverter::encodingForName(charset.toLatin1().constData());
    if (encoding) {
        QStringDecoder decoder(*encoding);
        return decoder(bytes);
    }
    if (charset.compare("iso-8859-1", Qt::CaseInsensitive) == 0 ||
        charset.compare("windows-1252", Qt::CaseInsensitive) == 0)
        return QString::fromLatin1(bytes);
    return QString::fromUtf8(bytes);
}
QString headerWords(const QString &value) {
    const QRegularExpression pattern("=\\?([^?]+)\\?([bBqQ])\\?([^?]*)\\?=");
    QString result;
    qsizetype offset = 0;
    auto matches = pattern.globalMatch(value);
    while (matches.hasNext()) {
        const auto match = matches.next();
        result += value.mid(offset, match.capturedStart() - offset);
        const auto bytes = match.captured(3).toLatin1();
        result += decode(match.captured(2).compare("b", Qt::CaseInsensitive) == 0
                             ? QByteArray::fromBase64(bytes)
                             : quotedPrintable(QByteArray(bytes).replace('_', ' ')),
                         match.captured(1));
        offset = match.capturedEnd();
    }
    return result + value.mid(offset);
}
QString parameter(const QString &header, const QString &key) {
    const auto match = QRegularExpression("(?:^|;)\\s*" + key + "\\s*=\\s*(?:\"([^\"]*)\"|([^;\\s]+))",
                                          QRegularExpression::CaseInsensitiveOption)
                           .match(header);
    return match.captured(1).isEmpty() ? match.captured(2) : match.captured(1);
}
QString parse(const QByteArray &original, int depth) {
    if (depth > 12 || original.size() > 16 * 1024 * 1024)
        return {};
    auto bytes = original;
    bytes.replace("\r\n", "\n");
    const auto separator = bytes.indexOf("\n\n");
    if (separator < 0)
        return QString::fromUtf8(bytes);
    auto rawHeaders = QString::fromUtf8(bytes.left(separator));
    rawHeaders.replace(QRegularExpression("\\n[ \\t]+"), " ");
    QMap<QString, QString> headers;
    for (const auto &line : rawHeaders.split('\n')) {
        const auto colon = line.indexOf(':');
        if (colon > 0)
            headers[line.left(colon).trimmed().toLower()] = line.mid(colon + 1).trimmed();
    }
    if (headers["content-disposition"].startsWith("attachment", Qt::CaseInsensitive))
        return {};
    QString text = headerWords(headers["subject"]);
    const auto type = headers.value("content-type", "text/plain");
    auto body = bytes.mid(separator + 2);
    if (type.startsWith("multipart/", Qt::CaseInsensitive)) {
        const auto boundary = parameter(type, "boundary").toUtf8();
        if (boundary.isEmpty())
            return text;
        const auto marker = "--" + boundary;
        // Boundaries are recognized as complete lines, not substrings in body text.
        QByteArray part;
        bool inside = false;
        for (const auto &line : body.split('\n')) {
            if (line == marker || line == marker + "--") {
                if (inside && !part.isEmpty())
                    text += "\n" + parse(part, depth + 1);
                part.clear();
                inside = line != marker + "--";
                if (!inside)
                    break;
            } else if (inside) {
                part += line;
                part += '\n';
            }
        }
        return text;
    }
    if (!type.startsWith("text/", Qt::CaseInsensitive))
        return text;
    const auto transfer = headers["content-transfer-encoding"].toLower();
    if (transfer == "base64")
        body = QByteArray::fromBase64(body);
    else if (transfer == "quoted-printable")
        body = quotedPrintable(body);
    return text + "\n" + decode(body, parameter(type, "charset"));
}
} // namespace
QString mailMessageText(const QByteArray &message) {
    return parse(message, 0);
}
} // namespace kirox
