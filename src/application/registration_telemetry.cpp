#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/error.hpp"
#include "registration_session.hpp"
#include <QJsonDocument>
#include <QRandomGenerator>

namespace kirox {
void RegistrationSession::safe(const std::function<void()> &action) {
    try {
        checkCancelled(stop_);
        action();
    } catch (const Error &error) {
        if (error.code() == ErrorCode::Cancelled)
            throw;
    } catch (const RegistrationFailure &) {
    }
}
void RegistrationSession::visitor(const QString &origin, const QString &referer) {
    const auto started = Clock::now();
    const auto id = uuid();
    const auto encoded = [](const QJsonObject &value) {
        return QJsonDocument(value)
            .toJson(QJsonDocument::Compact)
            .toBase64(QByteArray::Base64UrlEncoding | QByteArray::OmitTrailingEquals);
    };
    const auto input = encoded({{"kid", uuid()}, {"alg", "ES256"}}) + '.' +
                       encoded({{"vid", id}, {"iss", "s_p"}, {"exp", QDateTime::currentSecsSinceEpoch() + 3600}});
    const auto signature = crypto_.signEs256(input).signature;
    const auto jwt = QString::fromLatin1(
        input + '.' + signature.toBase64(QByteArray::Base64UrlEncoding | QByteArray::OmitTrailingEquals));
    auto h = headers(referer, origin);
    h.insert("sec-fetch-site", "cross-site");
    h.insert("Accept", "*/*");
    // These cookies intentionally accompany the collector request and are copied
    // to only the configured signin/profile origins, never arbitrary redirects.
    QByteArray cookie;
    for (const auto name : {"awsccc", "awsd2c-token", "awsd2c-token-c"}) {
        const auto value = transport_->cookie(QUrl(origin), name);
        if (!value.isEmpty()) {
            if (!cookie.isEmpty())
                cookie += "; ";
            cookie += QByteArray(name) + '=' + value;
        }
    }
    if (!cookie.isEmpty())
        h.insert("Cookie", cookie);
    const auto data = object(post(endpoints_.visitor, {{"token", jwt}}, h));
    const auto token = data.value("token").toString();
    if (!token.isEmpty()) {
        setCookie(origin, "awsd2c-token", token);
        setCookie(origin, "awsd2c-token-c", token);
    }
    visitorId_ = id;
    visitorDuration_ = std::chrono::duration<double, std::milli>(Clock::now() - started).count();
}
void RegistrationSession::fingerprintMetric(const QString &name, const QString &value, const QString &operation,
                                            const QString &ref) {
    auto h = headers(ref.isEmpty() ? signinUrl() : ref, endpoints_.signin);
    h.insert("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8");
    const auto response = send(endpoints_.signin + "/metrics/fingerprint", "POST",
                               formBody({{"name", name}, {"value", value}, {"operation", operation}}), h, false, false);
    if (response.status != 200)
        reject(response, "FINGERPRINT_METRIC_FAILED");
}
void RegistrationSession::d2cEvent(const QString &pageUrl) {
    const auto origin = pageUrl.startsWith(endpoints_.profile) ? endpoints_.profile : endpoints_.signin;
    auto h = headers(pageUrl, origin);
    h.insert("sec-fetch-site", "cross-site");
    const QJsonObject payload{
        {"batchId", "D2CLogger"},
        {"schemaVersion", "1.0.0"},
        {"batchEvents", QJsonArray{QJsonObject{{"pageURL", pageUrl},
                                               {"eventType", "logEvent"},
                                               {"eventTimestamp", QDateTime::currentMSecsSinceEpoch()},
                                               {"customData", QJsonObject{{"timeTakenToFetchVID",
                                                                           QString::number(visitorDuration_, 'f', 1)},
                                                                          {"logLevel", "info"}}},
                                               {"orgId", "awsme_scode"}}}}};
    const auto response = post(endpoints_.collector, payload, h);
    if (response.status != 200 && response.status != 202)
        reject(response, "COLLECTOR_FAILED");
}
void RegistrationSession::katal(bool verification) {
    struct Metric {
        QString key, schema;
        double value;
    };
    struct Group {
        QString producer;
        QList<Metric> metrics;
    };
    QList<Group> groups;
    const QString counter = "katal.client.metrics.Counter.3", timer = "katal.client.metrics.Timer.2";
    const auto elapsed = [&](Clock::time_point start) {
        return std::max(0.0, std::chrono::duration<double, std::milli>(Clock::now() - start).count());
    };
    if (verification) {
        groups = {{"Steps", {{"SignUpEmailVerificationStep", timer, elapsed(verificationStarted_)}}},
                  {"SignUpContextProvider",
                   {{"HTTPRequest.CreateIdentity.StatusCode.200", counter, 1},
                    {"HTTPRequest.CreateIdentity.StatusCode.2XX", counter, 1},
                    {"HTTPRequest.CreateIdentity.Latency", timer, elapsed(emailStarted_)},
                    {"HTTPRequest.CreateIdentity.Failure", counter, 0}}}};
    } else {
        const auto configMs = elapsed(profileStarted_), otpMs = elapsed(emailStarted_);
        groups = {{"AppContextProvider",
                   {{"HTTPRequest.GetConfig.StatusCode.200", counter, 1},
                    {"HTTPRequest.GetConfig.StatusCode.2XX", counter, 1},
                    {"HTTPRequest.GetConfig.Latency", timer, configMs},
                    {"HTTPRequest.GetConfig.Failure", counter, 0},
                    {"HTTPRequest.GetAppContext.StatusCode.404", counter, 1},
                    {"HTTPRequest.GetAppContext.StatusCode.4XX", counter, 1},
                    {"HTTPRequest.GetAppContext.Latency", timer, configMs},
                    {"HTTPRequest.GetAppContext.Failure", counter, 1}}},
                  {"Steps", {{"StartSignUp", timer, configMs}}},
                  {"SignUpContextProvider",
                   {{"HTTPRequest.SendOTP.StatusCode.200", counter, 1},
                    {"HTTPRequest.SendOTP.StatusCode.2XX", counter, 1},
                    {"HTTPRequest.SendOTP.Latency", timer, otpMs},
                    {"HTTPRequest.SendOTP.Failure", counter, 0}}}};
    }
    QJsonObject dictionary{{"#0", "site"},
                           {"#1", "AWSUserProfileFrontEnd"},
                           {"#2", "serviceName"},
                           {"#3", "actionId"},
                           {"#5", "cloudWatchDimensions"},
                           {"#6", "methodName"},
                           {"#8", "metricKey"},
                           {"#10", "value"},
                           {"#11", "isMonitor"},
                           {"#12", "producerId"},
                           {"#13", "katal"},
                           {"#14", "schemaId"},
                           {"#16", "timestamp"},
                           {"#17", "messageId"}};
    int slot = 18;
    const auto add = [&](const QString &value) {
        const auto key = "#" + QString::number(slot++);
        dictionary.insert(key, value);
        return key;
    };
    QJsonArray events;
    const auto timestamp = QDateTime::currentDateTimeUtc().toString("yyyy-MM-ddTHH:mm:ss.zzz'Z'");
    for (const auto &group : groups) {
        const auto action = add(uuid()), producer = add(group.producer);
        for (const auto &metric : group.metrics) {
            const auto key = add(metric.key), schema = add(metric.schema);
            events.append(QJsonObject{
                {"data", QJsonObject{{"#0", "#1"},
                                     {"#2", "#1"},
                                     {"#3", action},
                                     {"#6", producer},
                                     {"#8", key},
                                     {"#10", metric.value},
                                     {"#11", true},
                                     {"#12", "#13"},
                                     {"#14", schema},
                                     {"#16", timestamp},
                                     {"#17", QString("1-%1-%2")
                                                 .arg(QDateTime::currentMSecsSinceEpoch())
                                                 .arg(quint64(1000000000) +
                                                      QRandomGenerator::system()->generate64() % 9000000000ULL)}}}});
        }
    }
    auto h = headers({}, endpoints_.profile);
    h.insert("Content-Type", "text/plain;charset=UTF-8");
    h.insert("sec-fetch-mode", "no-cors");
    h.insert("sec-fetch-site", "cross-site");
    const auto response = post(endpoints_.metrics, {{"cs", QJsonObject{{"dct", dictionary}}}, {"events", events}}, h);
    if (response.status != 200 && response.status != 202 && response.status != 204)
        reject(response, "KATAL_METRICS_FAILED");
}
} // namespace kirox
