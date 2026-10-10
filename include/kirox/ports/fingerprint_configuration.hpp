#pragma once
#include "kirox/domain/fingerprint_crypto.hpp"
#include "kirox/ports/transport.hpp"

namespace kirox {
class IFingerprintConfiguration {
  public:
    virtual ~IFingerprintConfiguration() = default;
    virtual void warm(const TransportOptions &options, const QString &securityUserAgent) = 0;
    virtual void wait(std::stop_token stop = {}) = 0;
    virtual std::shared_ptr<const FingerprintCryptoConfig> snapshot() const = 0;
};
} // namespace kirox
