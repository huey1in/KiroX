#include "kirox/domain/fingerprint.hpp"
#include "kirox/domain/error.hpp"
#include "kirox/domain/fingerprint_crypto.hpp"
#include <QJsonArray>
#include <QJsonDocument>
#include <QRandomGenerator>
#include <QRegularExpression>
#include <algorithm>

namespace kirox {
namespace {
int random(int low, int high) {
    return low + QRandomGenerator::system()->bounded(high - low + 1);
}
QString identifier(const QString &prefix, qint64 seconds) {
    return QString("%1-%2-%3:%4")
        .arg(prefix)
        .arg(random(0, 9999999), 7, 10, QChar('0'))
        .arg(random(0, 9999999), 7, 10, QChar('0'))
        .arg(seconds);
}
QJsonArray samples(int count, int low, int high) {
    QJsonArray values;
    for (int i = 0; i < count; ++i)
        values.append(random(low, high));
    return values;
}
QJsonObject performance(qint64 now) {
    const qint64 end = now - random(500, 1500), duration = random(2000, 4000), base = end - duration;
    const qint64 dns = random(2, 9), connect = random(300, 599), response = connect + random(200, 599),
                 interactive = duration - random(5, 15);
    return {{"connectStart", base + dns + random(1, 3)},
            {"secureConnectionStart", base + dns + random(3, 7)},
            {"unloadEventEnd", 0},
            {"domainLookupStart", base + dns},
            {"domainLookupEnd", base + dns + random(0, 1)},
            {"responseStart", base + response},
            {"connectEnd", base + connect},
            {"responseEnd", base + response + random(0, 4)},
            {"requestStart", base + connect},
            {"domLoading", base + response + random(2, 6)},
            {"redirectStart", 0},
            {"loadEventEnd", end},
            {"domComplete", end},
            {"navigationStart", base},
            {"loadEventStart", end},
            {"domContentLoadedEventEnd", end},
            {"unloadEventStart", 0},
            {"redirectEnd", 0},
            {"domInteractive", base + interactive},
            {"fetchStart", base + dns},
            {"domContentLoadedEventStart", base + interactive + random(0, 2)}};
}
bool pageLoad(const FingerprintEvent &event) {
    return event.event == "PageLoad" || event.event == "first_load";
}
QJsonObject metrics(const FingerprintEvent &event) {
    QJsonObject result;
    for (const auto key : {"el", "script", "h", "batt", "perf", "auto", "tz", "fp2", "lsubid", "browser",
                           "capabilities", "gpu", "dnt", "math", "tts", "input", "canvas", "captchainput", "pow"})
        result.insert(key, 0);
    if (event.event == "first_load" || (event.event == "PageLoad" && event.page == "profile")) {
        result.insert("script", 1);
        if (event.page == "profile") {
            result.insert("batt", random(2, 3));
            result.insert("capabilities", random(3, 4));
            result.insert("input", random(10, 14));
            result.insert("canvas", random(3, 5));
        } else if (event.page == "signup") {
            result.insert("batt", 1);
            result.insert("capabilities", 1);
            result.insert("gpu", random(3, 4));
            result.insert("dnt", 1);
        } else
            result.insert("gpu", random(4, 6));
    } else
        result.insert("perf", random(0, 2));
    return result;
}
QJsonObject interaction(const FingerprintEvent &event) {
    int clicks = 0, keys = 0, pastes = 0;
    QJsonArray intervals, positions, cycles, mouse;
    if (!pageLoad(event)) {
        if (event.page == "signin" && (event.event == "PageSubmit" || event.event == "SignupStart")) {
            clicks = 2;
            keys = 2;
            pastes = 1;
        } else if (event.page == "signup" && (event.event == "PageSubmit" || event.event == "SignupStart")) {
            clicks = 5;
            keys = 16;
            pastes = 1;
        } else if (event.page == "profile" && event.event == "EmailVerification") {
            clicks = 1;
            keys = 2;
            pastes = 1;
        } else if (event.page == "profile" && event.event == "PageSubmit") {
            clicks = 2;
            keys = 2;
            pastes = 1;
        } else {
            clicks = random(1, 10);
            keys = random(3, 22);
        }
        intervals = samples(std::max(1, keys / 3) + random(0, std::max(1, keys / 2 - keys / 3 + 1) - 1), 30, 1529);
        cycles = samples(std::max(2, keys / 2) + random(0, std::max(1, keys * 2 / 3 - keys / 2 + 1) - 1), 10, 809);
        for (int i = 0; i < clicks; ++i)
            positions.append(QString("%1,%2").arg(random(50, 1549)).arg(random(50, 849)));
        mouse = samples(clicks, 20, 319);
    }
    return {{"clicks", clicks},
            {"touches", 0},
            {"keyPresses", keys},
            {"cuts", 0},
            {"copies", 0},
            {"pastes", pastes},
            {"keyPressTimeIntervals", intervals},
            {"mouseClickPositions", positions},
            {"keyCycles", cycles},
            {"mouseCycles", mouse},
            {"touchCycles", QJsonArray{}}};
}
QJsonObject form(const FingerprintEvent &event, qint64 now, const QJsonObject &actions) {
    if (event.page != "profile" || pageLoad(event) || event.email.isEmpty())
        return {};
    const auto name = QString("formField%1-%2-%3").arg(random(1, 99)).arg(now - random(10, 50)).arg(random(1000, 9999));
    const auto count = std::max(3, int(event.email.size()) / 3 + random(0, 9) - 3);
    const auto field =
        QJsonObject{{"clicks", 1},
                    {"touches", 0},
                    {"keyPresses", actions.value("keyPresses")},
                    {"cuts", 0},
                    {"copies", 0},
                    {"pastes", actions.value("pastes")},
                    {"keyPressTimeIntervals", samples(std::min(count - 1, 10), 30, 1529)},
                    {"mouseClickPositions", QJsonArray{QString("%1.5,%2.5").arg(random(100, 250)).arg(random(10, 20))}},
                    {"keyCycles", samples(std::min(count, 10), 10, 509)},
                    {"mouseCycles", QJsonArray{random(80, 150)}},
                    {"touchCycles", QJsonArray{}},
                    {"width", 180},
                    {"height", 32},
                    {"totalFocusTime", random(400, 2499)},
                    {"checksum", QString("%1").arg(crc32(event.email.toUtf8()), 8, 16, QChar('0')).toUpper()},
                    {"autocomplete", false},
                    {"prefilled", false}};
    return {{name, field}};
}
class OrderedJson {
  public:
    void append(const QString &key, const QJsonValue &value) {
        if (bytes_.size() > 1)
            bytes_ += ',';
        bytes_ += encoded(key) + ':' + encoded(value);
    }
    QByteArray finish() {
        return bytes_ + '}';
    }

