#pragma once
#include <QByteArray>
#include <QString>

namespace kirox {
// Decode text MIME parts; skip attachments and binary content.
QString mailMessageText(const QByteArray &message);
} // namespace kirox
