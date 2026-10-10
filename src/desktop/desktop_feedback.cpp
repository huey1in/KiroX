#include "desktop_feedback.hpp"
#include <QApplication>
#include <QQuickWindow>

namespace kirox {
DesktopFeedback::DesktopFeedback(QQuickWindow *window, QObject *parent) : QObject(parent), window_(window) {}
void DesktopFeedback::completed(const QVariantMap &status, const QVariantMap &settings) {
    if (settings.value("soundEnabled").toBool() && settings.value("soundVolume").toInt() > 0) {
        if (!sound_) {
            sound_ = std::make_unique<QSoundEffect>();
            sound_->setLoopCount(1);
            connect(sound_.get(), &QSoundEffect::statusChanged, this, [this] {
                if (sound_->status() == QSoundEffect::Ready)
                    sound_->play();
            });
            sound_->setSource(QUrl("qrc:/kirox/completion.wav"));
        }
        sound_->setVolume(qBound(0.0, settings.value("soundVolume").toInt() / 100.0, 1.0));
        if (sound_->status() == QSoundEffect::Ready)
            sound_->play();
    }
    if (!settings.value("desktopNotifications").toBool())
        return;
    window_->alert(0);
    if (!QSystemTrayIcon::isSystemTrayAvailable())
        return;
    if (!tray_) {
        tray_ = std::make_unique<QSystemTrayIcon>(QApplication::windowIcon());
        tray_->setToolTip("KiroX");
        tray_->show();
        connect(tray_.get(), &QSystemTrayIcon::activated, this, [this](QSystemTrayIcon::ActivationReason reason) {
            if (reason == QSystemTrayIcon::Trigger || reason == QSystemTrayIcon::DoubleClick) {
                window_->showNormal();
                window_->raise();
                window_->requestActivate();
            }
        });
    }
    const auto language = settings.value("language").toString();
    const auto title = language == "zh"   ? "注册任务已结束"
                       : language == "ja" ? "登録タスクが終了しました"
                                          : "Registration batch finished";
    const auto pattern = language == "zh"   ? "成功 %1 · 失败 %2 · 取消 %3"
                         : language == "ja" ? "成功 %1 · 失敗 %2 · キャンセル %3"
                                            : "Succeeded %1 · Failed %2 · Cancelled %3";
    tray_->showMessage(title,
                       QString::fromUtf8(pattern)
                           .arg(status.value("success").toInt())
                           .arg(status.value("failed").toInt())
                           .arg(status.value("cancelled").toInt()),
                       QSystemTrayIcon::Information, 5000);
}
} // namespace kirox
