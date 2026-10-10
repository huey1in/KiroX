#pragma once
#include "kirox/ports/fingerprint_configuration.hpp"
#include <condition_variable>
#include <mutex>
#include <thread>

namespace kirox {
class AppScriptCache final : public IFingerprintConfiguration {
  public:
    explicit AppScriptCache(const ITransportFactory &transport,
                            QUrl endpoint = QUrl("https://us-east-1.signin.aws/assets/js/app.js"));
    ~AppScriptCache() override;
    void warm(const TransportOptions &options, const QString &secUserAgent) override;
    void wait(std::stop_token stop = {}) override;
    std::shared_ptr<const FingerprintCryptoConfig> snapshot() const override;

  private:
    const ITransportFactory &transport_;
    QUrl endpoint_;
    std::shared_ptr<const FingerprintCryptoConfig> configuration_;
    mutable std::mutex mutex_;
    std::condition_variable_any ready_;
    bool started_ = false, complete_ = false;
    std::jthread loader_;
};
} // namespace kirox
