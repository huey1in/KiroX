#include "kirox/application/registration_service.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/infrastructure/native_crypto.hpp"
#include <QCryptographicHash>
#include <QJsonArray>
#include <QJsonDocument>
#include <QTest>
#include <QUrlQuery>

using namespace kirox;
namespace {
HttpResponse json(const QJsonObject &object, int status = 200) {
    return {status, QJsonDocument(object).toJson(QJsonDocument::Compact), {}};
}
class Configuration final : public IFingerprintConfiguration {
  public:
    void warm(const TransportOptions &, const QString &) override {}
    void wait(std::stop_token stop) override {
        checkCancelled(stop);
    }
    std::shared_ptr<const FingerprintCryptoConfig> snapshot() const override {
        return std::make_shared<FingerprintCryptoConfig>();
    }
};
class Cryptography final : public ICryptography {
  public:
    NativeCryptography native;
    QByteArray randomBytes(int count) const override {
        return native.randomBytes(count);
    }
    EphemeralSignature signEs256(const QByteArray &input) const override {
        return native.signEs256(input);
    }
    QByteArray rsaOaep256(const QJsonObject &, const QByteArray &plaintext) const override {
        if (plaintext.size() != 32)
            throw std::runtime_error("Expected a 256-bit content key");
        return QByteArray(256, 'x');
    }
    SealedMessage aes256Gcm(const QByteArray &key, const QByteArray &nonce, const QByteArray &plaintext,
                            const QByteArray &aad) const override {
        return native.aes256Gcm(key, nonce, plaintext, aad);
    }
};
struct Scenario {
    QList<HttpRequest> requests;
    bool mailboxOpened = false, alreadyRegistered = false, captcha = false, rejectPassword = false,
         completeFailure = false, badState = false, suspended = false, telemetryFailure = false;
    int devicePolls = 0, ssoPolls = 0, signupSubmissions = 0;
    QString callback, state, challenge;
    HttpResponse response(const HttpRequest &request) {
        requests.append(request);
        const auto path = request.url.path();
        const auto body = QJsonDocument::fromJson(request.body).object();
        if (path == "/oidc/client/register")
            return json({{"clientId", body.value("clientName") == "Kiro IDE" ? "kiro-client" : "cli-client"},
                         {"clientSecret", "synthetic-client-secret"}});
        if (path == "/oidc/device_authorization")
            return json({{"deviceCode", "device"}, {"userCode", "user-code"}});
        if (path == "/portal/login") {
            // The device redirect contains user_code; the later SSO redirect does not.
            const bool device =
                QUrlQuery(request.url).queryItemValue("redirect_url", QUrl::FullyDecoded).contains("user_code");
            return json({{"redirectUrl", QString("https://fixture.test/signin/login?workflowStateHandle=%1")
                                             .arg(device ? "initial" : "sso-wh")},
                         {"csrfToken", "csrf"}});
        }
        if (path == "/visitor") {
            const auto parts = body.value("token").toString().split('.');
            if (parts.size() != 3 ||
                QByteArray::fromBase64(parts[2].toLatin1(), QByteArray::Base64UrlEncoding).size() != 64)
                throw std::runtime_error("Invalid visitor JWT");
            return json({{"token", "synthetic-visitor-token"}});
        }
        if (path == "/signin/metrics/fingerprint" || path == "/signin/platform/user-event/send-event" ||
            path == "/collector" || path == "/metrics")
            return telemetryFailure ? json({}, 503) : json({});
        if (path == "/profile/")
            return {200, "<html><script src=\"/dist/main/app_0123456789abcdef.min.js\"></script></html>", {}};
        if (path == "/profile/api/get-config")
            return json({});
        if (path == "/profile/api/start")
            return json({{"workflowState", "profile-state"}});
        if (path == "/profile/api/send-otp") {
            if (!mailboxOpened)
                throw std::runtime_error("Mailbox baseline must precede OTP");
            return json({});
        }
        if (path == "/profile/api/create-identity") {
            if (body.value("otpCode") != "654321")
                throw std::runtime_error("Unexpected OTP");
            return json({{"registrationCode", "registration"}, {"signInState", "signin-state"}});
        }
        if (path == "/signin/platform/d-9067642ac7/signup/api/execute") {
            if (body.contains("state"))
                return json(
                    {{"workflowStateHandle", "password-wh"},
                     {"stepId", "dynamic-password-step"},
                     {"presentationContext",
                      QJsonObject{{"identityPoolId", "identity-pool"}, {"builderIdSourceDirectory", "source-dir"}}},
                     {"workflowResponseData",
                      QJsonObject{
                          {"encryptionContextResponse",
                           QJsonObject{{"publicKey",
                                        QJsonObject{{"n", "synthetic-modulus"}, {"e", "AQAB"}, {"kid", "key"}}}}}}}});
            if (body.value("actionId") == "SUBMIT") {
                ++signupSubmissions;
                const auto inputs = body.value("inputs").toArray();
                if (body.value("stepId") != "dynamic-password-step" ||
                    body.value("builderIdSession") != "builder-session" ||
                    inputs.at(0).toObject().value("password").toString().split('.').size() != 5 ||
                    inputs.at(1).toObject().value("directoryId") != "identity-pool")
                    throw std::runtime_error("Password contract mismatch");
                if (rejectPassword)
                    return json({{"message", QJsonObject{{"errorCode", "AUTHENTICATION_FAILED"},
                                                         {"text", "synthetic-secret"},
                                                         {"requestId", "request-123"}}},
                                 {"captchaResponse", QJsonObject{{"captchaToken", "secret-captcha-token"}}}},
                                400);
                return json(
                    {{"redirect",
                      QJsonObject{{"url",
                                   "https://fixture.test/signin/"
                                   "login?workflowStateHandle=finished&state=state&workflowResultHandle=result"}}}});
            }
            if (body.value("stepId") == "start")
                return json({{"redirect",
                              QJsonObject{{"url", "https://fixture.test/profile/?workflowID=profile-id#/signup"}}}});
            return json({{"stepId", "start"}, {"workflowStateHandle", "signup-wh"}});
        }
        if (path == "/signin/platform/source-dir/cookieread")
            return json({{"cookieValue", "builder-session"}});
        if (path == "/signin/platform/d-9067642ac7/api/execute") {
            if (body.value("actionId") == "SUBMIT") {
                if (captcha)
                    return json({{"captchaResponse", QJsonObject{{"captchaToken", "secret-captcha-token"}}}}, 400);
                return json({{"workflowStateHandle", "initial"}}, alreadyRegistered ? 200 : 400);
            }
            if (body.value("actionId") == "SIGNUP")
                return json(
                    {{"redirect",
                      QJsonObject{{"url", "https://fixture.test/signin/signup?workflowStateHandle=signup-wh"}}}});
            if (body.contains("workflowResultHandle") || body.value("workflowStateHandle") == "sso-wh") {
                if (completeFailure && body.contains("workflowResultHandle"))
                    return json({{"stepId", "failure"}});
                return json(
                    {{"stepId", "end-of-workflow-success"},
                     {"redirect",
                      QJsonObject{{"url",
                                   "https://fixture.test/view/"
                                   "start?workflowResultHandle=auth-code&state=sso-state&wdc_csrf_token=wdc"}}}});
            }
            return json({{"stepId", body.value("stepId") == "start" ? "get-identity-user" : "start"},
                         {"workflowStateHandle", "initial"}});
        }
        if (path == "/view/start/")
            return {200, "<html></html>", {}};
        if (path == "/portal/auth/sso-token") {
            if (++ssoPolls == 1)
                return json({{"errorMessage", "not authorized"}}, 401);
            return json({{"token", "synthetic-sso-token"}});
        }
        if (path == "/oidc/device_authorization/accept_user_code")
            return json({{"deviceContext", QJsonObject{{"device", "context"}}}});
        if (path == "/oidc/device_authorization/associate_token")
            return body.contains("authorizationResumptionContext")
                       ? json({{"location", "https://fixture.test/oidc/resume2"}})
                       : json({});
        if (path == "/oidc/token") {
            if (body.value("grantType") == "urn:ietf:params:oauth:grant-type:device_code") {
                if (++devicePolls == 1)
                    return json({{"error", "authorization_pending"}}, 400);
                return json({{"accessToken", "synthetic-aws-access"}, {"refreshToken", "synthetic-aws-refresh"}});
            }
            if (body.value("grantType") == "authorization_code") {
                const auto verifier = body.value("codeVerifier").toString().toLatin1();
                const auto actual =
                    QString::fromLatin1(QCryptographicHash::hash(verifier, QCryptographicHash::Sha256)
                                            .toBase64(QByteArray::Base64UrlEncoding | QByteArray::OmitTrailingEquals));
                if (actual != challenge || body.value("redirectUri") != callback)
                    throw std::runtime_error("PKCE contract mismatch");
                return json({{"accessToken", "synthetic-kiro-access"},
                             {"refreshToken", "synthetic-kiro-refresh"},
                             {"expiresIn", 3600}});
            }
            return json({{"accessToken", "synthetic-refreshed-access"}});
        }
        if (path == "/oidc/authorize") {
            const QUrlQuery query(request.url);
            callback = query.queryItemValue("redirect_uri", QUrl::FullyDecoded);
            state = query.queryItemValue("state");
            challenge = query.queryItemValue("code_challenge");
            return {302, {}, {{"location", "https://fixture.test/oidc/auth?orchestrator_id=orchestrator"}}};
        }
        if (path == "/oidc/authentication_result")
            return json({{"location", "https://fixture.test/oidc/resume1"}});
        if (path == "/oidc/resume1")
            return {
                302, {}, {{"location", "https://fixture.test/oidc/consent?authorizationResumptionContext=context"}}};
        if (path == "/oidc/resume2")
            return {302,
                    {},
                    {{"location", (callback + "?code=kiro-code&state=" + (badState ? "bad-state" : state)).toUtf8()}}};
        if (path == "/usage")
            return suspended ? json({}, 403)
                             : json({{"userInfo", QJsonObject{{"email", "fixture@example.test"}}},
                                     {"usageBreakdownList",
                                      QJsonArray{QJsonObject{
                                          {"resourceType", "CREDIT"},
                                          {"usageLimitWithPrecision", 50.5},
                                          {"currentUsageWithPrecision", 1.25},
                                          {"freeTrialInfo", QJsonObject{{"freeTrialStatus", "ACTIVE"},
                                                                        {"usageLimitWithPrecision", 100.5},
                                                                        {"currentUsageWithPrecision", 2.5}}}}}}});
        if (path == "/models" || path == "/refresh")
            return json({});
        throw std::runtime_error(("Unexpected endpoint " + path).toStdString());
    }
};
class Transport final : public ITransport {
  public:
    explicit Transport(Scenario &scenario) : scenario(scenario) {}
    HttpResponse send(const HttpRequest &request, std::stop_token stop) override {
        checkCancelled(stop);
        return scenario.response(request);
    }
    void setCookie(const QUrl &, const QByteArray &name, const QByteArray &value) override {
        cookies.insert(name, value);
    }
    QByteArray cookie(const QUrl &, const QByteArray &name) const override {
        return cookies.value(name);
    }
    Scenario &scenario;
    QMap<QByteArray, QByteArray> cookies;
};
class TransportFactory final : public ITransportFactory {
  public:
    explicit TransportFactory(Scenario &scenario) : scenario(scenario) {}
    std::unique_ptr<ITransport> create(const TransportOptions &) const override {
        return std::make_unique<Transport>(scenario);
    }
    Scenario &scenario;
};
class Mailbox final : public IMailboxSession {
  public:
    explicit Mailbox(Scenario &scenario) : scenario(scenario) {}
    QString open(std::stop_token stop) override {
        checkCancelled(stop);
        scenario.mailboxOpened = true;
        return "fixture@example.test";
    }
    QString poll(std::stop_token stop, std::chrono::milliseconds) override {
        checkCancelled(stop);
        return "654321";
    }
    Scenario &scenario;
};
class Mailboxes final : public IMailboxFactory {
  public:
    explicit Mailboxes(Scenario &scenario) : scenario(scenario) {}
    std::unique_ptr<IMailboxSession> create(const MailboxRequest &) const override {
        return std::make_unique<Mailbox>(scenario);
    }
    QJsonObject inspect(MailboxKind, const QJsonObject &, const TransportOptions &, std::stop_token) const override {
        return {};
    }
    Scenario &scenario;
};
RegistrationEndpoints endpoints() {
    RegistrationEndpoints result;
    result.oidc = "https://fixture.test/oidc";
    result.signin = "https://fixture.test/signin";
    result.profile = "https://fixture.test/profile";
    result.view = "https://fixture.test/view";
    result.portal = "https://fixture.test/portal";
    result.visitor = "https://fixture.test/visitor";
    result.collector = "https://fixture.test/collector";
    result.metrics = "https://fixture.test/metrics";
    result.usage = "https://fixture.test/usage";
    result.models = "https://fixture.test/models";
    result.refresh = "https://fixture.test/refresh";
    return result;
}
RegistrationResult run(Scenario &scenario, std::stop_token stop = {}, const RegistrationProgress &progress = {}) {
    TransportFactory transport(scenario);
    Mailboxes mailboxes(scenario);
    Cryptography crypto;
    Configuration config;
    RegistrationRequest request;
    request.identity = BrowserIdentity::generate();
    request.password = "Fixture!Password7";
    const RegistrationTiming timing{std::chrono::milliseconds(0), std::chrono::milliseconds(0),
                                    std::chrono::milliseconds(0), std::chrono::milliseconds(1)};
    return RegistrationService(transport, mailboxes, crypto, config, endpoints(), timing).run(request, stop, progress);
}
} // namespace
class RegistrationTests : public QObject {
    Q_OBJECT
  private slots:
    void completeProtocolProducesTokensAndUsage() {
        Scenario scenario;
        QStringList steps;
        const auto result = run(scenario, {}, [&](const QString &step) { steps.append(step); });
        QVERIFY2(result.successful(), qPrintable(QJsonDocument(result.values).toJson()));
        QVERIFY(result.passwordSet());
        QCOMPARE(scenario.signupSubmissions, 1);
        QCOMPARE(scenario.devicePolls, 2);
        QCOMPARE(scenario.ssoPolls, 2);
        QCOMPARE(result.values.value("kiro_tokens").toObject().value("refreshToken").toString(),
                 QString("synthetic-kiro-refresh"));
        const auto verification = result.values.value("verify").toObject();
        QCOMPARE(verification.value("credit_limit").toDouble(), 151.0);
        QCOMPARE(verification.value("credit_used").toDouble(), 3.75);
        QCOMPARE(steps.first(), QString("OIDC"));
        QCOMPARE(steps.last(), QString("Verify"));
        QCOMPARE(steps.size(), qsizetype(19));
        int batches = 0;
        for (const auto &request : scenario.requests)
            if (request.url.path() == "/metrics") {
                ++batches;
                const auto payload = QJsonDocument::fromJson(request.body).object(),
                           dictionary = payload.value("cs").toObject().value("dct").toObject();
                for (const auto event : payload.value("events").toArray()) {
                    const auto data = event.toObject().value("data").toObject();
                    for (const auto key : {"#0", "#2", "#3", "#6", "#8", "#12", "#14"})
                        QVERIFY(dictionary.contains(data.value(key).toString()));
                    QCOMPARE(dictionary.value(data.value("#12").toString()).toString(), QString("katal"));
                    const auto metric = dictionary.value(data.value("#8").toString()).toString();
                    if (metric.startsWith("HTTPRequest.SendOTP") || metric.startsWith("HTTPRequest.CreateIdentity"))
                        QCOMPARE(dictionary.value(data.value("#6").toString()).toString(),
                                 QString("SignUpContextProvider"));
                }
            }
        QCOMPARE(batches, 2);
    }
    void rejectedPasswordDoesNotConsumeMailboxOrLeakSecrets() {
        Scenario scenario;
        scenario.rejectPassword = true;
        const auto result = run(scenario);
        QVERIFY(!result.successful());
        QVERIFY(!result.passwordSet());
        QVERIFY(result.risk());
        QCOMPARE(result.values.value("errorCode").toString(), QString("AUTHENTICATION_FAILED"));
        QCOMPARE(result.values.value("requestId").toString(), QString("request-123"));
        const auto bytes = QJsonDocument(result.values).toJson();
        QVERIFY(!bytes.contains("secret"));
        QVERIFY(!bytes.contains("654321"));
        QVERIFY(!bytes.contains("Fixture!"));
    }
    void laterFailureKeepsPasswordConsumption() {
        Scenario scenario;
        scenario.completeFailure = true;
        const auto result = run(scenario);
        QVERIFY(!result.successful());
        QVERIFY(result.passwordSet());
        QCOMPARE(result.values.value("errorCode").toString(), QString("SIGNUP_COMPLETION_FAILED"));
    }
    void cancellationAfterPasswordKeepsConsumption() {
        Scenario scenario;
        std::stop_source stop;
        const auto result = run(scenario, stop.get_token(), [&](const QString &step) {
            if (step == "SSOWorkflow")
                stop.request_stop();
        });
        QVERIFY(!result.successful());
        QVERIFY(result.passwordSet());
        QCOMPARE(result.values.value("errorCode").toString(), QString("CANCELLED"));
    }
    void existingEmailAndCaptchaNeverSubmitPassword_data() {
        QTest::addColumn<bool>("captcha");
        QTest::newRow("existing") << false;
        QTest::newRow("captcha") << true;
    }
    void existingEmailAndCaptchaNeverSubmitPassword() {
        QFETCH(bool, captcha);
        Scenario scenario;
        scenario.captcha = captcha;
        scenario.alreadyRegistered = !captcha;
        const auto result = run(scenario);
        QVERIFY(!result.successful());
        QVERIFY(!result.passwordSet());
        QCOMPARE(result.risk(), captcha);
        QCOMPARE(scenario.signupSubmissions, 0);
    }
    void oauthStateMismatchCannotProduceTokens() {
        Scenario scenario;
        scenario.badState = true;
        const auto result = run(scenario);
        QVERIFY(!result.successful());
        QVERIFY(result.passwordSet());
        QCOMPARE(result.values.value("errorCode").toString(), QString("OAUTH_STATE_MISMATCH"));
        for (const auto &request : scenario.requests)
            QVERIFY(QJsonDocument::fromJson(request.body).object().value("grantType") != "authorization_code");
    }
    void telemetryFailureDoesNotInterruptRegistration() {
        Scenario scenario;
        scenario.telemetryFailure = true;
        QVERIFY(run(scenario).successful());
    }
    void suspendedAccountIsRiskAndConsumed() {
        Scenario scenario;
        scenario.suspended = true;
        const auto result = run(scenario);
        QVERIFY(!result.successful());
        QVERIFY(result.risk());
        QVERIFY(result.passwordSet());
    }
};
QTEST_GUILESS_MAIN(RegistrationTests)
#include "registration_tests.moc"
