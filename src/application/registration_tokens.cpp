#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/error.hpp"
#include "registration_session.hpp"
#include <QCryptographicHash>
#include <QRandomGenerator>

namespace kirox {
void RegistrationSession::ssoWorkflow() {
    const auto url = queryUrl(endpoints_.portal + "/login",
                              {{"directory_id", "view"}, {"redirect_url", endpoints_.view + "/start/#/"}})
                         .toString();
    const auto data = object(send(url, "GET", {}, headers(endpoints_.view + "/", endpoints_.view)));
    const auto csrf = data.value("csrfToken").toString();
    if (!csrf.isEmpty()) {
        loginCsrf_ = csrf;
        setCookie(endpoints_.portal, "loginCsrfToken", csrf);
    }
    auto handle = queryParameter(data.value("redirectUrl").toString(), "workflowStateHandle");
    if (handle.isEmpty())
        throw RegistrationFailure("MISSING_SSO_WORKFLOW");
    const auto ref =
        queryUrl(endpoints_.signin + "/platform/" + endpoints_.directory + "/login", {{"workflowStateHandle", handle}})
            .toString();
    const auto payload = [&](const QString &step) {
        return QJsonObject{{"stepId", step},
                           {"workflowStateHandle", handle},
                           {"inputs", QJsonArray{QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                                             {"fingerPrint", fingerprint("signin", "PageLoad")}}}}};
    };
    auto response = execute(payload(""), false, ref);
    handle = response.value("workflowStateHandle").toString(handle);
    if (response.value("stepId") == "start")
        response = execute(payload("start"), false, ref);
    if (response.value("stepId") != "end-of-workflow-success")
        throw RegistrationFailure("SSO_WORKFLOW_FAILED");
    const auto redirect = response.value("redirect").toObject().value("url").toString();
    authCode_ = queryParameter(redirect, "workflowResultHandle");
    ssoState_ = queryParameter(redirect, "state");
    wdcToken_ = queryParameter(redirect, "wdc_csrf_token");
    if (authCode_.isEmpty())
        throw RegistrationFailure("MISSING_SSO_AUTH_CODE");
    const auto start =
        queryUrl(endpoints_.view + "/start/",
                 {{"state", ssoState_}, {"workflowResultHandle", authCode_}, {"wdc_csrf_token", wdcToken_}})
            .toString();
    safe([&] { (void)send(start, "GET", {}, navigation(endpoints_.signin + "/"), true); });
}
QJsonObject RegistrationSession::ssoToken() {
    if (loginCsrf_.isEmpty())
        loginCsrf_ = QString::fromUtf8(transport_->cookie(QUrl(endpoints_.portal), "loginCsrfToken"));
    if (loginCsrf_.isEmpty())
        throw RegistrationFailure("MISSING_SSO_CSRF_TOKEN");
    auto h = headers(endpoints_.view + "/", endpoints_.view);
    h.insert("Content-Type", "application/x-www-form-urlencoded");
    h.insert("x-amz-sso-csrf-token", loginCsrf_.toUtf8());
    h.insert("sec-fetch-site", "cross-site");
    for (int attempt = 0; attempt < 5 && ssoToken_.isEmpty(); ++attempt) {
        const auto response = send(endpoints_.portal + "/auth/sso-token", "POST",
                                   formBody({{"authCode", authCode_}, {"state", ssoState_}, {"orgId", "view"}}), h);
        const auto data = object(response, false);
        ssoToken_ = data.value("token").toString();
        if (!ssoToken_.isEmpty())
            break;
        if (!data.value("errorMessage").toString().contains("not authorized", Qt::CaseInsensitive))
            reject(response, "SSO_TOKEN_FAILED");
        if (attempt < 4)
            interruptibleWait(stop_, timing_.ssoDelay);
    }
    if (ssoToken_.isEmpty())
        throw RegistrationFailure("SSO_TOKEN_NOT_READY");
    const auto accepted = object(post(endpoints_.oidc + "/device_authorization/accept_user_code",
                                      {{"userCode", userCode_}, {"userSessionId", ssoToken_}}));
    if (accepted.value("deviceContext").isNull() || accepted.value("deviceContext").isUndefined())
        throw RegistrationFailure("MISSING_DEVICE_CONTEXT");
    (void)object(post(endpoints_.oidc + "/device_authorization/associate_token",
                      {{"deviceContext", accepted.value("deviceContext")}, {"userSessionId", ssoToken_}}));
    for (int i = 0; i < 30; ++i) {
        const auto response =
            post(endpoints_.oidc + "/token", {{"clientId", clientId_},
                                              {"clientSecret", clientSecret_},
                                              {"deviceCode", deviceCode_},
                                              {"grantType", "urn:ietf:params:oauth:grant-type:device_code"}});
        const auto data = object(response, false);
        if (response.status == 200 && !data.value("accessToken").toString().isEmpty())
            return data;
        const auto error = data.value("error").toString();
        if (error != "authorization_pending" && error != "slow_down")
            reject(response, "DEVICE_TOKEN_FAILED");
        if (i < 29)
            interruptibleWait(stop_, timing_.tokenDelay +
                                         (error == "slow_down" ? timing_.ssoDelay : std::chrono::milliseconds{}));
    }
    throw Error(ErrorCode::Timeout, "Device token polling timed out");
}
QString RegistrationSession::kiroAuthorize() {
    const QJsonArray scopes{"codewhisperer:completions", "codewhisperer:analysis", "codewhisperer:conversations",
                            "codewhisperer:transformations", "codewhisperer:taskassist"};
    const auto registration = object(
        post(endpoints_.oidc + "/client/register", {{"clientName", "Kiro IDE"},
                                                    {"clientType", "public"},
                                                    {"scopes", scopes},
                                                    {"redirectUris", QJsonArray{"http://127.0.0.1/oauth/callback"}},
                                                    {"grantTypes", QJsonArray{"authorization_code", "refresh_token"}},
                                                    {"issuerUrl", endpoints_.view + "/start"}}));
    kiroClientId_ = registration.value("clientId").toString();
    kiroClientSecret_ = registration.value("clientSecret").toString();
    if (kiroClientId_.isEmpty() || kiroClientSecret_.isEmpty())
        throw RegistrationFailure("KIRO_CLIENT_REGISTRATION_FAILED");
    verifier_ = QString::fromLatin1(
        crypto_.randomBytes(32).toBase64(QByteArray::Base64UrlEncoding | QByteArray::OmitTrailingEquals));
    const auto challenge =
        QString::fromLatin1(QCryptographicHash::hash(verifier_.toLatin1(), QCryptographicHash::Sha256)
                                .toBase64(QByteArray::Base64UrlEncoding | QByteArray::OmitTrailingEquals));
    kiroState_ = uuid();
    redirectUri_ =
        QString("http://127.0.0.1:%1/oauth/callback").arg(49152 + QRandomGenerator::system()->bounded(16384));
    QStringList scopeNames;
    for (const auto scope : scopes)
        scopeNames.append(scope.toString());
    const auto authorize = queryUrl(endpoints_.oidc + "/authorize", {{"response_type", "code"},
                                                                     {"client_id", kiroClientId_},
                                                                     {"redirect_uri", redirectUri_},
                                                                     {"scopes", scopeNames.join(',')},
                                                                     {"state", kiroState_},
                                                                     {"code_challenge", challenge},
                                                                     {"code_challenge_method", "S256"}})
                               .toString();
    const auto redirected = [&](const QString &url) {
        const QUrl target(url);
        // Resume locations belong to this OIDC origin; callback is parsed, never fetched.
        const QUrl origin(endpoints_.oidc);
        if (!target.isValid() || target.scheme() != origin.scheme() || target.host() != origin.host() ||
            target.port() != origin.port() || !target.userInfo().isEmpty())
            throw RegistrationFailure("INVALID_OAUTH_REDIRECT");
        const auto response = send(url, "GET", {}, navigation());
        if (response.status != 302)
            reject(response, "EXPECTED_OAUTH_REDIRECT");
        return QString::fromUtf8(response.headers.value("location"));
    };
    const auto orchestrator = queryParameter(redirected(authorize), "orchestrator_id");
    if (orchestrator.isEmpty())
        throw RegistrationFailure("MISSING_ORCHESTRATOR");
    auto h = headers(endpoints_.view + "/", endpoints_.view);
    h.insert("sec-fetch-site", "cross-site");
    h.insert("x-amz-sso_bearer_token", ssoToken_.toUtf8());
    h.insert("x-amz-sso-bearer-token", ssoToken_.toUtf8());
    const auto result =
        object(post(endpoints_.oidc + "/authentication_result", {{"orchestrator_id", orchestrator}}, h));
    const auto firstResume = result.value("location").toString();
    if (firstResume.isEmpty())
        throw RegistrationFailure("MISSING_AUTHENTICATION_RESUME");
    const auto context = queryParameter(redirected(firstResume), "authorizationResumptionContext");
    if (context.isEmpty())
        throw RegistrationFailure("MISSING_RESUMPTION_CONTEXT");
    h.remove("x-amz-sso_bearer_token");
    h.remove("x-amz-sso-bearer-token");
    const auto consent = object(post(endpoints_.oidc + "/device_authorization/associate_token",
                                     {{"authorizationResumptionContext", context}, {"userSessionId", ssoToken_}}, h));
    const auto secondResume = consent.value("location").toString();
    if (secondResume.isEmpty())
        throw RegistrationFailure("MISSING_CONSENT_RESUME");
    const auto callback = redirected(secondResume);
    const QUrl returned(callback), expected(redirectUri_);
    if (returned.scheme() != expected.scheme() || returned.host() != expected.host() ||
        returned.port() != expected.port() || returned.path() != expected.path() ||
        queryParameter(callback, "state") != kiroState_)
        throw RegistrationFailure("OAUTH_STATE_MISMATCH");
    const auto code = queryParameter(callback, "code");
    if (code.isEmpty())
        throw RegistrationFailure("MISSING_AUTHORIZATION_CODE");
    return code;
}
QJsonObject RegistrationSession::kiroExchange(const QString &code) {
    const auto data = object(post(endpoints_.oidc + "/token", {{"clientId", kiroClientId_},
                                                               {"clientSecret", kiroClientSecret_},
                                                               {"grantType", "authorization_code"},
                                                               {"code", code},
                                                               {"redirectUri", redirectUri_},
                                                               {"codeVerifier", verifier_}}));
    if (data.value("accessToken").toString().isEmpty())
        throw RegistrationFailure("TOKEN_EXCHANGE_FAILED");
    return data;
}
QJsonObject RegistrationSession::verify(const QJsonObject &tokens) {
    try {
        const QJsonObject refreshBody{{"clientId", clientId_},
                                      {"clientSecret", clientSecret_},
                                      {"refreshToken", tokens.value("refreshToken")},
                                      {"grantType", "refresh_token"}};
        const auto refreshed = post(endpoints_.oidc + "/token", refreshBody);
        if (refreshed.status != 200)
            return {{"alive", false}, {"error", "refresh failed"}};
        const auto access = object(refreshed).value("accessToken").toString();
        if (access.isEmpty())
            return {{"alive", false}, {"error", "missing access token"}};
        const Headers h{{"Accept", "application/json"},
                        {"Authorization", "Bearer " + access.toUtf8()},
                        {"User-Agent", "aws-sdk-js/1.0.18 ua/2.1 os/windows lang/js md/nodejs#20.16.0 "
                                       "api/codewhispererstreaming#1.0.18 m/E KiroIDE-0.6.18"}};
        const auto usage = send(endpoints_.usage, "GET", {}, h);
        if (usage.status == 403)
            return {{"alive", false}, {"suspended", true}};
        if (usage.status != 200)
            return {{"alive", false}, {"error", "usage query failed"}};
        const auto models = send(endpoints_.models, "GET", {}, h);
        if (models.status == 403)
            return {{"alive", false}, {"suspended", true}};
        const auto kiro = post(endpoints_.refresh, refreshBody);
        if (kiro.status == 403)
            return {{"alive", false}, {"suspended", true}};
        const auto data = object(usage);
        double limit = 0, used = 0;
        for (const auto entry : data.value("usageBreakdownList").toArray()) {
            const auto item = entry.toObject();
            if (item.value("resourceType") != "CREDIT" && item.value("displayName") != "Credits")
                continue;
            const auto number = [](const QJsonObject &value, const char *precise, const char *fallback) {
                const auto amount = value.value(precise).toDouble();
                return amount == 0 ? value.value(fallback).toDouble() : amount;
            };
            limit = number(item, "usageLimitWithPrecision", "usageLimit");
            used = number(item, "currentUsageWithPrecision", "currentUsage");
            const auto trial = item.value("freeTrialInfo").toObject();
            if (trial.value("freeTrialStatus") == "ACTIVE") {
                limit += trial.value("usageLimitWithPrecision").toDouble();
                used += trial.value("currentUsageWithPrecision").toDouble();
            }
            break;
        }
        return {{"alive", true},
                {"email", data.value("userInfo").toObject().value("email")},
                {"credit_used", used},
                {"credit_limit", limit}};
    } catch (const Error &error) {
        if (error.code() == ErrorCode::Cancelled)
            throw;
        return {{"alive", false}, {"error", "verification failed"}};
    } catch (const RegistrationFailure &) {
        return {{"alive", false}, {"error", "verification failed"}};
    }
}
} // namespace kirox
