#include "kirox/infrastructure/app_script_cache.hpp"
#include "kirox/domain/cancellation.hpp"

namespace kirox {
AppScriptCache::AppScriptCache(const ITransportFactory &transport, QUrl endpoint)
    : transport_(transport), endpoint_(std::move(endpoint)),
      configuration_(std::make_shared<const FingerprintCryptoConfig>()) {}
AppScriptCache::~AppScriptCache() {
    loader_.request_stop();
}
void AppScriptCache::warm(const TransportOptions &options, const QString &secUserAgent) {
    std::lock_guard lock(mutex_);
    if (started_)
        return;
    loader_ = std::jthread([this, options, secUserAgent](std::stop_token stop) {
        auto configuration = std::make_shared<FingerprintCryptoConfig>();
        try {
            auto client = transport_.create(options);
            HttpRequest request;
            request.url = endpoint_;
            request.timeout = std::chrono::seconds(15);
            request.headers = {{"Accept", "*/*"},
                               {"Accept-Language", "en-US,en;q=0.9"},
                               {"Referer", "https://us-east-1.signin.aws/"},
                               {"sec-ch-ua", secUserAgent.toUtf8()},
                               {"sec-fetch-dest", "script"},
                               {"sec-fetch-mode", "no-cors"},
                               {"sec-fetch-site", "same-origin"}};
            const auto response = client->send(request, stop);
            response.requireSuccess();
            *configuration = fingerprintCryptoFromScript(QString::fromUtf8(response.body));
        } catch (const std::exception &) {
            // The baseline implementation also falls back when the remote bundle
            // cannot be loaded. Publish one immutable snapshot to every task.
        }
        {
            std::lock_guard lock(mutex_);
            configuration_ = std::move(configuration);
            complete_ = true;
        }
        ready_.notify_all();
    });
    started_ = true;
}
void AppScriptCache::wait(std::stop_token stop) {
    checkCancelled(stop);
    std::unique_lock lock(mutex_);
    if (!started_)
        throw Error(ErrorCode::Conflict, "Application script cache has not been started");
    ready_.wait(lock, stop, [&] { return complete_; });
    checkCancelled(stop);
}
std::shared_ptr<const FingerprintCryptoConfig> AppScriptCache::snapshot() const {
    std::lock_guard lock(mutex_);
    return configuration_;
}
} // namespace kirox
