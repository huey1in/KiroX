#pragma once
#include "kirox/application/password_encryptor.hpp"
#include "kirox/application/registration_service.hpp"
#include "kirox/domain/fingerprint.hpp"
#include <QJsonArray>
#include <chrono>

namespace kirox {
using Headers = QMap<QByteArray, QByteArray>;
QUrl queryUrl(const QString &url, const QList<QPair<QString, QString>> &parameters);
QByteArray formBody(const QList<QPair<QString, QString>> &parameters);
QString queryParameter(const QString &url, const QString &key);
QString uuid();
class RegistrationFailure final : public std::exception {
  public:
    RegistrationFailure(QString code, QString requestId = {}, bool risk = false)
        : code(std::move(code)), requestId(std::move(requestId)), risk(risk) {}
    const char *what() const noexcept override {
        return "Registration service rejected the request";
    }
    QString code, requestId;
    bool risk;
};
class RegistrationSession {
  public:
    RegistrationSession(const RegistrationRequest &request, const ITransportFactory &transport,
                        const IMailboxFactory &mailboxes, const ICryptography &crypto,
                        IFingerprintConfiguration &fingerprints, const RegistrationEndpoints &endpoints,
                        const RegistrationTiming &timing, std::stop_token stop, RegistrationProgress progress);
    RegistrationResult run();

  private:
    friend class RegistrationTelemetry;
    using Clock = std::chrono::steady_clock;
    HttpResponse send(const QString &url, const QByteArray &method, const QByteArray &body, Headers headers = {},
                      bool follow = false, bool retry = true);
    HttpResponse post(const QString &url, const QJsonObject &body, Headers headers = {});
    QJsonObject object(const HttpResponse &response, bool success = true) const;
    [[noreturn]] void reject(const HttpResponse &response, const QString &fallback) const;
    Headers headers(const QString &referer = {}, const QString &origin = {}, bool profile = false) const;
    Headers navigation(const QString &referer = {}) const;
    QString fingerprint(const QString &page, const QString &event, int time = 0, const QString &email = {},
                        const QString &location = {});
    QString signinUrl(bool signup = false) const;
    QString executeUrl(bool signup = false) const;
    QString profileUrl() const;
    void advance(const QString &step);
    void setCookie(const QString &origin, const QByteArray &name, const QString &value);
    QJsonObject execute(QJsonObject body, bool signup = false, const QString &ref = {}, bool check = true);
    void oidc();
    void device();
    void mailbox();
    void portal();
    void initialize();
    void submitEmail();
    void signup();
    void signupInit();
    void profileInit();
    void profileStart();
    void sendOtp();
    void createIdentity(const QString &otp);
    void setPassword();
    void completeSignup(const QString &redirect);
    void ssoWorkflow();
    QJsonObject ssoToken();
    QString kiroAuthorize();
    QJsonObject kiroExchange(const QString &code);
    QJsonObject verify(const QJsonObject &tokens);
    QJsonObject browserData(const QString &event, const QString &pageName, int time, const QString &fp) const;
    QJsonObject userEvent(const QString &directory, const QString &event, const QString &page, int time = 0) const;
    void visitor(const QString &origin, const QString &referer);
    void fingerprintMetric(const QString &name, const QString &value, const QString &operation,
                           const QString &ref = {});
    void d2cEvent(const QString &pageUrl);
    void katal(bool verification);
    void safe(const std::function<void()> &action);
    QJsonObject retryTokens(const std::function<QJsonObject()> &action);
    const RegistrationRequest &request_;
    const ICryptography &crypto_;
    IFingerprintConfiguration &fingerprints_;
    const RegistrationEndpoints &endpoints_;
    const RegistrationTiming &timing_;
    std::stop_token stop_;
    RegistrationProgress progress_;
    FingerprintContext fingerprint_;
    std::unique_ptr<ITransport> transport_;
    std::unique_ptr<IMailboxSession> mailbox_;
    QString step_, email_, password_, visitorId_, ubid_, clientId_, clientSecret_, deviceCode_, userCode_,
        workflowHandle_, workflowId_, workflowState_, registrationCode_, signinState_;
    QString authCode_, ssoState_, wdcToken_, ssoToken_, loginCsrf_, kiroClientId_, kiroClientSecret_, verifier_,
        kiroState_, redirectUri_;
    bool passwordSet_ = false;
    Clock::time_point profileStarted_{}, emailStarted_{}, verificationStarted_{};
    double visitorDuration_ = 0;
};
} // namespace kirox
