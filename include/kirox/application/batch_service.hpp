#pragma once
#include "kirox/application/account_service.hpp"
#include "kirox/application/identity_service.hpp"
#include "kirox/application/registration_service.hpp"
#include "kirox/domain/settings.hpp"
#include <QJsonArray>
#include <mutex>
#include <thread>

namespace kirox {
struct BatchRequest {
    int count = 1, concurrency = 1, delaySeconds = 1;
    MailboxKind provider = MailboxKind::Outlook;
    QStringList configurationNames, domains;
    bool randomDomains = false;
    QString proxyMode = "direct", proxyId, outputDirectory;
};
using BatchObserver = std::function<void(const QJsonObject &event)>;
class BatchService {
  public:
    BatchService(IRepository &repository, const IRegistrationService &registration);
    ~BatchService();
    // Admission is synchronous. Cancellation is published before threads start.
    void start(const BatchRequest &request, const Settings &settings, BatchObserver observer = {});
    bool stop();
    void wait();
    [[nodiscard]] QJsonObject status() const;
    [[nodiscard]] QJsonArray tasks() const;

  private:
    struct Plan;
    void run(std::shared_ptr<Plan> plan, BatchObserver observer);
    void process(const std::shared_ptr<Plan> &plan, int index, const BatchObserver &observer);
    void publish(const std::shared_ptr<Plan> &plan, QJsonObject event, const BatchObserver &observer);
    IRepository &repository_;
    const IRegistrationService &registration_;
    AccountService accounts_;
    IdentityService identities_;
    mutable std::mutex mutex_;
    std::mutex lifecycle_;
    std::shared_ptr<Plan> plan_;
    QJsonObject state_;
    QJsonArray tasks_;
    std::jthread worker_;
};
} // namespace kirox
