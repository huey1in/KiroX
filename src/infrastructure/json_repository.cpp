#include "kirox/infrastructure/json_repository.hpp"
#include "kirox/domain/error.hpp"
#include "kirox/domain/settings.hpp"
#include <QCryptographicHash>
#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonArray>
#include <QJsonParseError>
#include <QMap>
#include <QSaveFile>
#include <QStandardPaths>
#include <vector>

namespace kirox {
namespace {
QString fileName(Document d) {
    switch (d) {
    case Document::Settings:
        return "settings.json";
    case Document::Accounts:
        return "accounts.json";
    case Document::MoeMail:
        return "moemail.json";
    case Document::CloudMail:
        return "cloudmail.json";
    case Document::MailNest:
        return "mailnest.json";
    case Document::ProxyPool:
        return "proxy_pool.json";
    case Document::Identities:
        return "identities.json";
    }
    throw Error(ErrorCode::InvalidInput, "Unknown document");
}
QJsonDocument fallback(Document d) {
    if (d == Document::Settings)
        return QJsonDocument(QJsonObject{{"schemaVersion", 4}, {"runtime", Settings{}.toJson()}});
    if (d == Document::MailNest || d == Document::Identities)
        return QJsonDocument(QJsonObject{});
    if (d == Document::ProxyPool)
        return QJsonDocument(QJsonObject{{"entries", QJsonArray{}}});
    return QJsonDocument(QJsonArray{});
}
void ensureDirectory(const QString &dir) {
    if (!QDir().mkpath(dir))
        throw Error(ErrorCode::Storage, "Cannot create directory: " + dir);
}
} // namespace
JsonRepository::JsonRepository(QString root, QString legacyRoot)
    : root_(std::move(root)), legacyRoot_(std::move(legacyRoot)) {
    const bool discoverLegacy = root_.isEmpty() && qEnvironmentVariableIsEmpty("KIROX_DATA_HOME");
    if (root_.isEmpty())
        root_ = qEnvironmentVariable("KIROX_DATA_HOME");
    if (root_.isEmpty()) {
#ifdef Q_OS_WIN
        root_ = QDir(qEnvironmentVariable("LOCALAPPDATA",
                                          QStandardPaths::writableLocation(QStandardPaths::GenericDataLocation)))
                    .filePath("KiroX");
#elif defined(Q_OS_MACOS)
        root_ = QDir(QStandardPaths::writableLocation(QStandardPaths::GenericDataLocation)).filePath("KiroX");
#else
        root_ = QDir(QStandardPaths::writableLocation(QStandardPaths::GenericConfigLocation)).filePath("KiroX");
#endif
    }
    root_ = QDir::cleanPath(QFileInfo(root_).absoluteFilePath());
    if (legacyRoot_.isEmpty() && discoverLegacy) {
#ifdef Q_OS_WIN
        legacyRoot_ = QDir(qEnvironmentVariable("APPDATA")).filePath("kirox");
#else
        legacyRoot_ = QDir(QStandardPaths::writableLocation(QStandardPaths::GenericConfigLocation)).filePath("kirox");
#endif
    }
    initialize();
}
void JsonRepository::initialize() {
    ensureDirectory(root_);
    for (const auto &dir : {"data", "cache", "logs"})
        ensureDirectory(QDir(root_).filePath(dir));
    const auto settings = QDir(root_).filePath("settings.json");
    if (!QFileInfo::exists(settings)) {
        if (!legacyRoot_.isEmpty() && QFileInfo(legacyRoot_).isDir())
            migrateLegacy();
        else
            write(settings, fallback(Document::Settings));
    }
    const auto document = load(settings, fallback(Document::Settings));
    if (!document.isObject())
        throw Error(ErrorCode::Storage, "Settings must be a JSON object");
    ensureDirectory(pathsLocked().data);
}
QJsonDocument JsonRepository::load(const QString &path, const QJsonDocument &defaultValue) {
    QFile file(path);
    if (!file.exists())
        return defaultValue;
    if (!file.open(QIODevice::ReadOnly))
        throw Error(ErrorCode::Storage, "Cannot read " + path + ": " + file.errorString());
    QJsonParseError error;
    const auto document = QJsonDocument::fromJson(file.readAll(), &error);
    if (error.error != QJsonParseError::NoError)
        throw Error(ErrorCode::Storage, "Invalid JSON in " + path + ": " + error.errorString());
    // Never silently remove or overwrite malformed user data.
    return document;
}
void JsonRepository::write(const QString &path, const QJsonDocument &value) {
    writeBytes(path, value.toJson(QJsonDocument::Indented));
}
void JsonRepository::writeBytes(const QString &path, const QByteArray &bytes) {
    ensureDirectory(QFileInfo(path).absolutePath());
    QSaveFile file(path);
    file.setDirectWriteFallback(false);
    if (!file.open(QIODevice::WriteOnly))
        throw Error(ErrorCode::Storage, "Cannot write " + path + ": " + file.errorString());
    file.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner);
    if (file.write(bytes) != bytes.size() || !file.commit())
        throw Error(ErrorCode::Storage, "Cannot commit " + path + ": " + file.errorString());
}
void JsonRepository::migrateLegacy() {
    QMap<QString, QString> legacy;
    QFile configuration(QDir(legacyRoot_).filePath("storage.conf"));
    if (configuration.exists()) {
        if (!configuration.open(QIODevice::ReadOnly))
            throw Error(ErrorCode::Storage, "Cannot read legacy storage.conf");
        const auto text = QString::fromUtf8(configuration.readAll()).trimmed();
        if (!text.contains('='))
            legacy["data_dir"] = text;
        else
            for (auto line : text.split('\n')) {
                line = line.trimmed();
                if (line.isEmpty() || line.startsWith('#'))
                    continue;
                const auto equal = line.indexOf('=');
                if (equal <= 0)
                    continue;
                legacy[line.left(equal).trimmed()] = line.mid(equal + 1).trimmed();
            }
    }
    auto destination = QDir(root_).filePath("data");
    auto source = legacyRoot_;
    const auto custom = legacy.value("data_dir");
    if (!custom.isEmpty() && QFileInfo(custom).isDir()) {
        destination = QFileInfo(custom).absoluteFilePath();
        source = destination;
    }
    ensureDirectory(destination);
    QStringList created;
    try {
        for (auto document : {Document::Accounts, Document::MoeMail, Document::CloudMail, Document::MailNest,
                              Document::ProxyPool, Document::Identities}) {
            const auto name = fileName(document);
            const auto target =
                QDir(document == Document::Identities ? QDir(root_).filePath("cache") : destination).filePath(name);
            if (QFileInfo::exists(target))
                continue;
            const QStringList candidates{name, QString(name).replace(".json", ".dat")};
            for (const auto &candidate : candidates) {
                QFile input(QDir(source).filePath(candidate));
                if (!input.exists())
                    continue;
                if (!input.open(QIODevice::ReadOnly))
                    throw Error(ErrorCode::Storage, "Cannot read legacy " + candidate);
                const auto bytes = input.readAll();
                QJsonParseError error;
                const auto documentValue = QJsonDocument::fromJson(bytes, &error);
                if (error.error != QJsonParseError::NoError || documentValue.isArray() != fallback(document).isArray())
                    throw Error(ErrorCode::Storage, "Invalid legacy " + candidate + "; source file retained");
                writeBytes(target, bytes);
                created.append(target);
                break;
            }
        }
        auto settings = fallback(Document::Settings).object();
        auto runtime = Settings{}.toJson();
        runtime["language"] = legacy.value("language");
        settings["runtime"] = runtime;
        settings["language"] = legacy.value("language");
        settings["proxy"] = legacy.value("proxy");
        settings["resultOutputDir"] = legacy.value("result_output_dir");
        if (destination != QDir(root_).filePath("data"))
            settings["dataDir"] = destination;
        write(QDir(root_).filePath("settings.json"), QJsonDocument(settings));
    } catch (...) {
        for (const auto &path : created)
            QFile::remove(path);
        throw;
    }
}
DataPaths JsonRepository::pathsLocked() const {
    const auto settings = load(QDir(root_).filePath("settings.json"), fallback(Document::Settings)).object();
    auto data = settings.value("dataDir").toString().trimmed();
    auto results = settings.value("resultOutputDir").toString().trimmed();
    if (data.isEmpty())
        data = QDir(root_).filePath("data");
    if (results.isEmpty())
        results = QDir(QStandardPaths::writableLocation(QStandardPaths::DocumentsLocation)).filePath("KiroX");
    return {root_, data, results, QDir(root_).filePath("cache"), QDir(root_).filePath("logs")};
}
DataPaths JsonRepository::paths() const {
    std::lock_guard lock(mutex_);
    return pathsLocked();
}
QString JsonRepository::pathFor(Document document) const {
    const auto p = pathsLocked();
    return QDir(document == Document::Settings     ? root_
                : document == Document::Identities ? p.cache
                                                   : p.data)
        .filePath(fileName(document));
}
QJsonDocument JsonRepository::read(Document document) const {
    std::lock_guard lock(mutex_);
    const auto value = load(pathFor(document), fallback(document));
    if (value.isArray() != fallback(document).isArray())
        throw Error(ErrorCode::Storage, "Unexpected JSON shape: " + fileName(document));
    return value;
}
void JsonRepository::update(Document document, const std::function<void(QJsonDocument &)> &mutation) {
    std::lock_guard lock(mutex_);
    const auto path = pathFor(document);
    auto value = read(document);
    mutation(value);
    if (value.isArray() != fallback(document).isArray())
        throw Error(ErrorCode::InvalidInput, "Unexpected JSON shape: " + fileName(document));
    write(path, value);
}
void JsonRepository::relocateData(const QString &target) {
    std::lock_guard lock(mutex_);
    const auto source = pathsLocked().data;
    const auto destination =
        target.trimmed().isEmpty() ? QDir(root_).filePath("data") : QFileInfo(target).absoluteFilePath();
    if (QDir::cleanPath(source) == QDir::cleanPath(destination))
        return;
    ensureDirectory(destination);
    const auto settingsBefore = read(Document::Settings).object();
    const auto backups = settingsBefore["migrationBackups"].toObject();
    const auto knownDestination = backups[destination].toObject();
    QJsonObject sourceHashes;
    struct PreviousFile {
        QString path;
        QByteArray bytes;
        bool existed;
    };
    std::vector<PreviousFile> modified;
    try {
        for (auto d :
             {Document::Accounts, Document::MoeMail, Document::CloudMail, Document::MailNest, Document::ProxyPool}) {
            const auto from = QDir(source).filePath(fileName(d));
            const auto to = QDir(destination).filePath(fileName(d));
            if (!QFileInfo::exists(from))
                continue;
            QFile input(from);
            if (!input.open(QIODevice::ReadOnly))
                throw Error(ErrorCode::Storage, "Cannot read migration source");
            const auto bytes = input.readAll();
            const auto hash = [](const QByteArray &contents) {
                return QString::fromLatin1(QCryptographicHash::hash(contents, QCryptographicHash::Sha256).toHex());
            };
            sourceHashes[fileName(d)] = hash(bytes);
            const auto parsed = load(from, fallback(d));
            if (parsed.isArray() != fallback(d).isArray())
                throw Error(ErrorCode::Storage, "Invalid migration source " + fileName(d));
            QFile existing(to);
            QByteArray previous;
            const bool existed = existing.exists();
            if (existed) {
                if (!existing.open(QIODevice::ReadOnly))
                    throw Error(ErrorCode::Storage, "Cannot read migration destination");
                previous = existing.readAll();
                existing.close();
                if (previous == bytes)
                    continue;
                // A retained copy may be refreshed only if it has not been edited
                // since this application last left the directory.
                if (knownDestination[fileName(d)].toString() != hash(previous))
                    throw Error(ErrorCode::Conflict,
                                "Destination already contains independently modified " + fileName(d));
            }
            writeBytes(to, bytes);
            modified.push_back({to, previous, existed});
        }
        update(Document::Settings, [&](QJsonDocument &value) {
            auto o = value.object();
            o["dataDir"] = target.trimmed().isEmpty() ? QString{} : destination;
            auto history = o["migrationBackups"].toObject();
            history[source] = sourceHashes;
            history.remove(destination);
            o["migrationBackups"] = history;
            value.setObject(o);
        });
    } catch (...) {
        for (auto it = modified.crbegin(); it != modified.crend(); ++it) {
            if (it->existed)
                writeBytes(it->path, it->bytes);
            else
                QFile::remove(it->path);
        }
        throw;
    }
    // The previous directory is retained as an upgrade rollback copy.
}
void JsonRepository::setResultsDirectory(const QString &target) {
    std::lock_guard lock(mutex_);
    if (!target.trimmed().isEmpty())
        ensureDirectory(QFileInfo(target).absoluteFilePath());
    update(Document::Settings, [&](QJsonDocument &value) {
        auto o = value.object();
        o["resultOutputDir"] = target.trimmed().isEmpty() ? QString{} : QFileInfo(target).absoluteFilePath();
        value.setObject(o);
    });
}
void JsonRepository::saveResult(const QJsonObject &result, const QString &outputDirectory) {
    if (result.value("status").toString() != "success")
        return;
    const auto email = result.value("email").toString();
    if (email.isEmpty())
        throw Error(ErrorCode::InvalidInput, "Result is missing its email");
    std::lock_guard lock(mutex_);
    const auto directory = outputDirectory.isEmpty() ? pathsLocked().results : outputDirectory;
    const auto path = QDir(directory).filePath("accounts.json");
    const auto previous = load(path, QJsonDocument(QJsonArray{}));
    if (!previous.isArray())
        throw Error(ErrorCode::Storage, "Results must be a JSON array");
    QJsonArray items;
    for (const auto &item : previous.array())
        if (item.toObject().value("email").toString() != email)
            items.append(item);
    const auto token = result.value("aws_token").toObject();
    const auto verify = result.value("verify").toObject();
    QJsonObject item{{"provider", "BuilderId"},
                     {"region", "us-east-1"},
                     {"email", email},
                     {"time", QDateTime::currentDateTime().toString("yyyy-MM-dd HH:mm:ss")},
                     {"refreshToken", token.value("refreshToken")},
                     {"clientId", result.value("client_id")},
                     {"clientSecret", result.value("client_secret")}};
    if (!verify.isEmpty()) {
        item["creditUsed"] = verify.value("credit_used");
        item["creditLimit"] = verify.value("credit_limit");
    }
    items.append(item);
    write(path, QJsonDocument(items));
}
} // namespace kirox