  private:
    static QByteArray encoded(const QJsonValue &value) {
        auto bytes = QJsonDocument(QJsonArray{value}).toJson(QJsonDocument::Compact);
        return bytes.mid(1, bytes.size() - 2);
    }
    QByteArray bytes_{"{"};
};
} // namespace
FingerprintContext::FingerprintContext(BrowserIdentity identity) : identity_(std::move(identity)) {
    if (!identity_.valid())
        throw Error(ErrorCode::InvalidInput, "Invalid browser identity");
    signinId_ = identifier(identity_.json().value("LsubidPrefixSignin").toString(), QDateTime::currentSecsSinceEpoch());
}
void FingerprintContext::resetPage() {
    timing_ = {};
    start_.reset();
}
void FingerprintContext::setBundleHash(const QString &hash) {
    if (!QRegularExpression("^[a-fA-F0-9]{10,64}$").match(hash).hasMatch())
        throw Error(ErrorCode::InvalidInput, "Invalid application bundle hash");
    auto values = identity_.json();
    values.insert("WebpackHash", hash);
    identity_ = BrowserIdentity(values);
}
QByteArray FingerprintContext::serialize(const FingerprintEvent &event, const QString &version, qint64 now) {
    if (!QStringList{"signin", "profile", "signup"}.contains(event.page) || event.timeOnPage < 0 || version.isEmpty())
        throw Error(ErrorCode::InvalidInput, "Invalid fingerprint event");
    if (timing_.isEmpty())
        timing_ = performance(now);
    if (event.page == "profile" && profileId_.isEmpty())
        profileId_ = identifier(identity_.json().value("LsubidPrefixProfile").toString(),
                                timing_.value("loadEventEnd").toInteger() / 1000);
    auto timing = timing_;
    if (event.event == "first_load")
        for (const auto key : {"loadEventEnd", "loadEventStart", "domComplete"})
            timing.insert(key, 0);
    const qint64 end = now + random(0, 50);
    qint64 start;
    if (!pageLoad(event) && event.timeOnPage > 0)
        start = end - event.timeOnPage;
    else {
        if (!start_)
            start_ = now - (event.event == "first_load"
                                ? random(500, 1000)
                                : (event.event == "PageLoad" && event.page == "profile" ? random(30, 80) : 0));
        start = *start_;
    }
    const auto &data = identity_.json();
    const auto screen = data.value("Screen").toObject();
    const auto screenText = QString("%1-%2-%3-%4-*-*-*")
                                .arg(screen.value("Width").toInt())
                                .arg(screen.value("Height").toInt())
                                .arg(screen.value("AvailHeight").toInt())
                                .arg(screen.value("ColorDepth").toInt());
    QStringList pluginNames;
    for (const auto plugin : data.value("Plugins").toArray())
        pluginNames.append(plugin.toObject().value("name").toString());
    const auto pluginText = pluginNames.join(' ') + " ||" + screenText;
    QJsonArray scripts;
    int elapsed = 1, history = 5;
    if (event.page == "profile") {
        scripts.append("/dist/main/app_" + data.value("WebpackHash").toString() + ".min.js");
        history = pageLoad(event) ? 6 : 8;
    } else {
        scripts.append("/assets/js/app.js");
        if (event.page == "signup") {
            elapsed = 0;
            history = 9;
        } else
            scripts.append("https://d35uxhjf90umnp.cloudfront.net/index.js");
    }
    const auto actions = interaction(event);
    OrderedJson result;
    result.append("metrics", metrics(event));
    result.append("start", start);
    result.append("interaction", actions);
    result.append("scripts", QJsonObject{{"dynamicUrls", scripts},
                                         {"inlineHashes", QJsonArray{}},
                                         {"elapsed", elapsed},
                                         {"dynamicUrlCount", scripts.size()},
                                         {"inlineHashesCount", 0}});
    result.append("history", QJsonObject{{"length", history}});
    result.append("battery", QJsonObject{});
    result.append("performance", QJsonObject{{"timing", timing}});
    result.append("automation",
                  QJsonObject{{"wd", QJsonObject{{"properties", QJsonObject{{"document", QJsonArray{}},
                                                                            {"window", QJsonArray{}},
                                                                            {"navigator", QJsonArray{}}}}}},
                              {"phantom", QJsonObject{{"properties", QJsonObject{{"window", QJsonArray{}}}}}}});
    result.append("end", end);
    result.append("timeZone", data.value("TimezoneHours"));
    result.append("flashVersion", QJsonValue::Null);
    result.append("plugins", pluginText);
    result.append("dupedPlugins", pluginText);
    result.append("screenInfo", screenText);
    result.append("lsUbid", event.page == "profile" ? profileId_ : signinId_);
    result.append("referrer", event.referrer);
    result.append("userAgent", identity_.userAgent());
    result.append("deviceMemory", data.value("DeviceMemory"));
    result.append("hardwareConcurrency", data.value("HardwareConcurrency"));
    result.append("platform", data.value("Platform"));
    result.append("location", event.location);
    result.append("webDriver", false);
    result.append("capabilities", QJsonObject{{"css", QJsonObject{{"textShadow", 1},
                                                                  {"WebkitTextStroke", 1},
                                                                  {"boxShadow", 1},
                                                                  {"borderRadius", 1},
                                                                  {"borderImage", 1},
                                                                  {"opacity", 1},
                                                                  {"transform", 1},
                                                                  {"transition", 1}}},
                                              {"js", QJsonObject{{"audio", true},
                                                                 {"geolocation", true},
                                                                 {"localStorage", "supported"},
                                                                 {"touch", false},
                                                                 {"video", true},
                                                                 {"webWorker", true}}},
                                              {"elapsed", 0}});
    result.append("gpu", QJsonObject{{"vendor", data.value("GPUVendor")},
                                     {"model", data.value("GPUModel")},
                                     {"extensions", data.value("WebGLExts")}});
    result.append("dnt", QJsonValue::Null);
    result.append(
        "math",
        QJsonObject{{"tan", data.value("MathTan")}, {"sin", data.value("MathSin")}, {"cos", data.value("MathCos")}});
    if (event.page == "profile")
        result.append("timeToSubmit",
                      pageLoad(event) ? random(1, 5) : (event.timeOnPage > 0 ? event.timeOnPage : random(2000, 6000)));
    result.append("form", form(event, now, actions));
    result.append("canvas", QJsonObject{{"hash", data.value("CanvasHash")},
                                        {"emailHash", QJsonValue::Null},
                                        {"histogramBins", data.value("HistogramBase")}});
    result.append("token", QJsonObject{{"isCompatible", event.page != "signin"}, {"pageHasCaptcha", 0}});
    result.append("auth", QJsonObject{{"form", QJsonObject{{"method", "get"}}}});
    result.append("errors", QJsonArray{});
    result.append("version", version);
    return result.finish();
}
} // namespace kirox
