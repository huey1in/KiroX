#pragma once
#include "kirox/domain/proxy.hpp"
#include "kirox/ports/repository.hpp"
#include "kirox/ports/transport.hpp"
#include <vector>
namespace kirox {
class ProxyService {
  public:
    ProxyService(IRepository &repository, const ITransportFactory &transports)
        : repository_(repository), transports_(transports) {}
    [[nodiscard]] std::vector<ProxyEntry> list() const;
    [[nodiscard]] ProxyEntry add(QString name, const QString &url, int weight = 50);
    void update(const ProxyEntry &entry);
    void remove(const QString &id);
    [[nodiscard]] QJsonObject probe(const QString &id, std::stop_token stop = {});
    [[nodiscard]] QString pick() const;

  private:
    IRepository &repository_;
    const ITransportFactory &transports_;
};
} // namespace kirox
