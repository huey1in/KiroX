#pragma once
#include "kirox/ports/repository.hpp"
#include <mutex>

namespace kirox {
class JsonRepository final : public IRepository {
  public:
    // Passing a root keeps tests and portable installations isolated from user data.
    explicit JsonRepository(QString root = {}, QString legacyRoot = {});
    [[nodiscard]] QJsonDocument read(Document document) const override;
    void update(Document document, const std::function<void(QJsonDocument &)> &mutation) override;
    [[nodiscard]] DataPaths paths() const override;
    void relocateData(const QString &target) override;
    void setResultsDirectory(const QString &target) override;
    void saveResult(const QJsonObject &result, const QString &outputDirectory = {}) override;

  private:
    [[nodiscard]] QString pathFor(Document document) const;
    [[nodiscard]] DataPaths pathsLocked() const;
    [[nodiscard]] static QJsonDocument load(const QString &path, const QJsonDocument &fallback);
    static void write(const QString &path, const QJsonDocument &value);
    static void writeBytes(const QString &path, const QByteArray &bytes);
    void migrateLegacy();
    void initialize();
    QString root_;
    QString legacyRoot_;
    mutable std::recursive_mutex mutex_;
};
} // namespace kirox
