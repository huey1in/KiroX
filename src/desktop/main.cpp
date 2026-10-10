#include "controller.hpp"
#include "desktop_feedback.hpp"
#include "desktop_instance.hpp"
#include "kirox/application/registration_service.hpp"
#include "kirox/domain/error.hpp"
#include "kirox/infrastructure/activity_log.hpp"
#include "kirox/infrastructure/app_script_cache.hpp"
#include "kirox/infrastructure/curl_transport.hpp"
#include "kirox/infrastructure/http_mailbox.hpp"
#include "kirox/infrastructure/imap_mailbox.hpp"
#include "kirox/infrastructure/json_repository.hpp"
#include "kirox/infrastructure/native_crypto.hpp"
#include <QApplication>
#include <QCommandLineParser>
#include <QDir>
#include <QFont>
#include <QFontDatabase>
#include <QIcon>
#include <QLockFile>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QQuickStyle>
#include <QQuickWindow>
#include <QRegularExpression>
#include <QTimer>
#include <cstdio>

int main(int argc, char *argv[]) {
    qInstallMessageHandler([](QtMsgType, const QMessageLogContext &, const QString &message) {
        std::fprintf(stderr, "%s\n", message.toUtf8().constData());
    });
    QApplication app(argc, argv);
    // Headless renderers do not provide the operating system's font fallback
    // registry. Register the explicitly supplied test fonts, including TTC faces.
    if (QGuiApplication::platformName() == "offscreen") {
        const QDir fonts(qEnvironmentVariable("QT_QPA_FONTDIR"));
        if (!qEnvironmentVariableIsEmpty("QT_QPA_FONTDIR"))
            for (const auto &file : fonts.entryList({"*.ttf", "*.ttc", "*.otf"}, QDir::Files))
                QFontDatabase::addApplicationFont(fonts.filePath(file));
    }
#ifdef Q_OS_WIN
    app.setFont(QFont("Segoe UI", 10));
    QFontDatabase::setApplicationFallbackFontFamilies(QChar::Script_Han, {"Microsoft YaHei", "Yu Gothic"});
    QFontDatabase::setApplicationFallbackFontFamilies(QChar::Script_Hiragana, {"Yu Gothic"});
    QFontDatabase::setApplicationFallbackFontFamilies(QChar::Script_Katakana, {"Yu Gothic"});
#else
    app.setFont(QFontDatabase::systemFont(QFontDatabase::GeneralFont));
#endif
    app.setOrganizationName("huey1in");
    app.setApplicationName("KiroX");
    app.setApplicationVersion("2.0.0");
    app.setWindowIcon(QIcon(":/qt/qml/KiroX/assets/kirox-light.svg"));
    QQuickStyle::setStyle("Basic");
    QCommandLineParser parser;
    parser.addHelpOption();
    parser.addVersionOption();
    parser.addOption({"data-home", "Use an isolated data directory", "directory"});
    parser.addOption({"screenshot", "Save the rendered window and exit", "file"});
    parser.addOption({"page", "Select an initial page", "name", "overview"});
    parser.addOption({"window-size", "Set the initial window size (for example 820x580)", "size"});
    parser.process(app);
    try {
        kirox::JsonRepository repository(parser.value("data-home"));
        kirox::DesktopInstance instance(repository.paths().root);
        if (!instance.primary())
            return 0;
        kirox::CurlTransportFactory transports;
        kirox::NativeImapFactory imap;
        kirox::HttpMailboxFactory mailboxes(transports, {}, &imap);
        kirox::NativeCryptography crypto;
        kirox::AppScriptCache fingerprints(transports);
        kirox::RegistrationService registration(transports, mailboxes, crypto, fingerprints);
        kirox::ActivityLog log(repository.paths().logs);
        kirox::Controller controller(repository, transports, mailboxes, registration, log);
        QQmlApplicationEngine engine;
        engine.rootContext()->setContextProperty("backend", &controller);
        engine.rootContext()->setContextProperty("initialPage", parser.value("page"));
        QObject::connect(
            &engine, &QQmlApplicationEngine::objectCreationFailed, &app, [] { QCoreApplication::exit(1); },
            Qt::QueuedConnection);
        engine.loadFromModule("KiroX", "Main");
        if (engine.rootObjects().isEmpty())
            return 1;
        auto *window = qobject_cast<QQuickWindow *>(engine.rootObjects().first());
        if (const auto size = parser.value("window-size"); !size.isEmpty()) {
            const auto match = QRegularExpression("^(\\d{3,4})x(\\d{3,4})$").match(size);
            if (!match.hasMatch() || match.captured(1).toInt() < 820 || match.captured(2).toInt() < 580)
                throw kirox::Error(kirox::ErrorCode::InvalidInput, "Invalid window size; minimum is 820x580");
            window->resize(match.captured(1).toInt(), match.captured(2).toInt());
        }
        QObject::connect(&instance, &kirox::DesktopInstance::activationRequested, window, [window] {
            window->showNormal();
            window->raise();
            window->requestActivate();
        });
        kirox::DesktopFeedback feedback(window);
        QObject::connect(&controller, &kirox::Controller::batchFinished, &feedback,
                         [&](const QVariantMap &status) { feedback.completed(status, controller.settings()); });
        if (const auto output = parser.value("screenshot"); !output.isEmpty()) {
            QTimer::singleShot(1200, &app, [&engine, &app, output] {
                auto *window = qobject_cast<QQuickWindow *>(engine.rootObjects().first());
                const bool saved = window && window->grabWindow().save(output);
                app.exit(saved ? 0 : 2);
            });
        }
        return app.exec();
    } catch (const std::exception &error) {
        std::fprintf(stderr, "KiroX: %s\n", error.what());
        return 1;
    }
}
