#pragma once
#include "error.hpp"
#include <chrono>
#include <condition_variable>
#include <mutex>
#include <stop_token>

namespace kirox {
inline void checkCancelled(std::stop_token token) {
    if (token.stop_requested())
        throw Error(ErrorCode::Cancelled, QStringLiteral("Operation cancelled"));
}
inline void interruptibleWait(std::stop_token token, std::chrono::milliseconds duration) {
    checkCancelled(token);
    std::mutex mutex;
    std::condition_variable_any condition;
    std::unique_lock lock(mutex);
    condition.wait_for(lock, token, duration, [] { return false; });
    checkCancelled(token);
}
} // namespace kirox
