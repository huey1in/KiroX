#pragma once
#include "kirox/domain/mailbox.hpp"
#include "kirox/ports/transport.hpp"
#include <QJsonValue>
#include <functional>

namespace kirox {
struct MailboxRequest {
    MailboxKind provider = MailboxKind::Outlook;
    MailboxAccount account;
    QJsonObject configuration;
    QString name, domain;
    qint64 expiryMilliseconds = 3600000;
    TransportOptions transport;
};
// A session is confined to its task's worker. open() establishes a historical
// baseline before the registration sends an OTP. poll() only inspects new mail.
class IMailboxSession {
  public:
    virtual ~IMailboxSession() = default;
    virtual QString open(std::stop_token stop = {}) = 0;
    virtual QString poll(std::stop_token stop = {}, std::chrono::milliseconds budget = std::chrono::seconds(30)) = 0;
};
class IMailboxFactory {
  public:
    virtual ~IMailboxFactory() = default;
    virtual std::unique_ptr<IMailboxSession> create(const MailboxRequest &request) const = 0;
    virtual QJsonObject inspect(MailboxKind provider, const QJsonObject &configuration,
                                const TransportOptions &transport = {}, std::stop_token stop = {}) const = 0;
};
} // namespace kirox
