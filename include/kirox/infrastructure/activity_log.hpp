#pragma once
#include "kirox/ports/activity_log.hpp"
#include <QString>
#include <mutex>

namespace kirox {
class ActivityLog final : public IActivityLog {
  public:
    explicit ActivityLog(QString directory);
    void append(const QJsonObject &event, bool persistent, int retentionDays) override;
    QJsonArray entries() const override;
    void clear() override;

  private:
    QString directory_;
    mutable std::mutex mutex_;
    QJsonArray entries_;
    QString lastPrunedDate_;
};
} // namespace kirox
