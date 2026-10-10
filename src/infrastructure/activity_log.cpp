#include "kirox/infrastructure/activity_log.hpp"
#include "kirox/domain/error.hpp"
#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QJsonDocument>
#include <QRegularExpression>
#include <algorithm>

namespace kirox {
namespace {
QJsonObject allowedFields(const QJsonObject &event) {
    QJsonObject safe;
    for (const auto key : {"time", "index", "step", "status", "errorCode", "warning", "event"})
        if (event.contains(key))
            safe.insert(key, event.value(key));
    return safe;
}
} // namespace
ActivityLog::ActivityLog(QString directory) : directory_(std::move(directory)) {
    const QDir dir(directory_);
    const auto files = dir.entryList({"kirox-native-????"
                                      "-??"
                                      "-??.jsonl"},
                                     QDir::Files, QDir::Name | QDir::Reversed);
    // Bounded reads avoid loading an unbounded historical log into the UI.
    QList<QJsonObject> recent;
    for (const auto &name : files) {
        QFile file(dir.filePath(name));
        if (!file.open(QIODevice::ReadOnly))
            continue;
        if (file.size() > 1024 * 1024) {
            file.seek(file.size() - 1024 * 1024);
            file.readLine();
        }
        QList<QJsonObject> rows;
        while (!file.atEnd()) {
            const auto value = QJsonDocument::fromJson(file.readLine()).object();
            if (!value.isEmpty())
                rows.append(allowedFields(value));
            if (rows.size() > 500)
                rows.removeFirst();
        }
        for (auto it = rows.crbegin(); it != rows.crend() && recent.size() < 500; ++it)
            recent.prepend(*it);
        if (recent.size() >= 500)
            break;
    }
    for (const auto &row : recent)
        entries_.append(row);
}
void ActivityLog::append(const QJsonObject &event, bool persistent, int retentionDays) {
    std::scoped_lock lock(mutex_);
    auto row = allowedFields(event);
    row.insert("time", QDateTime::currentDateTime().toString(Qt::ISODateWithMs));
    entries_.append(row);
    if (entries_.size() > 500)
        entries_.removeFirst();
    if (!persistent)
        return;
    QDir directory(directory_);
    if (!directory.mkpath("."))
        throw Error(ErrorCode::Storage, "Cannot create the activity log directory");
    const auto date = QDate::currentDate();
    const auto dateText = date.toString(Qt::ISODate);
    if (lastPrunedDate_ != dateText) {
        for (const auto &name : directory.entryList({"kirox-native-????"
                                                     "-??"
                                                     "-??.jsonl"},
                                                    QDir::Files)) {
            const auto storedDate = QDate::fromString(name.mid(13, 10), Qt::ISODate);
            if (storedDate.isValid() && storedDate < date.addDays(-std::clamp(retentionDays, 1, 90)))
                QFile::remove(directory.filePath(name));
        }
        lastPrunedDate_ = dateText;
    }
    QFile file(directory.filePath("kirox-native-" + dateText + ".jsonl"));
    if (!file.open(QIODevice::WriteOnly | QIODevice::Append))
        throw Error(ErrorCode::Storage, "Cannot append the activity log");
    file.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner);
    const auto bytes = QJsonDocument(row).toJson(QJsonDocument::Compact) + '\n';
    if (file.write(bytes) != bytes.size() || !file.flush())
        throw Error(ErrorCode::Storage, "Cannot save the activity log");
}
QJsonArray ActivityLog::entries() const {
    std::scoped_lock lock(mutex_);
    return entries_;
}
void ActivityLog::clear() {
    std::scoped_lock lock(mutex_);
    entries_ = {};
}
} // namespace kirox
