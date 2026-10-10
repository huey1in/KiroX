#include "kirox/domain/browser_identity.hpp"
#include <QCryptographicHash>
#include <QDateTime>
#include <QJsonArray>
#include <QRandomGenerator>
#include <QSize>
#include <QUrl>
#include <algorithm>
#include <array>
#include <bit>
#include <limits>

namespace kirox {
namespace {
int random(int low, int high) {
    return low + QRandomGenerator::system()->bounded(high - low + 1);
}
QString choose(const QStringList &values) {
    return values.at(random(0, int(values.size()) - 1));
}
const QStringList prefixes{"X10", "X19", "X42", "X55", "X73", "X81", "X96"};
const QStringList pluginNames{"PDF Viewer", "Chrome PDF Viewer", "Chromium PDF Viewer", "Microsoft Edge PDF Viewer",
                              "WebKit built-in PDF"};
QJsonArray plugins() {
    QJsonArray result;
    for (const auto &name : pluginNames)
        result.append(QJsonObject{
            {"name", name}, {"filename", "internal-pdf-viewer"}, {"description", "Portable Document Format"}});
    return result;
}
void generateCanvas(QJsonObject &data) {
    std::array<int, 256> bins{};
    bins[0] = random(5000, 15000);
    bins[255] = random(6000, 16000);
    const auto first = random(100, 105), second = random(150, 157);
    bins[first] = random(500, 699);
    bins[second] = random(400, 699);
    int remaining = 36000 - bins[0] - bins[255] - bins[first] - bins[second];
    struct Range {
        int low, high, average;
    };
    for (const auto [low, high, average] : std::array<Range, 6>{
             {{1, 30, 30}, {31, 70, 20}, {71, 99, 25}, {106, 149, 15}, {158, 200, 18}, {201, 254, 25}}}) {
        const auto center = (low + high) / 2, distance = std::max(1, (high - low) / 2);
        for (int i = low; i <= high && remaining > 0; ++i) {
            const auto scale = std::max(0.2, double(distance - std::abs(i - center)) / distance);
            const auto base = int(average * scale);
            bins[i] = std::min(remaining, std::max(2, base + random(0, base / 2) - base / 4));
            remaining -= bins[i];
        }
    }
    for (int i = 1; i < 255 && remaining > 0; ++i)
        if (bins[i] == 0) {
            bins[i] = std::min(remaining, random(2, 9));
            remaining -= bins[i];
        }
    bins[0] += remaining;
    QJsonArray histogram;
    for (const auto count : bins)
        histogram.append(count);
    data.insert("HistogramBase", histogram);
    // CRC32's full signed result domain, matching the legacy serialized surface.
    data.insert("CanvasHash", qint64(std::bit_cast<qint32>(QRandomGenerator::system()->generate())));
}
} // namespace

BrowserIdentity BrowserIdentity::generate() {
    const auto major = choose({"131", "133", "144"});
    const auto version = major + ".0.0.0";
    const auto grease = choose(
        {"Not_A Brand", "Not(A:Brand", "Not-A.Brand", "Not)A;Brand", "Not/A)Brand", "Not A;Brand", "Not?A_Brand"});
    const auto ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" +
                    version + " Safari/537.36";
    const auto secUa = QString("\"%1\";v=\"%2\", \"Chromium\";v=\"%3\", \"Google Chrome\";v=\"%3\"")
                           .arg(grease)
                           .arg(random(8, 99))
                           .arg(major);
    struct Family {
        QString vendor, prefix;
        QStringList models;
    };
    const std::array<Family, 3> families{
        {{"Intel",
          "Intel(R) ",
          {"UHD Graphics 630", "UHD Graphics 730", "UHD Graphics 770", "HD Graphics 530", "HD Graphics 620",
           "HD Graphics 630", "Iris(R) Xe Graphics", "Iris(R) Xe Graphics (0x000046A6)", "Iris(R) Plus Graphics",
           "UHD Graphics", "HD Graphics 520", "UHD Graphics 620", "Iris(R) Plus Graphics 655",
           "Iris(R) Plus Graphics 640", "HD Graphics 4600", "HD Graphics 5500"}},
         {"NVIDIA", "NVIDIA ", {"GeForce GTX 960",      "GeForce GTX 970",
                                "GeForce GTX 980 Ti",   "GeForce GTX 1050 Ti",
                                "GeForce GTX 1060 6GB", "GeForce GTX 1070",
                                "GeForce GTX 1080",     "GeForce GTX 1080 Ti",
                                "GeForce GTX 1650",     "GeForce GTX 1660 Super",
                                "GeForce RTX 2060",     "GeForce RTX 2070",
                                "GeForce RTX 2080",     "GeForce RTX 3050 Laptop GPU",
                                "GeForce RTX 3060",     "GeForce RTX 3060 Ti",
                                "GeForce RTX 3070",     "GeForce RTX 3080",
                                "GeForce RTX 4060",     "GeForce RTX 4070",
                                "GeForce RTX 4080",     "GeForce RTX 4090"}},
         {"AMD",
          "AMD ",
          {"Radeon RX 580", "Radeon RX 5600 XT", "Radeon RX 5700 XT", "Radeon RX 6600 XT", "Radeon RX 6700 XT",
           "Radeon RX 6800 XT", "Radeon RX 7600", "Radeon RX 7800 XT", "Radeon RX 7900 XTX", "Radeon Vega 8 Graphics",
           "Radeon(TM) Graphics", "Radeon RX Vega 11 Graphics", "Radeon RX 5500 XT", "Radeon R9 390",
           "Radeon RX 480"}}}};
    const auto &gpu = families.at(random(0, 2));
    const auto vendor = "Google Inc. (" + gpu.vendor + ")";
    const auto model =
        QString("ANGLE (%1, %2%3 Direct3D11 vs_5_0 ps_5_0, D3D11)").arg(gpu.vendor, gpu.prefix, choose(gpu.models));
    const QList<QList<QSize>> screenPools{
        {{1366, 768}, {1536, 864}, {1600, 900}, {1920, 1080}, {2560, 1440}, {3840, 2160}},
        {{1440, 900}, {1680, 1050}, {1920, 1200}, {2560, 1600}},
        {{2560, 1080}, {3440, 1440}},
        {{1280, 720}, {1360, 768}, {2880, 1800}}};
    const auto poolIndex = std::array<int, 6>{0, 0, 0, 1, 2, 3}.at(random(0, 5));
    const auto &pool = screenPools.at(poolIndex);
    const auto screen = pool.at(random(0, int(pool.size()) - 1));
    const QList<int> memories = gpu.vendor == "Intel" ? QList<int>{8, 16, 32} : QList<int>{4, 8, 16, 32};
    const QList<int> concurrency =
        gpu.vendor == "Intel" ? QList<int>{4, 8, 12, 16, 20} : QList<int>{4, 8, 12, 16, 20, 24, 32};
    QStringList extensions{"ANGLE_instanced_arrays",
                           "EXT_blend_minmax",
                           "EXT_color_buffer_half_float",
                           "EXT_float_blend",
                           "EXT_frag_depth",
                           "EXT_shader_texture_lod",
                           "EXT_texture_filter_anisotropic",
                           "EXT_sRGB",
                           "KHR_parallel_shader_compile",
                           "OES_element_index_uint",
                           "OES_fbo_render_mipmap",
                           "OES_standard_derivatives",
                           "OES_texture_float",
                           "OES_texture_float_linear",
                           "OES_texture_half_float",
                           "OES_texture_half_float_linear",
                           "OES_vertex_array_object",
                           "WEBGL_color_buffer_float",
                           "WEBGL_compressed_texture_s3tc",
                           "WEBGL_compressed_texture_s3tc_srgb",
                           "WEBGL_debug_renderer_info",
                           "WEBGL_debug_shaders",
                           "WEBGL_depth_texture",
                           "WEBGL_draw_buffers",
                           "WEBGL_lose_context",
                           "WEBGL_multi_draw"};
    QStringList optional{"EXT_disjoint_timer_query",     "EXT_texture_compression_bptc",
                         "EXT_texture_compression_rgtc", "WEBGL_compressed_texture_astc",
                         "WEBGL_compressed_texture_etc", "OES_draw_buffers_indexed",
                         "EXT_color_buffer_float"};
    for (int i = 0, count = random(0, 4); i < count; ++i)
        extensions.append(optional.takeAt(random(0, int(optional.size()) - 1)));
    extensions.sort();
    QJsonObject data{{"ChromeVer", version},
                     {"UA", ua},
                     {"SecUA", secUa},
                     {"GPUVendor", vendor},
                     {"GPUModel", model},
                     {"WebGLExts", QJsonArray::fromStringList(extensions)},
                     {"MathTan", "-1.4214488238747245"},
                     {"MathSin", "0.8178819121159085"},
                     {"MathCos", "-0.5753861119575491"},
                     {"Plugins", plugins()},
                     {"Screen", QJsonObject{{"Width", screen.width()},
                                            {"Height", screen.height()},
                                            {"AvailWidth", screen.width()},
                                            {"AvailHeight", screen.height() - (random(32, 48) / 8) * 8},
                                            {"ColorDepth", 24}}},
                     {"DeviceMemory", memories.at(random(0, int(memories.size()) - 1))},
                     {"HardwareConcurrency", concurrency.at(random(0, int(concurrency.size()) - 1))},
                     {"Platform", "Win32"},
                     {"TimezoneHours", QDateTime::currentDateTime().offsetFromUtc() / 3600}};
    generateCanvas(data);
    return BrowserIdentity(data).newSession();
}

bool BrowserIdentity::valid() const {
    const auto major = majorVersion();
    if (!QStringList{"131", "133", "144"}.contains(major) ||
        !userAgent().contains("Chrome/" + properties_.value("ChromeVer").toString()) ||
        !securityUserAgent().contains("\"Chromium\";v=\"" + major + "\"") ||
        !securityUserAgent().contains("\"Google Chrome\";v=\"" + major + "\""))
        return false;
    const auto screen = properties_.value("Screen").toObject();
    if (properties_.value("Platform") != "Win32" || screen.value("ColorDepth") != 24 ||
        screen.value("Width").toInt() <= 0 || screen.value("Height").toInt() <= 0 ||
        screen.value("AvailWidth") != screen.value("Width") || screen.value("AvailHeight").toInt() <= 0 ||
        screen.value("AvailHeight").toInt() > screen.value("Height").toInt())
        return false;
    if (!QList<int>{4, 8, 16, 32}.contains(properties_.value("DeviceMemory").toInt()) ||
        !QList<int>{4, 8, 12, 16, 20, 24, 32}.contains(properties_.value("HardwareConcurrency").toInt()) ||
        properties_.value("MathTan") != "-1.4214488238747245" || properties_.value("MathSin") != "0.8178819121159085" ||
        properties_.value("MathCos") != "-0.5753861119575491" || properties_.value("Plugins") != plugins())
        return false;
    const auto histogram = properties_.value("HistogramBase").toArray();
    if (histogram.size() != 256 || !properties_.value("CanvasHash").isDouble() ||
        properties_.value("GPUVendor").toString().isEmpty() || properties_.value("GPUModel").toString().isEmpty() ||
        properties_.value("WebGLExts").toArray().isEmpty())
        return false;
    qint64 sum = 0;
    for (const auto value : histogram) {
        const auto number = value.toInteger(-1);
        if (number < 0 || number > 36000)
            return false;
        sum += number;
    }
    const auto hash = properties_.value("CanvasHash").toInteger();
    return sum == 36000 && hash >= std::numeric_limits<qint32>::min() && hash <= std::numeric_limits<qint32>::max();
}
QString BrowserIdentity::majorVersion() const {
    return properties_.value("ChromeVer").toString().section('.', 0, 0);
}
QString BrowserIdentity::userAgent() const {
    return properties_.value("UA").toString();
}
QString BrowserIdentity::securityUserAgent() const {
    return properties_.value("SecUA").toString();
}
BrowserIdentity BrowserIdentity::newSession(bool resampleCanvas) const {
    auto data = properties_;
    data.insert("LsubidPrefixSignin", choose(prefixes));
    data.insert("LsubidPrefixProfile", choose(prefixes));
    const auto hash = QCryptographicHash::hash(userAgent().toUtf8(), QCryptographicHash::Sha256).toHex();
    data.insert("WebpackHash", QString::fromLatin1(hash.mid(random(0, 19), 10)));
    if (resampleCanvas)
        generateCanvas(data);
    return BrowserIdentity(data);
}
QString browserIdentityKey(const QString &proxy) {
    const auto text = proxy.trimmed();
    if (text.isEmpty())
        return "direct";
    QUrl url(text, QUrl::StrictMode);
    if (!url.isValid() || url.host().isEmpty())
        return "raw:" + QString::fromLatin1(
                            QCryptographicHash::hash(text.toUtf8(), QCryptographicHash::Sha256).toHex().left(16));
    // Keep explicit port and IPv6 brackets, omitting scheme and credentials.
    url.setUserInfo({});
    return url.authority().toLower();
}
} // namespace kirox
