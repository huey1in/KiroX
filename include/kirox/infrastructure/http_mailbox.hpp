#pragma once
#include "kirox/ports/imap.hpp"
#include "kirox/ports/mailbox.hpp"

namespace kirox {
// Endpoint options allow deterministic local protocol tests, never user secrets.
struct MailboxEndpoints {
    QString mailNest = "https://mailnest.top";
    QString outlookOAuth = "https://login.microsoftonline.com";
    QString outlookGraph = "https://graph.microsoft.com/v1.0";
};
class HttpMailboxFactory final : public IMailboxFactory {
  public:
    explicit HttpMailboxFactory(const ITransportFactory &transports, MailboxEndpoints endpoints = {},
                                const IImapFactory *imap = nullptr)
        : transports_(transports), endpoints_(std::move(endpoints)), imap_(imap) {}
    std::unique_ptr<IMailboxSession> create(const MailboxRequest &request) const override;
    QJsonObject inspect(MailboxKind provider, const QJsonObject &configuration, const TransportOptions &transport = {},
                        std::stop_token stop = {}) const override;

  private:
    const ITransportFactory &transports_;
    MailboxEndpoints endpoints_;
    const IImapFactory *imap_;
};
} // namespace kirox
