#pragma once
#include <QObject>
#include <QSoundEffect>
#include <QSystemTrayIcon>
#include <QVariantMap>
#include <memory>

class QQuickWindow;
namespace kirox {
class DesktopFeedback final : public QObject {
  public:
    explicit DesktopFeedback(QQuickWindow *window, QObject *parent = nullptr);
    void completed(const QVariantMap &status, const QVariantMap &settings);

  private:
    QQuickWindow *window_;
    std::unique_ptr<QSoundEffect> sound_;
    std::unique_ptr<QSystemTrayIcon> tray_;
};
} // namespace kirox
