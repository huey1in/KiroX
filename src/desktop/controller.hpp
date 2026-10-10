#pragma once
#include "kirox/application/account_service.hpp"
#include "kirox/application/batch_service.hpp"
#include "kirox/application/mailbox_service.hpp"
#include "kirox/application/proxy_service.hpp"
#include "kirox/application/settings_service.hpp"
#include "kirox/application/update_service.hpp"
#include "kirox/ports/activity_log.hpp"
#include <QObject>
#include <QVariantList>
#include <QVariantMap>
#include <thread>
namespace kirox {
class Controller final : public QObject {
    Q_OBJECT
    Q_PROPERTY(QVariantMap settings READ settings NOTIFY settingsChanged)
    Q_PROPERTY(QVariantList mailboxes READ mailboxes NOTIFY mailboxesChanged)
    Q_PROPERTY(QVariantList proxies READ proxies NOTIFY proxiesChanged)
    Q_PROPERTY(QVariantMap directories READ directories NOTIFY settingsChanged)
    Q_PROPERTY(QString language READ language NOTIFY settingsChanged)
    Q_PROPERTY(bool busy READ busy NOTIFY busyChanged)
    Q_PROPERTY(QVariantList providerConfigurations READ providerConfigurations NOTIFY providerConfigurationsChanged)
    Q_PROPERTY(QVariantMap batchStatus READ batchStatus NOTIFY batchChanged)
    Q_PROPERTY(QVariantList tasks READ tasks NOTIFY batchChanged)
    Q_PROPERTY(QVariantList activity READ activity NOTIFY activityChanged)
    Q_PROPERTY(QVariantMap updateInfo READ updateInfo NOTIFY updateChanged)
    Q_PROPERTY(bool updateChecking READ updateChecking NOTIFY updateChanged)
  public:
    Controller(IRepository &repository, const ITransportFactory &transports, const IMailboxFactory &mailboxes,
               const IRegistrationService &registration, IActivityLog &log);
    ~Controller() override;
    QVariantMap settings() const;
    QVariantList mailboxes() const;
    QVariantList proxies() const;
    QVariantMap directories() const;
    QString language() const;
    bool busy() const {
        return busy_;
    }
    QVariantList providerConfigurations() const;
    QVariantMap batchStatus() const;
    QVariantList tasks() const;
    QVariantList activity() const;
    QVariantMap updateInfo() const {
        return updateInfo_;
    }
    bool updateChecking() const {
        return updateChecking_;
    }
    Q_INVOKABLE void checkUpdates();
    Q_INVOKABLE bool startBatch(const QVariantMap &request);
    Q_INVOKABLE void stopBatch();
    Q_INVOKABLE void clearActivity();
    Q_INVOKABLE QString stepLabel(const QString &step) const;
    Q_INVOKABLE QVariantMap providerConfiguration(const QString &provider, const QString &name);
    Q_INVOKABLE bool saveProviderConfiguration(const QString &provider, const QVariantMap &configuration,
                                               const QString &oldName);
    Q_INVOKABLE void removeProviderConfiguration(const QString &provider, const QString &name);
    Q_INVOKABLE void inspectProvider(const QString &provider, const QString &name);
    Q_INVOKABLE void cancelOperation();
    Q_INVOKABLE bool patchSettings(const QVariantMap &changes);
    Q_INVOKABLE void importAccounts(const QString &provider, const QString &text);
    Q_INVOKABLE void importAccountsFile(const QString &provider);
    Q_INVOKABLE void deleteAccount(const QString &provider, const QString &address);
    Q_INVOKABLE void clearAccounts(const QString &provider, bool registeredOnly);
    Q_INVOKABLE void addProxy(const QString &name, const QString &url, int weight);
    Q_INVOKABLE QVariantMap proxyConfiguration(const QString &id);
    Q_INVOKABLE bool saveProxy(const QString &id, const QString &name, const QString &url, int weight);
    Q_INVOKABLE void importProxies(const QString &text);
    Q_INVOKABLE void deleteProxy(const QString &id);
    Q_INVOKABLE void enableProxy(const QString &id, bool enabled);
    Q_INVOKABLE void testProxy(const QString &id);
    Q_INVOKABLE void selectDirectory(const QString &kind);
    Q_INVOKABLE void resetDirectory(const QString &kind);
    Q_INVOKABLE void openDirectory(const QString &kind);
    Q_INVOKABLE void openUrl(const QString &url);
    Q_INVOKABLE void quit();
  signals:
    void settingsChanged();
    void mailboxesChanged();
    void proxiesChanged();
    void busyChanged();
    void providerConfigurationsChanged();
    void batchChanged();
    void activityChanged();
    void updateChanged();
    void batchFinished(const QVariantMap &status);
    void providerInspected(const QString &provider, const QVariantMap &result);
    void operationSucceeded(const QString &message);
    void operationFailed(const QString &message);

  private:
    template <class Action> bool perform(Action action) {
        try {
            action();
            return true;
        } catch (const std::exception &error) {
            emit operationFailed(errorMessage(error));
            return false;
        }
    }
    QString text(const QString &zh, const QString &en, const QString &ja) const;
    QString errorMessage(const std::exception &error) const;
    void probeProxies(const QStringList &ids);
    SettingsService settings_;
    AccountService accounts_;
    ProxyService proxies_;
    MailboxConfigurationService configurations_;
    BatchService batch_;
    IActivityLog &log_;
    UpdateService updates_;
    QVariantMap updateInfo_;
    bool updateChecking_ = false;
    const IMailboxFactory &mailboxFactory_;
    bool busy_ = false;
    std::jthread worker_;
    std::jthread updateWorker_;
};
} // namespace kirox
