#include "kirox/domain/version.hpp"
#include "kirox/domain/error.hpp"
#include <QRegularExpression>
#include <QStringList>

namespace kirox {
namespace {
struct Version {
    QStringList numbers, pre;
};
Version parse(const QString &version) {
    const auto match = QRegularExpression("^v?(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:"
                                          "\\.[0-9A-Za-z-]+)*))?(?:\\+[0-9A-Za-z-]+(?:\\.[0-9A-Za-z-]+)*)?$")
                           .match(version);
    if (!match.hasMatch())
        throw Error(ErrorCode::InvalidInput, "Invalid semantic version");
    Version result{{match.captured(1), match.captured(2), match.captured(3)}, {}};
    if (!match.captured(4).isEmpty())
        result.pre = match.captured(4).split('.');
    for (const auto &identifier : result.pre)
        if (QRegularExpression("^[0-9]+$").match(identifier).hasMatch() && identifier.size() > 1 &&
            identifier.startsWith('0'))
            throw Error(ErrorCode::InvalidInput, "Invalid numeric prerelease identifier");
    return result;
}
int numericCompare(const QString &a, const QString &b) {
    if (a.size() != b.size())
        return a.size() > b.size() ? 1 : -1;
    return QString::compare(a, b, Qt::CaseSensitive);
}
} // namespace
bool newerVersion(const QString &candidate, const QString &current) {
    const auto a = parse(candidate), b = parse(current);
    for (int i = 0; i < 3; ++i) {
        const auto comparison = numericCompare(a.numbers.at(i), b.numbers.at(i));
        if (comparison != 0)
            return comparison > 0;
    }
    if (a.pre.isEmpty() || b.pre.isEmpty())
        return a.pre.isEmpty() && !b.pre.isEmpty();
    for (qsizetype i = 0; i < std::min(a.pre.size(), b.pre.size()); ++i) {
        const auto &x = a.pre.at(i), &y = b.pre.at(i);
        if (x == y)
            continue;
        const bool xn = QRegularExpression("^[0-9]+$").match(x).hasMatch(),
                   yn = QRegularExpression("^[0-9]+$").match(y).hasMatch();
        if (xn != yn)
            return !xn;
        return (xn ? numericCompare(x, y) : QString::compare(x, y, Qt::CaseSensitive)) > 0;
    }
    return a.pre.size() > b.pre.size();
}
} // namespace kirox
