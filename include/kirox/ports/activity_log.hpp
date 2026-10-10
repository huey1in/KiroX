#pragma once
#include <QJsonArray>
#include <QJsonObject>

namespace kirox {
class IActivityLog {
  public:
    virtual ~IActivityLog() = default;
    virtual void append(const QJsonObject &event, bool persistent, int retentionDays) = 0;
    virtual QJsonArray entries() const = 0;
    virtual void clear() = 0;
};
} // namespace kirox
