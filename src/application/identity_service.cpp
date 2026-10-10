#include "kirox/application/identity_service.hpp"
#include <QRandomGenerator>

namespace kirox {
BrowserIdentity IdentityService::forProxy(const QString &proxy, bool registration, qint64 nowSeconds) {
    BrowserIdentity selected;
    const auto key = browserIdentityKey(proxy);
    repository_.update(Document::Identities, [&](QJsonDocument &document) {
        auto entries = document.object();
        // Prune expired records without discarding unrelated valid cache entries.
        for (auto it = entries.begin(); it != entries.end();) {
            const auto created = it.value().toObject().value("createdAt").toInteger();
            if (created > nowSeconds || nowSeconds - created >= 6 * 60 * 60)
                it = entries.erase(it);
            else
                ++it;
        }
        auto entry = entries.value(key).toObject();
        selected = BrowserIdentity(entry.value("identity").toObject());
        if (!selected.valid()) {
            selected = BrowserIdentity::generate();
            entries.insert(key, QJsonObject{{"identity", selected.json()}, {"createdAt", nowSeconds}});
        }
        document = QJsonDocument(entries);
    });
    return selected.newSession(registration && QRandomGenerator::system()->bounded(100) < 15);
}
} // namespace kirox
