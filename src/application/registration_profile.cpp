#include "kirox/domain/error.hpp"
#include "registration_session.hpp"
#include <QRandomGenerator>
#include <QRegularExpression>

namespace kirox {
void RegistrationSession::signup() {
    const auto data =
        execute({{"stepId", "get-identity-user"},
                 {"workflowStateHandle", workflowHandle_},
                 {"actionId", "SIGNUP"},
                 {"visitorId", visitorId_},
                 {"inputs", QJsonArray{QJsonObject{{"input_type", "UserRequestInput"}, {"username", email_}},
                                       QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                                   {"fingerPrint", fingerprint("signup", "PageSubmit")}}}}});
    const auto handle =
        queryParameter(data.value("redirect").toObject().value("url").toString(), "workflowStateHandle");
    if (!handle.isEmpty())
        workflowHandle_ = handle;
}
void RegistrationSession::signupInit() {
    fingerprint_.resetPage();
    const auto payload = [&](const QString &step, const QString &event) {
        return QJsonObject{{"stepId", step},
                           {"workflowStateHandle", workflowHandle_},
                           {"visitorId", visitorId_},
                           {"inputs", QJsonArray{QJsonObject{{"input_type", "UserRequestInput"}, {"username", email_}},
                                                 QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                                             {"fingerPrint", fingerprint("signup", event)}}}}};
    };
    const auto initial = execute(payload("", "first_load"), true);
    if (initial.value("stepId") != "start")
        throw RegistrationFailure("UNEXPECTED_SIGNUP_STEP");
    const auto data = execute(payload("start", "PageLoad"), true);
    workflowId_ = queryParameter(data.value("redirect").toObject().value("url").toString(), "workflowID");
    if (workflowId_.isEmpty())
        throw RegistrationFailure("MISSING_PROFILE_WORKFLOW");
}
void RegistrationSession::profileInit() {
    const auto digits = [](int count) {
        QString value;
        for (int i = 0; i < count; ++i)
            value.append(QChar('0' + QRandomGenerator::system()->bounded(10)));
        return value;
    };
    ubid_ = "186-" + digits(7) + "-" + digits(6);
    setCookie(endpoints_.profile, "aws-user-profile-ubid", ubid_);
    setCookie(endpoints_.profile, "i18next", "zh-CN");
    profileStarted_ = Clock::now();
    const auto response = send(profileUrl(), "GET", {}, navigation(signinUrl(true)), true);
    if (response.status < 200 || response.status >= 300)
        reject(response, "PROFILE_PAGE_FAILED");
    const auto match = QRegularExpression("(?:^|[/_])app_([a-fA-F0-9]{10,64})\\.min\\.js(?:[\"'?]|$)")
                           .match(QString::fromUtf8(response.body));
    if (match.hasMatch())
        fingerprint_.setBundleHash(match.captured(1));
    fingerprint_.resetPage();
    safe([&] {
        (void)post(endpoints_.profile + "/api/get-config", {}, headers(profileUrl(), endpoints_.profile, true));
    });
    visitor(endpoints_.profile, profileUrl());
}
QJsonObject RegistrationSession::browserData(const QString &event, const QString &pageName, int time,
                                             const QString &fp) const {
    QJsonObject attributes{{"fingerprint", fp},
                           {"eventTimestamp", QDateTime::currentDateTimeUtc().toString("yyyy-MM-ddTHH:mm:ss.zzz'Z'")},
                           {"timeSpentOnPage", QString::number(time)},
                           {"eventType", event},
                           {"ubid", ubid_},
                           {"visitorId", visitorId_}};
    if (!pageName.isEmpty())
        attributes.insert("pageName", pageName);
    return {{"attributes", attributes}, {"cookies", QJsonObject{}}};
}
void RegistrationSession::profileStart() {
    const auto elapsed =
        int(std::chrono::duration_cast<std::chrono::milliseconds>(Clock::now() - profileStarted_).count());
    const auto data =
        object(post(endpoints_.profile + "/api/start",
                    {{"workflowID", workflowId_},
                     {"browserData", browserData("PageLoad", {}, elapsed, fingerprint("profile", "PageLoad"))}},
                    headers(profileUrl(), endpoints_.profile, true)));
    workflowState_ = data.value("workflowState").toString();
    if (workflowState_.isEmpty())
        throw RegistrationFailure("MISSING_PROFILE_STATE");
    emailStarted_ = Clock::now();
    safe([&] { d2cEvent(profileUrl() + "#/signup/start?workflowID=" + workflowId_); });
}
void RegistrationSession::sendOtp() {
    const int time = 5000 + QRandomGenerator::system()->bounded(3001);
    const auto response = post(endpoints_.profile + "/api/send-otp",
                               {{"workflowState", workflowState_},
                                {"email", email_},
                                {"browserData", browserData("PageSubmit", "EMAIL_COLLECTION", time,
                                                            fingerprint("profile", "PageSubmit", time, email_))}},
                               headers(profileUrl(), endpoints_.profile, true));
    if (response.status != 200)
        reject(response, "SEND_OTP_FAILED");
    verificationStarted_ = Clock::now();
    safe([&] { katal(false); });
}
void RegistrationSession::createIdentity(const QString &otp) {
    const auto data = object(post(endpoints_.profile + "/api/create-identity",
                                  {{"workflowState", workflowState_},
                                   {"userData", QJsonObject{{"email", email_}, {"fullName", request_.fullName}}},
                                   {"otpCode", otp},
                                   {"browserData", browserData("EmailVerification", "EMAIL_VERIFICATION", 45000,
                                                               fingerprint("profile", "EmailVerification"))}},
                                  headers(profileUrl(), endpoints_.profile, true)));
    registrationCode_ = data.value("registrationCode").toString();
    signinState_ = data.value("signInState").toString();
    if (registrationCode_.isEmpty() || signinState_.isEmpty())
        throw RegistrationFailure("MISSING_REGISTRATION_CODE");
    safe([&] { katal(true); });
}
void RegistrationSession::setPassword() {
    fingerprint_.resetPage();
    const auto ref = queryUrl(endpoints_.signin + "/platform/" + endpoints_.directory + "/signup",
                              {{"registrationCode", registrationCode_}, {"state", signinState_}})
                         .toString();
    const auto makeFingerprint = [&] { return fingerprint("signup", "PageSubmit", 0, {}, ref); };
    const auto data = execute({{"stepId", ""},
                               {"state", signinState_},
                               {"inputs", QJsonArray{QJsonObject{{"input_type", "UserRegistrationRequestInput"},
                                                                 {"registrationCode", registrationCode_},
                                                                 {"state", signinState_}},
                                                     QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                                                 {"fingerPrint", makeFingerprint()}}}}},
                              true, ref);
    const auto encryption = data.value("workflowResponseData").toObject().value("encryptionContextResponse").toObject();
    const auto jwk = encryption.value("publicKey").toObject();
    if (jwk.value("n").toString().isEmpty())
        throw RegistrationFailure("MISSING_ENCRYPTION_KEY");
    const auto presentation = data.value("presentationContext").toObject();
    auto directory = presentation.value("identityPoolId").toString();
    if (directory.isEmpty())
        directory = endpoints_.directory;
    safe([&] {
        auto h = headers(ref, endpoints_.signin);
        const auto id = uuid();
        h.insert("x-amzn-requestid", id.toLatin1());
        (void)object(post(endpoints_.signin + "/platform/user-event/send-event",
                          {{"inputs", QJsonArray{userEvent(directory, "PAGE_LOAD", "CREDENTIAL_COLLECTION"),
                                                 QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                                             {"fingerPrint", fingerprint("signup", "PageLoad")}}}},
                           {"requestId", id}},
                          h));
    });
    safe([&] {
        fingerprintMetric("IsFingerprintFileLoaded:Success", "1", "AWSSignin:FingerprintMetrics:OnLoad_Password_Page",
                          ref);
    });
    const auto encrypted = PasswordEncryptor(crypto_).encrypt(
        password_, jwk, encryption.value("issuer").toString("signin"),
        encryption.value("audience").toString("AWSPasswordService"), encryption.value("region").toString("us-east-1"));
    auto step = data.value("stepId").toString();
    if (step.isEmpty())
        step = "get-new-password-for-password-creation";
    QJsonObject payload{{"stepId", step},
                        {"workflowStateHandle", workflowHandle_},
                        {"actionId", "SUBMIT"},
                        {"visitorId", visitorId_},
                        {"inputs", QJsonArray{QJsonObject{{"input_type", "PasswordRequestInput"},
                                                          {"password", encrypted},
                                                          {"successfullyEncrypted", "SUCCESSFUL"},
                                                          {"errorLog", QJsonValue::Null}},
                                              userEvent(directory, "PAGE_SUBMIT", "CREDENTIAL_COLLECTION", 5000),
                                              QJsonObject{{"input_type", "UserRequestInput"}, {"username", email_}},
                                              QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                                          {"fingerPrint", makeFingerprint()}}}}};
    const auto source = presentation.value("builderIdSourceDirectory").toString().trimmed();
    if (!source.isEmpty())
        safe([&] {
            if (!QRegularExpression("^[A-Za-z0-9_-]+$").match(source).hasMatch())
                throw Error(ErrorCode::Protocol, "Invalid source directory");
            auto h = navigation(ref);
            h.insert("sec-fetch-dest", "iframe");
            h.insert("sec-fetch-site", "same-origin");
            const auto session = object(send(endpoints_.signin + "/platform/" + source + "/cookieread", "GET", {}, h))
                                     .value("cookieValue")
                                     .toString();
            if (!session.isEmpty())
                payload.insert("builderIdSession", session);
        });
    const auto response = execute(payload, true, ref);
    const auto redirect = response.value("redirect").toObject().value("url").toString();
    if (redirect.isEmpty())
        throw RegistrationFailure("MISSING_PASSWORD_REDIRECT");
    // The password was accepted. Preserve this fact even if finishing SSO fails.
    passwordSet_ = true;
    completeSignup(redirect);
}
void RegistrationSession::completeSignup(const QString &redirect) {
    const auto handle = queryParameter(redirect, "workflowStateHandle"), state = queryParameter(redirect, "state"),
               result = queryParameter(redirect, "workflowResultHandle");
    if (handle.isEmpty() || result.isEmpty())
        throw RegistrationFailure("INVALID_SIGNUP_REDIRECT");
    const auto ref = queryUrl(endpoints_.signin + "/platform/" + endpoints_.directory + "/login",
                              {{"workflowStateHandle", handle}, {"state", state}, {"workflowResultHandle", result}})
                         .toString();
    const auto data =
        execute({{"stepId", ""},
                 {"workflowStateHandle", handle},
                 {"workflowResultHandle", result},
                 {"state", state},
                 {"visitorId", visitorId_},
                 {"inputs", QJsonArray{QJsonObject{{"input_type", "UserRequestInput"}, {"username", email_}},
                                       QJsonObject{{"input_type", "FingerPrintRequestInput"},
                                                   {"fingerPrint", fingerprint("signin", "PageLoad")}}}}},
                false, ref);
    if (data.value("stepId") != "end-of-workflow-success")
        throw RegistrationFailure("SIGNUP_COMPLETION_FAILED");
    const auto url = data.value("redirect").toObject().value("url").toString();
    authCode_ = queryParameter(url, "workflowResultHandle");
    ssoState_ = queryParameter(url, "state");
    wdcToken_ = queryParameter(url, "wdc_csrf_token");
}
} // namespace kirox
