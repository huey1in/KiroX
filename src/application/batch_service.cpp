#include "kirox/application/batch_service.hpp"
#include "kirox/application/mailbox_service.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/error.hpp"
#include "kirox/domain/proxy.hpp"
#include <QDir>
#include <QJsonArray>
#include <QRandomGenerator>
#include <QUuid>
#include <atomic>

namespace kirox {
struct BatchService::Plan {
    QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    BatchRequest request;
    Settings settings;
    QList<MailboxAccount> accounts;
    QList<QJsonObject> configurations;
    std::vector<ProxyEntry> proxies;
    std::stop_source cancellation;
    std::atomic<int> nextTask{0};
    std::mutex poolMutex;
    qsizetype nextAccount = 0;
    std::chrono::steady_clock::time_point started = std::chrono::steady_clock::now();
};
BatchService::BatchService(IRepository &repository, const IRegistrationService &registration)
    : repository_(repository), registration_(registration), accounts_(repository), identities_(repository),
      state_{{"running", false}, {"total", 0},     {"completed", 0}, {"success", 0},
             {"failed", 0},      {"cancelled", 0}, {"elapsed", 0}} {}
BatchService::~BatchService() {
    stop();
    wait();
}
void BatchService::start(const BatchRequest &request, const Settings &settings, BatchObserver observer) {
    std::scoped_lock lifecycle(lifecycle_);
    {
        std::scoped_lock lock(mutex_);
        if (state_.value("running").toBool())
            throw Error(ErrorCode::Conflict, "A batch is already running");
    }
    if (worker_.joinable())
        worker_.join();
    if (request.count <= 0 || request.count > 10000 || request.concurrency <= 0 || request.concurrency > 64 ||
        request.delaySeconds < 0 || request.delaySeconds > 86400)
        throw Error(ErrorCode::InvalidInput, "Invalid batch size, concurrency or delay");
    auto plan = std::make_shared<Plan>();
    plan->request = request;
    plan->settings = settings;
    plan->settings.normalize();
    if (request.provider == MailboxKind::Outlook || request.provider == MailboxKind::ICloud) {
        for (const auto &account : accounts_.list(request.provider, true))
            plan->accounts.append(account);
        if (plan->accounts.size() < request.count)
            throw Error(ErrorCode::InvalidInput, "Insufficient available mailbox accounts");
    } else {
        const auto document = MailboxConfigurationService(repository_).get(request.provider);
        const auto entries = document.isObject() ? QJsonArray{document.object()} : document.array();
        for (const auto &entry : entries) {
            const auto config = entry.toObject();
            if (!config.isEmpty() &&
                (request.configurationNames.isEmpty() || request.provider == MailboxKind::MailNest ||
                 request.configurationNames.contains(config.value("name").toString())))
                plan->configurations.append(config);
        }
        if (plan->configurations.isEmpty())
            throw Error(ErrorCode::InvalidInput, "Select a configured mailbox service");
        for (const auto &configuration : plan->configurations)
            MailboxConfigurationService::validate(request.provider, configuration);
        for (const auto &name : request.configurationNames) {
            if (request.provider == MailboxKind::MailNest)
                break;
            if (std::none_of(plan->configurations.begin(), plan->configurations.end(),
                             [&](const auto &config) { return config.value("name") == name; }))
                throw Error(ErrorCode::InvalidInput, "Mailbox configuration no longer exists");
        }
        for (const auto &domain : request.domains)
            if (domain.trimmed().isEmpty() || domain.contains('/') || domain.contains('@') || domain.contains(' '))
                throw Error(ErrorCode::InvalidInput, "Invalid mailbox domain");
    }
    if (request.proxyMode != "direct" && request.proxyMode != "selected" && request.proxyMode != "pool")
        throw Error(ErrorCode::InvalidInput, "Invalid proxy selection");
    for (const auto &value : repository_.read(Document::ProxyPool).object().value("entries").toArray()) {
        auto entry = ProxyEntry::fromJson(value.toObject());
        if (entry.enabled &&
            (request.proxyMode == "pool" || (request.proxyMode == "selected" && entry.id == request.proxyId))) {
            entry.url = normalizeProxy(entry.url);
            if (entry.url.isEmpty())
                throw Error(ErrorCode::InvalidInput, "Empty proxy URL");
            plan->proxies.push_back(entry);
        }
    }
    if (request.proxyMode != "direct" && plan->proxies.empty())
        throw Error(ErrorCode::InvalidInput, "No enabled proxy is available");
    if (!request.outputDirectory.isEmpty() && !QDir().mkpath(request.outputDirectory))
        throw Error(ErrorCode::Storage, "Cannot create the result output directory");
    {
        std::scoped_lock lock(mutex_);
        plan_ = plan;
        tasks_ = {};
        for (int i = 0; i < request.count; ++i)
            tasks_.append(QJsonObject{{"index", i + 1}, {"status", "queued"}, {"step", ""}});
        state_ = {{"running", true}, {"stopping", false}, {"total", request.count},
                  {"completed", 0},  {"success", 0},      {"failed", 0},
                  {"cancelled", 0},  {"elapsed", 0},      {"riskStopped", false}};
        state_.insert("batchId", plan->id);
    }
    try {
        worker_ = std::jthread([this, plan, observer = std::move(observer)] { run(plan, observer); });
    } catch (...) {
        std::scoped_lock lock(mutex_);
        state_.insert("running", false);
        plan_.reset();
        throw;
    }
}
bool BatchService::stop() {
    std::shared_ptr<Plan> plan;
    {
        std::scoped_lock lock(mutex_);
        if (!state_.value("running").toBool())
            return false;
        plan = plan_;
        state_.insert("stopping", true);
    }
    return plan && plan->cancellation.request_stop();
}
void BatchService::wait() {
    std::scoped_lock lifecycle(lifecycle_);
    if (worker_.joinable())
        worker_.join();
}
QJsonObject BatchService::status() const {
    std::scoped_lock lock(mutex_);
    auto result = state_;
    if (plan_ && result.value("running").toBool())
        result.insert("elapsed",
                      std::chrono::duration<double>(std::chrono::steady_clock::now() - plan_->started).count());
    return result;
}
QJsonArray BatchService::tasks() const {
    std::scoped_lock lock(mutex_);
    return tasks_;
}
void BatchService::publish(const std::shared_ptr<Plan> &plan, QJsonObject event, const BatchObserver &observer) {
    event.insert("batchId", plan->id);
    {
        std::scoped_lock lock(mutex_);
        if (plan_ != plan)
            return;
        const auto index = event.value("index").toInt() - 1;
        if (index >= 0 && index < tasks_.size()) {
            auto row = tasks_.at(index).toObject();
            for (auto it = event.begin(); it != event.end(); ++it)
                row.insert(it.key(), it.value());
            tasks_[index] = row;
        }
    }
    if (observer) {
        try {
            observer(event);
        } catch (...) {
        }
    }
}
void BatchService::process(const std::shared_ptr<Plan> &plan, int index, const BatchObserver &observer) {
    const auto stop = plan->cancellation.get_token();
    RegistrationResult result;
    RegistrationRequest request;
    const bool pool = plan->request.provider == MailboxKind::Outlook || plan->request.provider == MailboxKind::ICloud;
    QString warning;
    try {
        checkCancelled(stop);
        request.mailbox.provider = plan->request.provider;
        const auto claim = [&] {
            std::scoped_lock lock(plan->poolMutex);
            if (plan->nextAccount >= plan->accounts.size())
                throw Error(ErrorCode::NotFound, "Mailbox pool exhausted");
            request.mailbox.account = plan->accounts.at(plan->nextAccount++);
        };
        if (pool)
            claim();
        else {
            if (!plan->request.domains.isEmpty()) {
                const int selected = plan->request.randomDomains
                                         ? QRandomGenerator::system()->bounded(int(plan->request.domains.size()))
                                         : index % int(plan->request.domains.size());
                request.mailbox.domain = plan->request.domains.at(selected);
            }
            QList<QJsonObject> candidates;
            for (const auto &config : plan->configurations) {
                const auto domains = config.value("domains").toArray();
                if (request.mailbox.domain.isEmpty() || domains.isEmpty() || domains.contains(request.mailbox.domain))
                    candidates.append(config);
            }
            if (candidates.isEmpty())
                throw Error(ErrorCode::InvalidInput, "No mailbox configuration supports the selected domain");
            request.mailbox.configuration = candidates.at(QRandomGenerator::system()->bounded(int(candidates.size())));
            request.mailbox.name = generateMailboxName();
            request.mailbox.expiryMilliseconds = qint64(plan->settings.moeMailExpiryMinutes) * 60000;
        }
        if (!plan->proxies.empty()) {
            int total = 0;
            for (const auto &entry : plan->proxies)
                total += entry.weight;
            int target = QRandomGenerator::system()->bounded(total);
            for (const auto &entry : plan->proxies) {
                target -= entry.weight;
                if (target < 0) {
                    request.transport.proxy = entry.url;
                    break;
                }
            }
        }
        request.identity = identities_.forProxy(request.transport.proxy);
        request.transport.userAgent = request.identity.userAgent();
        request.transport.browserProfile = "chrome" + request.identity.majorVersion();
        request.mailbox.transport = {plan->settings.mailboxProxy(request.transport.proxy), request.identity.userAgent(),
                                     request.transport.browserProfile};
        request.otpTimeoutSeconds = plan->settings.otpTimeoutSeconds;
        request.networkRetries =
            plan->settings.retryProfile == "fast" ? 0 : (plan->settings.retryProfile == "stable" ? 3 : 2);
        // Avoid allocating a second paid mailbox after an ambiguous network failure.
        const int attempts =
            !pool || plan->settings.retryProfile == "fast" ? 1 : (plan->settings.retryProfile == "stable" ? 3 : 2);
        publish(plan, {{"index", index + 1}, {"status", "running"}}, observer);
        for (int attempt = 0; attempt < attempts; ++attempt) {
            checkCancelled(stop);
            if (attempt > 0)
                interruptibleWait(stop, std::chrono::seconds(2 + attempt));
            result = registration_.run(request, stop, [&](const QString &step) {
                publish(plan, {{"index", index + 1}, {"step", step}}, observer);
            });
            if (result.successful() || result.passwordSet() || result.risk() ||
                result.values.value("errorCode") == "CANCELLED")
                break;
            if (pool && result.values.value("errorCode") == "EMAIL_ALREADY_REGISTERED") {
                accounts_.mark(plan->request.provider, request.mailbox.account.email, false);
                claim();
                attempt = -1;
                continue;
            }
            const auto code = result.values.value("errorCode").toString();
            if (code != "NETWORK_ERROR" && code != "TIMEOUT")
                break;
        }
        if (pool && (result.passwordSet() || result.successful())) {
            try {
                accounts_.mark(plan->request.provider, request.mailbox.account.email, result.successful());
            } catch (const std::exception &) {
                warning = "MAILBOX_STATUS_NOT_SAVED";
                // Stop before another worker can reuse an account whose status was not persisted.
                plan->cancellation.request_stop();
            }
        }
        if (result.successful()) {
            try {
                repository_.saveResult(result.values, plan->request.outputDirectory);
            } catch (const std::exception &) {
                repository_.saveResult(result.values, QDir(repository_.paths().root).filePath("recovery-results"));
                warning = "RESULT_SAVED_TO_RECOVERY";
            }
        }
    } catch (const Error &error) {
        const bool passwordSet = result.passwordSet(), risk = result.risk();
        result.values = {{"status", "failed"},
                         {"email", request.mailbox.account.email},
                         {"errorCode", error.code() == ErrorCode::Cancelled ? "CANCELLED" : "TASK_ERROR"},
                         {"passwordSet", passwordSet},
                         {"risk", risk}};
    } catch (const std::exception &) {
        const bool passwordSet = result.passwordSet(), risk = result.risk();
        result.values = {{"status", "failed"},
                         {"email", request.mailbox.account.email},
                         {"errorCode", "TASK_ERROR"},
                         {"passwordSet", passwordSet},
                         {"risk", risk}};
    }
    if (result.risk() && plan->settings.stopOnRisk) {
        {
            std::scoped_lock lock(mutex_);
            state_.insert("riskStopped", true);
            state_.insert("stopping", true);
        }
        plan->cancellation.request_stop();
    }
    const bool cancelled = result.values.value("errorCode") == "CANCELLED";
    {
        std::scoped_lock lock(mutex_);
        state_.insert("completed", state_.value("completed").toInt() + 1);
        const auto key = result.successful() ? "success" : (cancelled ? "cancelled" : "failed");
        state_.insert(key, state_.value(key).toInt() + 1);
    }
    publish(plan,
            {{"index", index + 1},
             {"status", result.successful() ? "success" : (cancelled ? "cancelled" : "failed")},
             {"email", result.values.value("email")},
             {"errorCode", result.values.value("errorCode")},
             {"passwordSet", result.passwordSet()},
             {"warning", warning}},
            observer);
}
void BatchService::run(std::shared_ptr<Plan> plan, BatchObserver observer) {
    std::vector<std::jthread> workers;
    try {
        for (int i = 0; i < std::min(plan->request.count, plan->request.concurrency); ++i)
            workers.emplace_back([this, plan, observer] {
                const auto stop = plan->cancellation.get_token();
                while (!stop.stop_requested()) {
                    const int index = plan->nextTask.fetch_add(1);
                    if (index >= plan->request.count)
                        break;
                    process(plan, index, observer);
                    try {
                        if (plan->request.concurrency == 1 && plan->request.delaySeconds > 0 && index + 1 < plan->request.count)
                            interruptibleWait(stop, std::chrono::seconds(plan->request.delaySeconds));
                    } catch (const Error &) {
                        break;
                    }
                }
            });
        for (auto &worker : workers)
            worker.join();
    } catch (...) {
        plan->cancellation.request_stop();
        workers.clear();
    }
    for (int i = 0; i < plan->request.count; ++i) {
        bool queued;
        {
            std::scoped_lock lock(mutex_);
            queued = tasks_.at(i).toObject().value("status") == "queued";
            if (queued) {
                state_.insert("completed", state_.value("completed").toInt() + 1);
                state_.insert("cancelled", state_.value("cancelled").toInt() + 1);
            }
        }
        if (queued)
            publish(plan, {{"index", i + 1}, {"status", "cancelled"}, {"errorCode", "CANCELLED"}}, observer);
    }
    {
        std::scoped_lock lock(mutex_);
        state_.insert("running", false);
        state_.insert("stopping", false);
        state_.insert("elapsed",
                      std::chrono::duration<double>(std::chrono::steady_clock::now() - plan->started).count());
    }
    publish(plan, {{"event", "finished"}}, observer);
}
} // namespace kirox
