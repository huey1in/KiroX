#pragma once
#include <QLocalServer>
#include <QLockFile>
#include <QObject>

namespace kirox {
class DesktopInstance final : public QObject {
    Q_OBJECT
  public:
    explicit DesktopInstance(const QString &root, QObject *parent = nullptr);
    bool primary() const {
        return primary_;
    }
  signals:
    void activationRequested();

  private:
    QLockFile lock_;
    QLocalServer server_;
    bool primary_ = false;
};
} // namespace kirox
