#pragma once
#include <QString>
#include <stdexcept>

namespace kirox {
enum class ErrorCode { InvalidInput, Storage, Network, Timeout, Cancelled, Protocol, Conflict, NotFound };
class Error final : public std::runtime_error {
  public:
    Error(ErrorCode code, QString message)
        : std::runtime_error(message.toStdString()), code_(code), message_(std::move(message)) {}
    [[nodiscard]] ErrorCode code() const noexcept {
        return code_;
    }
    [[nodiscard]] const QString &message() const noexcept {
        return message_;
    }

  private:
    ErrorCode code_;
    QString message_;
};
} // namespace kirox
