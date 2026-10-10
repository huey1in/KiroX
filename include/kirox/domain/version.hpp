#pragma once
#include <QString>
namespace kirox {
// Invalid versions are rejected, rather than guessed from partial numbers.
bool newerVersion(const QString &candidate, const QString &current);
} // namespace kirox
