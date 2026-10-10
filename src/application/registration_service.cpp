#include "kirox/application/mailbox_service.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/error.hpp"
#include "registration_session.hpp"
#include <QJsonDocument>
#include <QLocale>
#include <QRandomGenerator>
#include <QRegularExpression>
#include <QUrlQuery>
#include <QUuid>

namespace kirox {
QByteArray formBody(const QList<QPair<QString, QString>> &parameters) {
    QByteArray result;
    for (const auto &[key, value] : parameters) {
        if (!result.isEmpty())
            result += '&';
        result += QUrl::toPercentEncoding(key) + '=' + QUrl::toPercentEncoding(value);
    }
    return result;
}
QUrl queryUrl(const QString &url, const QList<QPair<QString, QString>> &parameters) {
    QUrl result(url);
    result.setQuery(QString::fromLatin1(formBody(parameters)));
    return result;
}
QString queryParameter(const QString &url, const QString &key) {
    return QUrlQuery(QUrl(url)).queryItemValue(key, QUrl::FullyDecoded);
}
QString uuid() {
    return QUuid::createUuid().toString(QUuid::WithoutBraces);
}
RegistrationService::RegistrationService(const ITransportFactory &transport, const IMailboxFactory &mailboxes,
                                         const ICryptography &crypto, IFingerprintConfiguration &fingerprints,
                                         RegistrationEndpoints endpoints, RegistrationTiming timing)
    : transport_(transport), mailboxes_(mailboxes), crypto_(crypto), fingerprints_(fingerprints),
      endpoints_(std::move(endpoints)), timing_(timing) {}
RegistrationResult RegistrationService::run(const RegistrationRequest &request, std::stop_token stop,
                                            const RegistrationProgress &progress) const {
    if (!request.identity.valid() || request.networkRetries < 0 || request.networkRetries > 4 ||
        request.otpTimeoutSeconds <= 0 || request.otpTimeoutSeconds > 3600 || request.fullName.trimmed().isEmpty())
        throw Error(ErrorCode::InvalidInput, "Invalid registration configuration");
    return RegistrationSession(request, transport_, mailboxes_, crypto_, fingerprints_, endpoints_, timing_, stop,
                               progress)
        .run();
}
RegistrationSession::RegistrationSession(const RegistrationRequest &request, const ITransportFactory &transport,
                                         const IMailboxFactory &mailboxes, const ICryptography &crypto,
                                         IFingerprintConfiguration &fingerprints,
                                         const RegistrationEndpoints &endpoints, const RegistrationTiming &timing,
                                         std::stop_token stop, RegistrationProgress progress)
    : request_(request), crypto_(crypto), fingerprints_(fingerprints), endpoints_(endpoints), timing_(timing),
      stop_(stop), progress_(std::move(progress)), fingerprint_(request.identity) {
    auto options = request.transport;
    options.userAgent = request.identity.userAgent();
    options.browserProfile = "chrome" + request.identity.majorVersion();
    transport_ = transport.create(options);
    mailbox_ = mailboxes.create(request.mailbox);
    fingerprints_.warm(options, request.identity.securityUserAgent());
    password_ = request.password;
    if (password_.isEmpty()) {
        QByteArray value;
        const QList<QPair<QByteArray, int>> groups{
            {"ABCDEFGHIJKLMNOPQRSTUVWXYZ", 3}, {"abcdefghijklmnopqrstuvwxyz", 6}, {"0123456789", 3}, {"!@#$%^&*", 2}};
        for (const auto &[alphabet, count] : groups)
            for (int i = 0; i < count; ++i)
                value += alphabet.at(QRandomGenerator::system()->bounded(int(alphabet.size())));
        for (int i = int(value.size()) - 1; i > 0; --i)
            std::swap(value[i], value[QRandomGenerator::system()->bounded(i + 1)]);
        password_ = QString::fromLatin1(value);
    }
    visitorId_ = uuid();
}
void RegistrationSession::advance(const QString &step) {
    checkCancelled(stop_);
    step_ = step;
    if (progress_)
        progress_(step);
}
HttpResponse RegistrationSession::send(const QString &url, const QByteArray &method, const QByteArray &body,
                                       Headers requestHeaders, bool follow, bool retry) {
    HttpRequest request;
    request.url = QUrl(url);
    request.method = method;
    request.body = body;
    request.headers = std::move(requestHeaders);
    request.followRedirects = follow;
    request.timeout = std::chrono::seconds(60);
    for (int attempt = 0;; ++attempt) {
        checkCancelled(stop_);
        try {
            return transport_->send(request, stop_);
        } catch (const Error &error) {
            if (!retry || attempt >= request_.networkRetries ||
                (error.code() != ErrorCode::Network && error.code() != ErrorCode::Timeout))
                throw;
            interruptibleWait(stop_, timing_.retryDelay);
        }
    }
}
HttpResponse RegistrationSession::post(const QString &url, const QJsonObject &body, Headers requestHeaders) {
    if (requestHeaders.isEmpty())
        requestHeaders = {{"Content-Type", "application/json"}};
    return send(url, "POST", QJsonDocument(body).toJson(QJsonDocument::Compact), std::move(requestHeaders));
}
[[noreturn]] void RegistrationSession::reject(const HttpResponse &response, const QString &fallback) const {
    const auto data = QJsonDocument::fromJson(response.body).object(), message = data.value("message").toObject();
    auto code = message.value("errorCode").toString();
    if (code.isEmpty())
        code = data.value("error").toString();
    const bool captcha = !data.value("captchaResponse").toObject().isEmpty();
    if (captcha && code.isEmpty())
        code = "CAPTCHA_REQUIRED";
    if (!QRegularExpression("^[A-Za-z0-9_.:-]{1,100}$").match(code).hasMatch())
        code = fallback;
    auto requestId = message.value("requestId").toString(data.value("requestId").toString());
    if (!QRegularExpression("^[A-Za-z0-9_-]{1,128}$").match(requestId).hasMatch())
        requestId.clear();
    const bool risk = captcha || response.status == 403 || response.status == 429 ||
                      (step_ == "SendOTP" && response.status == 400) || code.contains("BLOCKED", Qt::CaseInsensitive) ||
                      code.contains("SUSPENDED", Qt::CaseInsensitive);
    throw RegistrationFailure(code, requestId, risk);
}
QJsonObject RegistrationSession::object(const HttpResponse &response, bool success) const {
    if (success && (response.status < 200 || response.status >= 300))
        reject(response, "HTTP_" + QString::number(response.status));
    const auto document = response.json();
    if (!document.isObject())
        throw Error(ErrorCode::Protocol, "Expected a JSON object");
    return document.object();
}
Headers RegistrationSession::headers(const QString &referer, const QString &origin, bool profile) const {
    Headers result{{"Accept", profile ? "*/*" : "application/json, text/plain, */*"},
                   {"Content-Type", profile ? "application/json;charset=UTF-8" : "application/json"},
                   {"Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8"},
                   {"User-Agent", request_.identity.userAgent().toUtf8()},
                   {"sec-ch-ua", request_.identity.securityUserAgent().toUtf8()},
                   {"sec-ch-ua-mobile", "?0"},
                   {"sec-ch-ua-platform", "\"Windows\""},
                   {"sec-fetch-dest", "empty"},
                   {"sec-fetch-mode", "cors"},
                   {"sec-fetch-site", "same-origin"},
                   {"priority", "u=1, i"}};
    if (!referer.isEmpty())
        result.insert("Referer", referer.toUtf8());
    if (!origin.isEmpty())
        result.insert("Origin", origin.toUtf8());
    return result;
}
Headers RegistrationSession::navigation(const QString &referer) const {
    auto result = headers(referer);
    result.remove("Content-Type");
    result.insert("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8");
    result.insert("sec-fetch-dest", "document");
    result.insert("sec-fetch-mode", "navigate");
    result.insert("sec-fetch-site", "cross-site");
    result.insert("Upgrade-Insecure-Requests", "1");
    return result;
}
QString RegistrationSession::signinUrl(bool signup) const {
    return queryUrl(endpoints_.signin + "/platform/" + endpoints_.directory + (signup ? "/signup" : "/login"),
                    {{"workflowStateHandle", workflowHandle_}})
        .toString();
}
QString RegistrationSession::executeUrl(bool signup) const {
    return endpoints_.signin + "/platform/" + endpoints_.directory + (signup ? "/signup" : "") + "/api/execute";
}
QString RegistrationSession::profileUrl() const {
    return queryUrl(endpoints_.profile + "/", {{"workflowID", workflowId_}}).toString();
}
void RegistrationSession::setCookie(const QString &origin, const QByteArray &name, const QString &value) {
    if (!value.isEmpty())
        transport_->setCookie(QUrl(origin), name, value.toUtf8());
}
QString RegistrationSession::fingerprint(const QString &page, const QString &event, int time, const QString &email,
                                         const QString &location) {
    auto url = location;
    if (url.isEmpty()) {
        if (page == "profile")
            url = profileUrl() + (event == "PageSubmit" ? "#/signup/enter-email" : "#/signup/start");
        else
            url = signinUrl(page == "signup");
    }
    const auto ref = page == "profile"
                         ? signinUrl(true)
                         : (page == "signup" && !location.isEmpty() ? profileUrl() : endpoints_.view + "/");
    const auto configuration = fingerprints_.snapshot();
    return encryptFingerprint(fingerprint_.serialize({url, ref, page, event, email, time}, configuration->version),
                              *configuration);
}
QJsonObject RegistrationSession::execute(QJsonObject body, bool signup, const QString &ref, bool check) {
    const auto id = uuid();
    body.insert("requestId", id);
    auto h = headers(ref.isEmpty() ? signinUrl(signup) : ref, endpoints_.signin);
    h.insert("x-amzn-requestid", id.toLatin1());
    h.insert("x-amz-date",
             QLocale::c().toString(QDateTime::currentDateTimeUtc(), "ddd, dd MMM yyyy HH:mm:ss 'GMT'").toLatin1());
    const auto data = object(post(executeUrl(signup), body, h), check);
    const auto handle = data.value("workflowStateHandle").toString();
    if (!handle.isEmpty())
        workflowHandle_ = handle;
    return data;
}
void RegistrationSession::oidc() {
    const auto data = object(post(
        endpoints_.oidc + "/client/register",
        {{"clientName", "Amazon Q Developer for command line"},
         {"clientType", "public"},
         {"scopes", QJsonArray{"codewhisperer:completions", "codewhisperer:analysis", "codewhisperer:conversations",
                               "codewhisperer:transformations", "codewhisperer:taskassist"}}}));
    clientId_ = data.value("clientId").toString();
    clientSecret_ = data.value("clientSecret").toString();
    if (clientId_.isEmpty() || clientSecret_.isEmpty())
        throw RegistrationFailure("CLIENT_REGISTRATION_FAILED");
}
void RegistrationSession::device() {
    const auto data = object(
        post(endpoints_.oidc + "/device_authorization",
             {{"clientId", clientId_}, {"clientSecret", clientSecret_}, {"startUrl", endpoints_.view + "/start"}}));
    deviceCode_ = data.value("deviceCode").toString();
    userCode_ = data.value("userCode").toString();
    if (deviceCode_.isEmpty() || userCode_.isEmpty())
        throw RegistrationFailure("DEVICE_AUTHORIZATION_FAILED");
}
void RegistrationSession::mailbox() {
    email_ = mailbox_->open(stop_);
    if (!email_.contains('@'))
        throw Error(ErrorCode::Protocol, "Mailbox returned an invalid address");
}
void RegistrationSession::portal() {
    fingerprints_.wait(stop_);
    const auto awsccc = QString::fromLatin1(
        QJsonDocument(QJsonObject{{"e", 1}, {"p", 1}, {"f", 1}, {"a", 1}, {"i", uuid()}, {"v", "1"}})
            .toJson(QJsonDocument::Compact)
            .toBase64());
    for (const auto &origin : {endpoints_.portal, endpoints_.signin, endpoints_.profile})
        setCookie(origin, "awsccc", awsccc);
    const auto redirect =
        endpoints_.view + "/start/#/device?user_code=" + QString::fromLatin1(QUrl::toPercentEncoding(userCode_));
    const auto data = object(
        send(queryUrl(endpoints_.portal + "/login", {{"directory_id", "view"}, {"redirect_url", redirect}}).toString(),
             "GET", {}, headers(endpoints_.view + "/", endpoints_.view)));
    workflowHandle_ = queryParameter(data.value("redirectUrl").toString(), "workflowStateHandle");
    loginCsrf_ = data.value("csrfToken").toString();
    setCookie(endpoints_.portal, "loginCsrfToken", loginCsrf_);
    if (workflowHandle_.isEmpty())
        throw RegistrationFailure("MISSING_WORKFLOW_HANDLE");
    visitor(endpoints_.signin, signinUrl());
}
void RegistrationSession::initialize() {
    auto data = execute({{"stepId", ""},
                         {"workflowStateHandle", workflowHandle_},
                         {"inputs", QJsonArray{QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                                           {"fingerPrint", fingerprint("signin", "first_load")}}}}});
    safe([&] {
        fingerprintMetric("IsFingerprintGenerated:Success", fingerprint("signin", "first_load"),
                          "AWSSignin:FingerprintMetrics:start");
    });
    if (data.value("stepId") == "start")
        data = execute({{"stepId", "start"},
                        {"workflowStateHandle", workflowHandle_},
                        {"inputs", QJsonArray{QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                                          {"fingerPrint", fingerprint("signin", "PageLoad")}}}}});
    safe([&] {
        fingerprintMetric("IsFingerprintFileLoaded:Success", "1", "AWSSignin:FingerprintMetrics:OnLoad_Username_Page");
    });
    safe([&] { d2cEvent(signinUrl()); });
}
QJsonObject RegistrationSession::userEvent(const QString &directory, const QString &event, const QString &page,
                                           int time) const {
    QJsonObject detail{{"input_type", "UserEvent"}, {"eventType", event}, {"pageName", page}};
    if (time > 0)
        detail.insert("timeSpentOnPage", time);
    return {{"input_type", "UserEventRequestInput"},
            {"directoryId", directory},
            {"userName", email_},
            {"userEvents", QJsonArray{detail}}};
}
void RegistrationSession::submitEmail() {
    QJsonObject payload{
        {"stepId", "get-identity-user"},
        {"workflowStateHandle", workflowHandle_},
        {"actionId", "SUBMIT"},
        {"visitorId", visitorId_},
        {"requestId", uuid()},
        {"inputs", QJsonArray{QJsonObject{{"input_type", "UserRequestInput"}, {"username", email_}},
                              QJsonObject{{"input_type", "ApplicationTypeRequestInput"},
                                          {"applicationType", "SSO_INDIVIDUAL_ID"}},
                              userEvent(endpoints_.directory, "PAGE_SUBMIT", "IDENTIFICATION", 5000),
                              QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                          {"fingerPrint", fingerprint("signin", "PageSubmit", 0, email_)}}}}};
    const auto response = post(executeUrl(), payload, headers(signinUrl(), endpoints_.signin));
    const auto data = object(response, false);
    const auto handle = data.value("workflowStateHandle").toString();
    if (!handle.isEmpty())
        workflowHandle_ = handle;
    if (response.status == 200)
        throw RegistrationFailure("EMAIL_ALREADY_REGISTERED");
    if (response.status != 400 || !data.value("captchaResponse").toObject().isEmpty())
        reject(response, "EMAIL_SUBMISSION_FAILED");
}
QJsonObject RegistrationSession::retryTokens(const std::function<QJsonObject()> &action) {
    for (int attempt = 0;; ++attempt) {
        checkCancelled(stop_);
        try {
            return action();
        } catch (const Error &e) {
            if (attempt >= 2 || e.code() == ErrorCode::Cancelled || e.code() == ErrorCode::InvalidInput)
                throw;
        } catch (const RegistrationFailure &e) {
            if (attempt >= 2 || e.risk)
                throw;
        }
        interruptibleWait(stop_, timing_.tokenDelay);
    }
}
RegistrationResult RegistrationSession::run() {
    try {
        advance("OIDC");
        oidc();
        advance("Device");
        device();
        advance("Email");
        mailbox();
        advance("Portal");
        portal();
        advance("WorkflowInit");
        initialize();
        advance("SubmitEmail");
        submitEmail();
        advance("Signup");
        signup();
        advance("SignupInit");
        signupInit();
        advance("ProfileInit");
        profileInit();
        advance("ProfileStart");
        profileStart();
        advance("SendOTP");
        sendOtp();
        advance("GetOTP");
        const auto otp = waitForVerificationCode(*mailbox_, std::chrono::seconds(request_.otpTimeoutSeconds),
                                                 timing_.mailboxInterval, stop_);
        advance("CreateIdentity");
        createIdentity(otp);
        advance("SetPassword");
        setPassword();
        advance("SSOWorkflow");
        ssoWorkflow();
        interruptibleWait(stop_, timing_.tokenDelay);
        advance("SSOToken");
        const auto aws = retryTokens([&] { return ssoToken(); });
        advance("KiroAuthorize");
        const auto code = retryTokens([&] { return QJsonObject{{"code", kiroAuthorize()}}; }).value("code").toString();
        advance("KiroExchange");
        const auto kiro = retryTokens([&] { return kiroExchange(code); });
        advance("Verify");
        const auto verification = verify(aws);
        checkCancelled(stop_);
        if (verification.value("suspended").toBool())
            throw RegistrationFailure("SUSPENDED", {}, true);
        return {{{"email", email_},
                 {"password", password_},
                 {"status", "success"},
                 {"passwordSet", true},
                 {"client_id", clientId_},
                 {"client_secret", clientSecret_},
                 {"device_code", deviceCode_},
                 {"aws_token", aws},
                 {"kiro_tokens", kiro},
                 {"verify", verification}}};
    } catch (const RegistrationFailure &error) {
        return {{{"status", "failed"},
                 {"email", email_},
                 {"passwordSet", passwordSet_},
                 {"step", step_},
                 {"errorCode", error.code},
                 {"requestId", error.requestId},
                 {"risk", error.risk},
                 {"error", step_ + ": " + error.code}}};
    } catch (const Error &error) {
        const QString code = error.code() == ErrorCode::Cancelled
                                 ? "CANCELLED"
                                 : (error.code() == ErrorCode::Timeout
                                        ? "TIMEOUT"
                                        : (error.code() == ErrorCode::Network ? "NETWORK_ERROR" : "PROTOCOL_ERROR"));
        return {{{"status", "failed"},
                 {"email", email_},
                 {"passwordSet", passwordSet_},
                 {"step", step_},
                 {"errorCode", code},
                 {"error", step_ + ": " + code}}};
    }
}
} // namespace kirox
