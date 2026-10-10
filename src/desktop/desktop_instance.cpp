#include "desktop_instance.hpp"
#include "kirox/domain/error.hpp"
#include <QCryptographicHash>
#include <QDir>
#include <QLocalSocket>

namespace kirox {
DesktopInstance::DesktopInstance(const QString &root, QObject *parent)
    : QObject(parent), lock_(QDir(root).filePath("desktop.lock")) {
    auto path = QDir(root).absolutePath();
#ifdef Q_OS_WIN
    path = path.toLower();
#endif
    const auto name =
        "kirox-" + QString::fromLatin1(QCryptographicHash::hash(path.toUtf8(), QCryptographicHash::Sha256).toHex());
    lock_.setStaleLockTime(0);
    primary_ = lock_.tryLock(100);
    if (!primary_) {
        QLocalSocket socket;
        socket.connectToServer(name);
        if (socket.waitForConnected(1500)) {
            socket.write("activate\n");
            socket.waitForBytesWritten(1000);
        }
        return;
    }
    server_.setSocketOptions(QLocalServer::UserAccessOption);
    QLocalServer::removeServer(name);
    if (!server_.listen(name))
        throw Error(ErrorCode::Conflict, "Cannot listen for desktop activation");
    connect(&server_, &QLocalServer::newConnection, this, [this] {
        while (auto *socket = server_.nextPendingConnection()) {
            connect(socket, &QLocalSocket::disconnected, socket, &QObject::deleteLater);
            connect(socket, &QLocalSocket::readyRead, this, [this, socket] {
                if (socket->readAll().startsWith("activate"))
                    emit activationRequested();
                socket->disconnectFromServer();
            });
            if (socket->bytesAvailable() > 0) {
                if (socket->readAll().startsWith("activate"))
                    emit activationRequested();
                socket->disconnectFromServer();
            }
        }
    });
}
} // namespace kirox
