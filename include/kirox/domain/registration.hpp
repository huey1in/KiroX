#pragma once
#include "kirox/domain/browser_identity.hpp"
#include "kirox/ports/mailbox.hpp"
#include <QJsonObject>
#include <chrono>

namespace kirox {
struct RegistrationEndpoints {
    QString oidc = "https://oidc.us-east-1.amazonaws.com";
    QString signin = "https://us-east-1.signin.aws";
    QString profile = "https://profile.aws.amazon.com";
    QString view = "https://view.awsapps.com";
    QString portal = "https://portal.sso.us-east-1.amazonaws.com";
    QString visitor = "https://vs.aws.amazon.com/token";
    QString collector = "https://d2c.aws.amazon.com/csds/collector/v1/events/batch";
    QString metrics = "https://unagi-na.amazon.com/1/events/com.amazon.eel.katal.metrics.core.nexus";
    QString usage = "https://q.us-east-1.amazonaws.com/"
                    "getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST&isEmailRequired=true";
    QString models = "https://q.us-east-1.amazonaws.com/ListAvailableModels?origin=AI_EDITOR";
    QString refresh = "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken";
    QString directory = "d-9067642ac7";
};
struct RegistrationRequest {
    BrowserIdentity identity;
    MailboxRequest mailbox;
    TransportOptions transport;
    QString password, fullName = "Test User";
    int otpTimeoutSeconds = 120;
    int networkRetries = 2;
};
struct RegistrationResult {
    QJsonObject values;
    [[nodiscard]] bool successful() const {
        return values.value("status") == "success";
    }
    [[nodiscard]] bool passwordSet() const {
        return values.value("passwordSet").toBool();
    }
    [[nodiscard]] bool risk() const {
        return values.value("risk").toBool();
    }
};
// Injectable delays keep local protocol fixtures deterministic and fast.
struct RegistrationTiming {
    std::chrono::milliseconds tokenDelay{2000};
    std::chrono::milliseconds ssoDelay{3000};
    std::chrono::milliseconds retryDelay{1500};
    std::chrono::milliseconds mailboxInterval{3000};
};
} // namespace kirox
