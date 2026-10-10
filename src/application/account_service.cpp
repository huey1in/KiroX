#include "kirox/application/account_service.hpp"
#include "kirox/domain/error.hpp"
#include <QDateTime>
namespace kirox {
namespace {
bool matches(const QJsonObject &o, MailboxKind kind) {
    return providerKind(o.value("provider").toString()) == kind;
}
QString now() {
    return QDateTime::currentDateTime().toString("yyyy-MM-dd HH:mm:ss");
}
} // namespace
std::vector<MailboxAccount> AccountService::list(MailboxKind kind, bool availableOnly) const {
    std::vector<MailboxAccount> output;
    for (const auto &value : repository_.read(Document::Accounts).array()) {
        auto object = value.toObject();
        if (matches(object, kind) && (!availableOnly || !object.value("registered").toBool()))
            output.push_back(MailboxAccount::fromJson(object));
    }
    return output;
}
ImportResult AccountService::import(MailboxKind kind, const QString &text) {
    auto parsed = parseAccounts(kind, text);
    ImportResult result{0, 0, parsed.invalid};
    if (parsed.accounts.empty())
        throw Error(ErrorCode::InvalidInput, "No valid accounts found");
    repository_.update(Document::Accounts, [&](QJsonDocument &value) {
        auto items = value.array();
        for (auto &account : parsed.accounts) {
            bool duplicate = false;
            for (const auto &item : items)
                if (matches(item.toObject(), kind) && item.toObject().value("email").toString() == account.email) {
                    duplicate = true;
                    break;
                }
            if (duplicate) {
                ++result.duplicates;
                continue;
            }
            auto o = account.toJson();
            o["addedAt"] = now();
            items.append(o);
            ++result.added;
        }
        value.setArray(items);
    });
    return result;
}
void AccountService::remove(MailboxKind kind, const QString &email) {
    repository_.update(Document::Accounts, [&](QJsonDocument &value) {
        QJsonArray output;
        bool found = false;
        for (const auto &item : value.array()) {
            const auto o = item.toObject();
            if (matches(o, kind) && o.value("email").toString() == email)
                found = true;
            else
                output.append(item);
        }
        if (!found)
            throw Error(ErrorCode::NotFound, "Account not found");
        value.setArray(output);
    });
}
int AccountService::clear(MailboxKind kind, bool registeredOnly) {
    int removed = 0;
    repository_.update(Document::Accounts, [&](QJsonDocument &value) {
        QJsonArray output;
        for (const auto &item : value.array()) {
            const auto o = item.toObject();
            if (matches(o, kind) && (!registeredOnly || o.value("registered").toBool()))
                ++removed;
            else
                output.append(item);
        }
        value.setArray(output);
    });
    return removed;
}
void AccountService::mark(MailboxKind kind, const QString &email, bool success) {
    repository_.update(Document::Accounts, [&](QJsonDocument &value) {
        auto items = value.array();
        bool found = false;
        for (qsizetype i = 0; i < items.size(); ++i) {
            auto o = items[i].toObject();
            if (matches(o, kind) && o.value("email").toString() == email) {
                o["registered"] = true;
                o["success"] = success;
                o["registeredAt"] = now();
                items[i] = o;
                found = true;
                break;
            }
        }
        if (!found)
            throw Error(ErrorCode::NotFound, "Account not found");
        value.setArray(items);
    });
}
} // namespace kirox
