#include "kirox/application/settings_service.hpp"
namespace kirox {
Settings SettingsService::get() const {
    const auto document = repository_.read(Document::Settings).object();
    auto runtime = document.value("runtime").toObject();
    if (!runtime.contains("language"))
        runtime["language"] = document.value("language");
    return Settings::fromJson(runtime);
}
Settings SettingsService::patch(const QJsonObject &changes) {
    Settings saved;
    repository_.update(Document::Settings, [&](QJsonDocument &value) {
        auto document = value.object();
        auto runtime = document.value("runtime").toObject();
        for (auto it = changes.constBegin(); it != changes.constEnd(); ++it)
            runtime[it.key()] = it.value();
        saved = Settings::fromJson(runtime);
        const auto normalized = saved.toJson();
        // Keep extension settings from newer versions, and patch only the requested fields.
        for (auto it = normalized.constBegin(); it != normalized.constEnd(); ++it)
            runtime[it.key()] = it.value();
        document["runtime"] = runtime;
        document["language"] = saved.language;
        document["schemaVersion"] = 4;
        value.setObject(document);
    });
    return saved;
}
} // namespace kirox
