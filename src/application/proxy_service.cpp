#include "kirox/application/proxy_service.hpp"
#include "kirox/domain/error.hpp"
#include <QDateTime>
#include <QElapsedTimer>
#include <QJsonArray>
#include <QRandomGenerator>
#include <QUuid>
#include <algorithm>
namespace kirox {
std::vector<ProxyEntry> ProxyService::list() const {
    std::vector<ProxyEntry> result;
    for (const auto &o : repository_.read(Document::ProxyPool).object().value("entries").toArray())
        result.push_back(ProxyEntry::fromJson(o.toObject()));
    return result;
}
ProxyEntry ProxyService::add(QString name, const QString &url, int weight) {
    ProxyEntry entry;
    entry.id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    entry.url = normalizeProxy(url);
    if (entry.url.isEmpty())
        throw Error(ErrorCode::InvalidInput, "Proxy URL is required");
    entry.name = name.trimmed().isEmpty() ? redactProxy(entry.url) : name.trimmed();
    entry.weight = std::clamp(weight, 1, 100);
    repository_.update(Document::ProxyPool, [&](QJsonDocument &value) {
        auto document = value.object();
        auto entries = document.value("entries").toArray();
        for (const auto &item : entries)
            if (item.toObject().value("url").toString() == entry.url)
                throw Error(ErrorCode::Conflict, "Proxy already exists");
        entries.append(entry.toJson());
        document["entries"] = entries;
        value.setObject(document);
    });
    return entry;
}
void ProxyService::update(const ProxyEntry &entry) {
    const auto url = normalizeProxy(entry.url);
    if (url.isEmpty())
        throw Error(ErrorCode::InvalidInput, "Proxy URL is required");
    repository_.update(Document::ProxyPool, [&](QJsonDocument &value) {
        auto document = value.object();
        auto entries = document.value("entries").toArray();
        bool found = false;
        for (qsizetype i = 0; i < entries.size(); ++i) {
            const auto previous = entries[i].toObject();
            if (previous.value("id").toString() == entry.id) {
                auto normalized = entry;
                normalized.url = url;
                normalized.weight = std::clamp(entry.weight, 1, 100);
                entries[i] = normalized.toJson();
                found = true;
            } else if (previous.value("url").toString() == url)
                throw Error(ErrorCode::Conflict, "Proxy already exists");
        }
        if (!found)
            throw Error(ErrorCode::NotFound, "Proxy not found");
        document["entries"] = entries;
        value.setObject(document);
    });
}
void ProxyService::remove(const QString &id) {
    repository_.update(Document::ProxyPool, [&](QJsonDocument &value) {
        auto document = value.object();
        QJsonArray entries;
        bool found = false;
        for (const auto &item : document.value("entries").toArray()) {
            if (item.toObject().value("id").toString() == id)
                found = true;
            else
                entries.append(item);
        }
        if (!found)
            throw Error(ErrorCode::NotFound, "Proxy not found");
        document["entries"] = entries;
        value.setObject(document);
    });
}
QJsonObject ProxyService::probe(const QString &id, std::stop_token stop) {
    ProxyEntry entry;
    bool found = false;
    for (const auto &candidate : list())
        if (candidate.id == id) {
            entry = candidate;
            found = true;
            break;
        }
    if (!found)
        throw Error(ErrorCode::NotFound, "Proxy not found");
    QElapsedTimer timer;
    timer.start();
    auto transport = transports_.create({entry.url, "KiroX/2.0", {}});
    HttpRequest request;
    request.timeout = std::chrono::seconds{8};
    request.url = QUrl(
        "http://ip-api.com/json/?fields=status,message,country,countryCode,regionName,city,isp,hosting,mobile,query");
    try {
        const auto response = transport->send(request, stop);
        response.requireSuccess();
        const auto info = response.json().object();
        if (info.value("status").toString() != "success")
            throw Error(ErrorCode::Protocol, info.value("message").toString());
        entry.probe["probeOk"] = true;
        entry.probe["probeIp"] = info.value("query");
        for (auto key : {"country", "countryCode", "city", "isp"}) {
            auto output = QString(key);
            output[0] = output[0].toUpper();
            entry.probe["probe" + output] = info.value(key);
        }
        entry.probe["probeRegion"] = info.value("regionName");
        entry.probe["probeType"] = info.value("hosting").toBool()  ? "datacenter"
                                   : info.value("mobile").toBool() ? "mobile"
                                                                   : "residential";
        entry.probe["probeError"] = "";
    } catch (const Error &error) {
        if (error.code() == ErrorCode::Cancelled)
            throw;
        entry.probe["probeOk"] = false;
        entry.probe["probeError"] = error.message();
    }
    entry.probe["probeAt"] = QDateTime::currentSecsSinceEpoch();
    entry.probe["probeMs"] = timer.elapsed();
    // Merge only probe fields so concurrent name/enabled edits survive the network request.
    repository_.update(Document::ProxyPool, [&](QJsonDocument &value) {
        auto document = value.object();
        auto entries = document.value("entries").toArray();
        for (qsizetype i = 0; i < entries.size(); ++i) {
            auto previous = entries[i].toObject();
            if (previous.value("id").toString() != id)
                continue;
            for (auto it = entry.probe.constBegin(); it != entry.probe.constEnd(); ++it)
                if (it.key().startsWith("probe"))
                    previous[it.key()] = it.value();
            entries[i] = previous;
        }
        document["entries"] = entries;
        value.setObject(document);
    });
    return entry.toJson();
}
QString ProxyService::pick() const {
    const auto entries = list();
    int total = 0;
    for (const auto &entry : entries)
        if (entry.enabled)
            total += entry.weight;
    if (total == 0)
        return {};
    int choice = QRandomGenerator::global()->bounded(total);
    for (const auto &entry : entries)
        if (entry.enabled) {
            choice -= entry.weight;
            if (choice < 0)
                return entry.url;
        }
    return {};
}
} // namespace kirox
