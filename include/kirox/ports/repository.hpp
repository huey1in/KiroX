#pragma once
#include <QJsonDocument>
#include <QString>
#include <functional>
namespace kirox {
enum class Document { Settings, Accounts, MoeMail, CloudMail, MailNest, ProxyPool, Identities };
struct DataPaths {
    QString root, data, results, cache, logs;
};
class IRepository {
  public:
    virtual ~IRepository() = default;
    [[nodiscard]] virtual QJsonDocument read(Document document) const = 0;
    virtual void update(Document document, const std::function<void(QJsonDocument &)> &mutation) = 0;
    [[nodiscard]] virtual DataPaths paths() const = 0;
    virtual void relocateData(const QString &target) = 0;
    virtual void setResultsDirectory(const QString &target) = 0;
    virtual void saveResult(const QJsonObject &result, const QString &outputDirectory = {}) = 0;
};
} // namespace kirox
