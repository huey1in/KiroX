#pragma once
#include "kirox/domain/settings.hpp"
#include "kirox/ports/repository.hpp"
namespace kirox {
class SettingsService {
  public:
    explicit SettingsService(IRepository &repository) : repository_(repository) {}
    [[nodiscard]] Settings get() const;
    [[nodiscard]] Settings patch(const QJsonObject &changes);
    [[nodiscard]] DataPaths paths() const {
        return repository_.paths();
    }
    void changeDataDirectory(const QString &directory) {
        repository_.relocateData(directory);
    }
    void changeResultsDirectory(const QString &directory) {
        repository_.setResultsDirectory(directory);
    }

  private:
    IRepository &repository_;
};
} // namespace kirox
