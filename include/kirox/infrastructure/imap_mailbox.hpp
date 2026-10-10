#pragma once
#include "kirox/ports/imap.hpp"
#include "kirox/ports/mailbox.hpp"

namespace kirox {
class NativeImapFactory final : public IImapFactory {
  public:
    explicit NativeImapFactory(QString endpoint = "imaps://outlook.office365.com:993", QString caFile = {})
        : endpoint_(std::move(endpoint)), caFile_(std::move(caFile)) {}
    std::unique_ptr<IImapConnection> create(const QString &email, const QString &token,
                                            const TransportOptions &options) const override;

  private:
    QString endpoint_, caFile_;
};
std::unique_ptr<IMailboxSession> createImapMailbox(const MailboxRequest &request, const ITransportFactory &transports,
                                                   const IImapFactory &imap, const QString &oauthEndpoint);
} // namespace kirox
