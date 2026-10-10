#pragma once
#include "kirox/ports/mailbox.hpp"
#include "kirox/ports/repository.hpp"

namespace kirox {
class MailboxConfigurationService {
  public:
    explicit MailboxConfigurationService(IRepository &repository) : repository_(repository) {}
    QJsonDocument get(MailboxKind provider) const;
    static void validate(MailboxKind provider, const QJsonObject &configuration);
    void save(MailboxKind provider, const QJsonDocument &configuration);
    void upsert(MailboxKind provider, const QJsonObject &configuration, const QString &oldName = {});
    void remove(MailboxKind provider, const QString &name);

  private:
    IRepository &repository_;
};
// Deadline and cancellation are shared across all provider implementations.
QString waitForVerificationCode(IMailboxSession &session, std::chrono::milliseconds timeout,
                                std::chrono::milliseconds interval, std::stop_token stop = {},
                                const std::function<void(const QString &)> &onRetry = {});
QString generateMailboxName();
} // namespace kirox
