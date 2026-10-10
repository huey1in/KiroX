#pragma once
#include "kirox/ports/transport.hpp"

namespace kirox {
class IImapConnection {
  public:
    virtual ~IImapConnection() = default;
    virtual QByteArray command(const QString &folder, const QByteArray &command, std::stop_token stop,
                               std::chrono::milliseconds timeout) = 0;
    virtual QByteArray message(const QString &folder, quint32 uid, std::stop_token stop,
                               std::chrono::milliseconds timeout) = 0;
};
class IImapFactory {
  public:
    virtual ~IImapFactory() = default;
    virtual std::unique_ptr<IImapConnection> create(const QString &email, const QString &token,
                                                    const TransportOptions &options) const = 0;
};
} // namespace kirox
