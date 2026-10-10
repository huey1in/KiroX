#include "kirox/application/update_service.hpp"
#include "kirox/domain/error.hpp"
#include "kirox/domain/version.hpp"
namespace kirox {
QJsonObject UpdateService::check(const QString &current, std::stop_token stop) const {
    auto transport = transport_.create({{}, "KiroX/" + current, {}});
    HttpRequest request;
    request.url = endpoint_;
    request.timeout = std::chrono::seconds(10);
    request.headers = {{"Accept", "application/vnd.github+json"}, {"User-Agent", ("KiroX/" + current).toLatin1()}};
    const auto response = transport->send(request, stop);
    const QString releases = "https://github.com/huey1in/KiroX/releases/latest";
    if (response.status == 404)
        return {{"hasUpdate", false},
                {"currentVersion", current},
                {"latestVersion", current},
                {"releaseURL", releases},
                {"noRelease", true}};
    response.requireSuccess();
    const auto data = response.json().object();
    const auto version = data.value("tag_name").toString();
    if (version.isEmpty())
        throw Error(ErrorCode::Protocol, "Release response is missing its version");
    QUrl url(data.value("html_url").toString());
    if (url.scheme() != "https" || url.host() != "github.com" ||
        !url.path().toLower().startsWith("/huey1in/kirox/releases/"))
        url = QUrl(releases);
    return {{"hasUpdate", newerVersion(version, current)},
            {"currentVersion", current},
            {"latestVersion", version},
            {"releaseURL", url.toString()},
            {"releaseDate", data.value("published_at").toString().left(10)},
            {"changelog", data.value("body")}};
}
} // namespace kirox
