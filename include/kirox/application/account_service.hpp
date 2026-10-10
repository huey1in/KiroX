#pragma once
#include "kirox/domain/mailbox.hpp"
#include "kirox/ports/repository.hpp"
namespace kirox {
class AccountService {
  public:
    explicit AccountService(IRepository &repository) : repository_(repository) {}
    [[nodiscard]] std::vector<MailboxAccount> list(MailboxKind kind, bool availableOnly = false) const;
    [[nodiscard]] ImportResult import(MailboxKind kind, const QString &text);
    void remove(MailboxKind kind, const QString &email);
    [[nodiscard]] int clear(MailboxKind kind, bool registeredOnly = false);
    void mark(MailboxKind kind, const QString &email, bool success);

  private:
    IRepository &repository_;
};
} // namespace kirox
