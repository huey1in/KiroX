#include "controller.hpp"
#include "kirox/domain/error.hpp"
#include <QApplication>
#include <QDesktopServices>
#include <QFile>
#include <QFileDialog>
#include <QLocale>
#include <QRegularExpression>
#include <QTimer>
#include <QUrl>
namespace kirox {
QString Controller::text(const QString &zh, const QString &en, const QString &ja) const {
    return language() == "zh" ? zh : language() == "ja" ? ja : en;
}
QString Controller::errorMessage(const std::exception &error) const {
    const auto *typed = dynamic_cast<const Error *>(&error);
    if (!typed)
        return text("操作未完成，请重试", "The operation could not finish. Please retry.",
                    "操作を完了できませんでした。再試行してください。");
    const auto detail = typed->message();
    static const QMap<QString, QStringList> messages{
        {"Insufficient available mailbox accounts", {"可用邮箱数量不足", "利用可能なメールが不足しています"}},
        {"Select a configured mailbox service", {"请先配置邮箱服务", "メールサービスを設定してください"}},
        {"Duplicate mailbox configuration name", {"邮箱服务名称已存在", "メールサービス名が重複しています"}},
        {"Proxy already exists", {"该代理已存在", "このプロキシは既に存在します"}},
        {"No enabled proxy is available", {"没有可用的已启用代理", "有効なプロキシがありません"}},
        {"A batch is already running", {"已有任务正在运行", "タスクは既に実行中です"}},
        {"Wait for the current operation to finish", {"请等待当前操作完成", "現在の操作が終了するまでお待ちください"}},
        {"Stop the batch before deleting mailboxes",
         {"请先停止任务再删除邮箱", "タスクを停止してからメールを削除してください"}},
        {"Stop the batch before clearing mailboxes",
         {"请先停止任务再清空邮箱", "タスクを停止してからメールを消去してください"}},
        {"Stop the batch before changing folders",
         {"请先停止任务再更改目录", "タスクを停止してからフォルダーを変更してください"}}};
    if (messages.contains(detail))
        return language() == "en" ? detail : messages.value(detail).at(language() == "ja" ? 1 : 0);
    switch (typed->code()) {
    case ErrorCode::InvalidInput:
        return text("输入无效，请检查填写内容", "Invalid input. Check the entered values.",
                    "入力内容を確認してください。");
    case ErrorCode::Storage:
        return text("无法读写文件，请检查目录权限和磁盘空间",
                    "Cannot read or write files. Check folder access and disk space.",
                    "ファイルを読み書きできません。アクセス権と空き容量を確認してください。");
    case ErrorCode::Network:
        return text("连接失败，请检查网络和代理", "Connection failed. Check your network and proxy.",
                    "接続に失敗しました。ネットワークとプロキシを確認してください。");
    case ErrorCode::Timeout:
        return text("操作超时，请重试", "The operation timed out. Please retry.",
                    "操作がタイムアウトしました。再試行してください。");
    case ErrorCode::Cancelled:
        return text("操作已取消", "Operation cancelled.", "操作をキャンセルしました。");
    case ErrorCode::Protocol:
        return text("服务响应无效，请检查配置后重试", "Invalid service response. Check the configuration and retry.",
                    "サービスの応答が無効です。設定を確認して再試行してください。");
    case ErrorCode::Conflict:
        return text("操作发生冲突，请检查现有配置", "The operation conflicts with the existing configuration.",
                    "既存の設定と競合しています。設定を確認してください。");
    case ErrorCode::NotFound:
        return text("项目已不存在，请刷新后重试", "The item no longer exists. Refresh and retry.",
                    "項目が存在しません。更新して再試行してください。");
    }
    return {};
}
Controller::Controller(IRepository &repository, const ITransportFactory &transports, const IMailboxFactory &mailboxes,
                       const IRegistrationService &registration, IActivityLog &log)
    : settings_(repository), accounts_(repository), proxies_(repository, transports), configurations_(repository),
      batch_(repository, registration), log_(log), updates_(transports), mailboxFactory_(mailboxes) {
    if (settings_.get().language.isEmpty()) {
        const auto locale = QLocale::system().language();
        (void)settings_.patch({{"language", locale == QLocale::Chinese    ? "zh"
                                            : locale == QLocale::Japanese ? "ja"
                                                                          : "en"}});
    }
    auto *timer = new QTimer(this);
    timer->setInterval(1000);
    connect(timer, &QTimer::timeout, this, [this] {
        if (batch_.status().value("running").toBool())
            emit batchChanged();
    });
    timer->start();
    QTimer::singleShot(2000, this, [this] {
        if (settings_.get().autoCheckUpdates)
            checkUpdates();
    });
}
void Controller::checkUpdates() {
    if (updateChecking_)
        return;
    updateChecking_ = true;
    emit updateChanged();
    updateWorker_ = std::jthread([this](std::stop_token stop) {
        QJsonObject result;
        try {
            result = updates_.check(QApplication::applicationVersion(), stop);
        } catch (const std::exception &) {
            result = {{"error", true}};
        }
        QMetaObject::invokeMethod(
            this,
            [this, result] {
                updateChecking_ = false;
                updateInfo_ = result.toVariantMap();
                emit updateChanged();
            },
            Qt::QueuedConnection);
    });
}
Controller::~Controller() {
    worker_.request_stop();
    updateWorker_.request_stop();
    batch_.stop();
    if (worker_.joinable())
        worker_.join();
    if (updateWorker_.joinable())
        updateWorker_.join();
    batch_.wait();
}
QVariantMap Controller::batchStatus() const {
    return batch_.status().toVariantMap();
}
QVariantList Controller::tasks() const {
    return batch_.tasks().toVariantList();
}
QVariantList Controller::activity() const {
    return log_.entries().toVariantList();
}
QString Controller::stepLabel(const QString &step) const {
    static const QMap<QString, QStringList> names{
        {"OIDC", {"初始化注册", "Initialize registration", "登録を初期化"}},
        {"Device", {"设备授权", "Authorize device", "デバイス認証"}},
        {"Email", {"准备邮箱", "Prepare mailbox", "メールを準備"}},
        {"Portal", {"访问门户", "Open portal", "ポータルにアクセス"}},
        {"WorkflowInit", {"初始化流程", "Initialize workflow", "フローを初期化"}},
        {"SubmitEmail", {"提交邮箱", "Submit email", "メールを送信"}},
        {"Signup", {"开始注册", "Start signup", "登録を開始"}},
        {"SignupInit", {"初始化注册", "Initialize signup", "登録を初期化"}},
        {"ProfileInit", {"加载身份页面", "Load profile", "プロフィールを読み込み"}},
        {"ProfileStart", {"启动身份流程", "Start profile", "プロフィールを開始"}},
        {"SendOTP", {"发送验证码", "Send verification code", "確認コードを送信"}},
        {"GetOTP", {"等待验证码", "Wait for verification code", "確認コードを待機"}},
        {"CreateIdentity", {"创建身份", "Create identity", "ID を作成"}},
        {"SetPassword", {"设置密码", "Set password", "パスワードを設定"}},
        {"SSOWorkflow", {"完成登录流程", "Complete SSO workflow", "SSO フローを完了"}},
        {"SSOToken", {"获取登录凭证", "Get SSO token", "SSO トークンを取得"}},
        {"KiroAuthorize", {"授权 Kiro", "Authorize Kiro", "Kiro を認証"}},
        {"KiroExchange", {"获取访问令牌", "Exchange tokens", "トークンを取得"}},
        {"Verify", {"检查账号状态", "Verify account", "アカウントを確認"}}};
    const auto labels = names.value(step);
    if (labels.isEmpty())
        return step;
    return labels.at(language() == "zh" ? 0 : language() == "ja" ? 2 : 1);
}
bool Controller::startBatch(const QVariantMap &options) {
    return perform([&] {
        if (busy_)
            throw Error(ErrorCode::Conflict, "Wait for the current operation to finish");
        BatchRequest request;
        request.provider = providerKind(options.value("provider").toString());
        request.count = options.value("count", 1).toInt();
        request.concurrency = options.value("concurrency", 1).toInt();
        request.delaySeconds = options.value("delaySeconds", 1).toInt();
        request.configurationNames = options.value("configurationNames").toStringList();
        request.domains = options.value("domains").toStringList();
        request.randomDomains = options.value("randomDomains").toBool();
        request.proxyMode = options.value("proxyMode", "direct").toString();
        request.proxyId = options.value("proxyId").toString();
        request.outputDirectory = options.value("outputDirectory").toString();
        batch_.start(request, settings_.get(), [this](const QJsonObject &event) {
            QMetaObject::invokeMethod(
                this,
                [this, event] {
                    if (event.value("batchId") != batch_.status().value("batchId"))
                        return;
                    const auto runtime = settings_.get();
                    // Activity contains only structured status fields; no credentials.
                    if (event.value("event") == "finished" || event.contains("step") || event.contains("status")) {
                        try {
                            log_.append(event, runtime.persistentLogs, runtime.logRetentionDays);
                            emit activityChanged();
                        } catch (const std::exception &) {
                            emit operationFailed(language() == "zh"   ? "无法保存运行日志"
                                                 : language() == "ja" ? "ログを保存できません"
                                                                      : "Cannot save the activity log");
                        }
                    }
                    emit batchChanged();
                    if (event.value("event") == "finished") {
                        emit mailboxesChanged();
                        emit batchFinished(batchStatus());
                    }
                },
                Qt::QueuedConnection);
        });
        emit batchChanged();
    });
}
void Controller::stopBatch() {
    batch_.stop();
    emit batchChanged();
}
void Controller::clearActivity() {
    log_.clear();
    emit activityChanged();
}
QVariantMap Controller::settings() const {
    return settings_.get().toJson().toVariantMap();
}
QVariantList Controller::mailboxes() const {
    QVariantList result;
    for (auto kind : {MailboxKind::Outlook, MailboxKind::ICloud})
        for (const auto &a : accounts_.list(kind)) {
            auto row = a.toJson().toVariantMap();
            // Secret values remain in the repository; list models expose only UI fields.
            row.remove("password");
            row.remove("refreshToken");
            row.remove("clientId");
            row.remove("messagesURL");
            result.append(row);
        }
    return result;
}
QVariantList Controller::proxies() const {
    QVariantList result;
    for (const auto &entry : proxies_.list()) {
        auto row = entry.toJson().toVariantMap();
        row["url"] = redactProxy(entry.url);
        result.append(row);
    }
    return result;
}
QVariantMap Controller::directories() const {
    const auto paths = settings_.paths();
    return {{"data", paths.data}, {"results", paths.results}, {"logs", paths.logs}, {"cache", paths.cache}};
}
QString Controller::language() const {
    return settings_.get().language;
}
QVariantList Controller::providerConfigurations() const {
    QVariantList result;
    for (const auto kind : {MailboxKind::MoeMail, MailboxKind::CloudMail, MailboxKind::MailNest}) {
        const auto document = configurations_.get(kind);
        const auto entries = document.isArray()            ? document.array()
                             : document.object().isEmpty() ? QJsonArray{}
                                                           : QJsonArray{document.object()};
        for (const auto &entry : entries) {
            auto row = entry.toObject();
            row.remove("apiKey");
            row.remove("password");
            row["provider"] = providerId(kind);
            result.append(row.toVariantMap());
        }
    }
    return result;
}
QVariantMap Controller::providerConfiguration(const QString &provider, const QString &name) {
    QVariantMap result;
    perform([&] {
        const auto document = configurations_.get(providerKind(provider));
        if (document.isObject())
            result = document.object().toVariantMap();
        else
            for (const auto &entry : document.array())
                if (entry.toObject()["name"].toString() == name) {
                    result = entry.toObject().toVariantMap();
                    break;
                }
    });
    return result;
}
bool Controller::saveProviderConfiguration(const QString &provider, const QVariantMap &configuration,
                                           const QString &oldName) {
    return perform([&] {
        configurations_.upsert(providerKind(provider), QJsonObject::fromVariantMap(configuration), oldName);
        emit providerConfigurationsChanged();
    });
}
void Controller::removeProviderConfiguration(const QString &provider, const QString &name) {
    perform([&] {
        configurations_.remove(providerKind(provider), name);
        emit providerConfigurationsChanged();
    });
}
void Controller::inspectProvider(const QString &provider, const QString &name) {
    if (busy_)
        return;
    const auto configuration = QJsonObject::fromVariantMap(providerConfiguration(provider, name));
    if (configuration.isEmpty())
        return;
    busy_ = true;
    emit busyChanged();
    const auto runtime = settings_.get();
    worker_ = std::jthread([this, provider, configuration, runtime](std::stop_token stop) {
        QString error;
        QJsonObject result;
        try {
            result = mailboxFactory_.inspect(providerKind(provider), configuration,
                                             {runtime.mailboxProxy({}), "KiroX/2.0", {}}, stop);
        } catch (const std::exception &e) {
            error = errorMessage(e);
        }
        QMetaObject::invokeMethod(
            this,
            [this, provider, error, result] {
                busy_ = false;
                emit busyChanged();
                if (!error.isEmpty())
                    emit operationFailed(error);
                else
                    emit providerInspected(provider, result.toVariantMap());
            },
            Qt::QueuedConnection);
    });
}
void Controller::cancelOperation() {
    worker_.request_stop();
}
bool Controller::patchSettings(const QVariantMap &changes) {
    return perform([&] {
        (void)settings_.patch(QJsonObject::fromVariantMap(changes));
        emit settingsChanged();
    });
}
void Controller::importAccounts(const QString &provider, const QString &text) {
    perform([&] {
        const auto result = accounts_.import(providerKind(provider), text);
        emit mailboxesChanged();
        emit operationSucceeded(this->text("新增 %1 · 重复 %2 · 无效 %3", "%1 added · %2 duplicates · %3 invalid",
                                           "追加 %1 · 重複 %2 · 無効 %3")
                                    .arg(result.added)
                                    .arg(result.duplicates)
                                    .arg(result.invalid));
    });
}
void Controller::importAccountsFile(const QString &provider) {
    const auto path = QFileDialog::getOpenFileName(
        nullptr, text("导入邮箱", "Import mailboxes", "メールをインポート"), {},
        text("账号文件 (*.txt *.csv);;所有文件 (*)", "Account files (*.txt *.csv);;All files (*)",
             "アカウントファイル (*.txt *.csv);;すべてのファイル (*)"));
    if (path.isEmpty())
        return;
    perform([&] {
        QFile file(path);
        if (!file.open(QIODevice::ReadOnly))
            throw Error(ErrorCode::Storage, file.errorString());
        importAccounts(provider, QString::fromUtf8(file.readAll()));
    });
}
void Controller::deleteAccount(const QString &provider, const QString &address) {
    perform([&] {
        if (batch_.status().value("running").toBool())
            throw Error(ErrorCode::Conflict, "Stop the batch before deleting mailboxes");
        accounts_.remove(providerKind(provider), address);
        emit mailboxesChanged();
    });
}
void Controller::clearAccounts(const QString &provider, bool registeredOnly) {
    perform([&] {
        if (batch_.status().value("running").toBool())
            throw Error(ErrorCode::Conflict, "Stop the batch before clearing mailboxes");
        (void)accounts_.clear(providerKind(provider), registeredOnly);
        emit mailboxesChanged();
    });
}
void Controller::addProxy(const QString &name, const QString &url, int weight) {
    QString id;
    perform([&] {
        id = proxies_.add(name, url, weight).id;
        emit proxiesChanged();
    });
    if (!id.isEmpty() && settings_.get().autoProbeProxies)
        probeProxies({id});
}
QVariantMap Controller::proxyConfiguration(const QString &id) {
    for (const auto &entry : proxies_.list())
        if (entry.id == id)
            return entry.toJson().toVariantMap();
    return {};
}
bool Controller::saveProxy(const QString &id, const QString &name, const QString &url, int weight) {
    const bool saved = perform([&] {
        const auto stored = proxyConfiguration(id);
        if (stored.isEmpty())
            throw Error(ErrorCode::NotFound, "Proxy not found");
        auto entry = ProxyEntry::fromJson(QJsonObject::fromVariantMap(stored));
        if (entry.url != normalizeProxy(url))
            entry.probe = {};
        entry.url = url;
        entry.name = name.trimmed().isEmpty() ? redactProxy(normalizeProxy(url)) : name.trimmed();
        entry.weight = weight;
        proxies_.update(entry);
        emit proxiesChanged();
    });
    if (saved && settings_.get().autoProbeProxies)
        probeProxies({id});
    return saved;
}
void Controller::importProxies(const QString &text) {
    int count = 0;
    QStringList ids;
    for (const auto &line : text.split(QRegularExpression("[\\r\\n]+"), Qt::SkipEmptyParts)) {
        if (perform([&] { ids.append(proxies_.add({}, line).id); }))
            ++count;
    }
    emit proxiesChanged();
    emit operationSucceeded(
        this->text("已添加 %1 个代理", "%1 proxies added", "プロキシを %1 件追加しました").arg(count));
    if (!ids.isEmpty() && settings_.get().autoProbeProxies)
        probeProxies(ids);
}
void Controller::deleteProxy(const QString &id) {
    perform([&] {
        proxies_.remove(id);
        emit proxiesChanged();
    });
}
void Controller::enableProxy(const QString &id, bool enabled) {
    perform([&] {
        for (auto entry : proxies_.list())
            if (entry.id == id) {
                entry.enabled = enabled;
                proxies_.update(entry);
                break;
            }
        emit proxiesChanged();
    });
}
void Controller::testProxy(const QString &id) {
    probeProxies({id});
}
void Controller::probeProxies(const QStringList &ids) {
    if (busy_)
        return;
    busy_ = true;
    emit busyChanged();
    worker_ = std::jthread([this, ids](std::stop_token stop) {
        QString error;
        try {
            for (const auto &id : ids) {
                if (stop.stop_requested())
                    break;
                (void)proxies_.probe(id, stop);
            }
        } catch (const std::exception &e) {
            error = errorMessage(e);
        }
        QMetaObject::invokeMethod(
            this,
            [this, error] {
                busy_ = false;
                emit busyChanged();
                emit proxiesChanged();
                if (!error.isEmpty())
                    emit operationFailed(error);
            },
            Qt::QueuedConnection);
    });
}
void Controller::selectDirectory(const QString &kind) {
    if (batch_.status().value("running").toBool()) {
        emit operationFailed(language() == "zh"   ? "请先停止任务再更改目录"
                             : language() == "ja" ? "タスクを停止してからフォルダーを変更してください"
                                                  : "Stop the batch before changing folders");
        return;
    }
    const auto directory =
        QFileDialog::getExistingDirectory(nullptr, text("选择目录", "Select directory", "フォルダーを選択"));
    if (directory.isEmpty())
        return;
    perform([&] {
        if (kind == "data")
            settings_.changeDataDirectory(directory);
        else
            settings_.changeResultsDirectory(directory);
        emit settingsChanged();
        emit mailboxesChanged();
        emit proxiesChanged();
    });
}
void Controller::resetDirectory(const QString &kind) {
    perform([&] {
        if (batch_.status().value("running").toBool())
            throw Error(ErrorCode::Conflict, "Stop the batch before changing folders");
        if (kind == "data")
            settings_.changeDataDirectory({});
        else
            settings_.changeResultsDirectory({});
        emit settingsChanged();
        emit mailboxesChanged();
        emit proxiesChanged();
    });
}
void Controller::openDirectory(const QString &kind) {
    QDesktopServices::openUrl(QUrl::fromLocalFile(directories().value(kind).toString()));
}
void Controller::openUrl(const QString &url) {
    const QUrl target(url);
    if (target.scheme() == "https" || target.scheme() == "http")
        QDesktopServices::openUrl(target);
}
void Controller::quit() {
    QApplication::quit();
}
} // namespace kirox
