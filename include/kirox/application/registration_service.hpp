#pragma once
#include "kirox/domain/registration.hpp"
#include "kirox/ports/crypto.hpp"
#include "kirox/ports/fingerprint_configuration.hpp"
#include <functional>

namespace kirox {
using RegistrationProgress = std::function<void(const QString &step)>;
class IRegistrationService {
  public:
    virtual ~IRegistrationService() = default;
    virtual RegistrationResult run(const RegistrationRequest &request, std::stop_token stop = {},
                                   const RegistrationProgress &progress = {}) const = 0;
};
class RegistrationService final : public IRegistrationService {
  public:
    RegistrationService(const ITransportFactory &transport, const IMailboxFactory &mailboxes,
                        const ICryptography &crypto, IFingerprintConfiguration &fingerprints,
                        RegistrationEndpoints endpoints = {}, RegistrationTiming timing = {});
    RegistrationResult run(const RegistrationRequest &request, std::stop_token stop = {},
                           const RegistrationProgress &progress = {}) const override;

  private:
    const ITransportFactory &transport_;
    const IMailboxFactory &mailboxes_;
    const ICryptography &crypto_;
    IFingerprintConfiguration &fingerprints_;
    RegistrationEndpoints endpoints_;
    RegistrationTiming timing_;
};
} // namespace kirox
