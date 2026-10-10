#include "kirox/infrastructure/curl_transport.hpp"
#include "curl_runtime.hpp"
#include "kirox/domain/cancellation.hpp"
#include "kirox/domain/proxy.hpp"
#include <QDateTime>
#include <algorithm>
#include <array>
#include <climits>
#include <thread>

namespace kirox {
namespace {
using detail::requireCurl;
bool validCookieToken(const QByteArray &text) {
    return !text.isEmpty() && !text.contains('\r') && !text.contains('\n') && !text.contains('\t') &&
           !text.contains(';');
}
} // namespace
struct CurlTransport::Impl {
    CURL *easy = nullptr;
    CURLM *multi = nullptr;
    const std::thread::id owner = std::this_thread::get_id();
    TransportOptions options;
    std::stop_token stop;
    HttpResponse response;
    std::array<char, CURL_ERROR_SIZE> error{};
    bool bodyLimitExceeded = false;
    explicit Impl(TransportOptions value) : options(std::move(value)) {
        detail::initializeCurl();
        easy = curl_easy_init();
        multi = curl_multi_init();
        if (!easy || !multi) {
            if (easy)
                curl_easy_cleanup(easy);
            if (multi)
                curl_multi_cleanup(multi);
            throw Error(ErrorCode::Network, "Cannot allocate native TLS session");
        }
    }
    ~Impl() {
        curl_multi_cleanup(multi);
        curl_easy_cleanup(easy);
    }
    void assertThread() const {
        if (owner != std::this_thread::get_id())
            throw Error(ErrorCode::Conflict, "HTTP session used from another thread");
    }
    static size_t write(char *bytes, size_t size, size_t count, void *context) noexcept {
        auto &self = *static_cast<Impl *>(context);
        const auto length = size * count;
        constexpr size_t maxBody = 64 * 1024 * 1024;
        if (length > maxBody || static_cast<size_t>(self.response.body.size()) > maxBody - length) {
            self.bodyLimitExceeded = true;
            return 0;
        }
        try {
            self.response.body.append(bytes, static_cast<qsizetype>(length));
            return length;
        } catch (...) {
            return 0;
        }
    }
    static size_t header(char *bytes, size_t size, size_t count, void *context) noexcept {
        auto &self = *static_cast<Impl *>(context);
        const auto length = size * count;
        try {
            const QByteArray line(bytes, static_cast<qsizetype>(length));
            if (line.startsWith("HTTP/")) {
                self.response.headers.clear();
                self.response.body.clear();
            } else if (const auto colon = line.indexOf(':'); colon > 0)
                self.response.headers.insert(line.left(colon).trimmed().toLower(), line.mid(colon + 1).trimmed());
            return length;
        } catch (...) {
            return 0;
        }
    }
    static int progress(void *context, curl_off_t, curl_off_t, curl_off_t, curl_off_t) noexcept {
        return static_cast<Impl *>(context)->stop.stop_requested() ? 1 : 0;
    }
};
CurlTransport::CurlTransport(const TransportOptions &options) : impl_(std::make_unique<Impl>(options)) {
    if (!impl_->options.proxy.isEmpty())
        impl_->options.proxy = normalizeProxy(impl_->options.proxy);
    if (!options.browserProfile.isEmpty() && options.browserProfile != "chrome131" &&
        options.browserProfile != "chrome133" && options.browserProfile != "chrome144")
        throw Error(ErrorCode::InvalidInput, "Unsupported browser TLS profile");
}
CurlTransport::~CurlTransport() = default;
HttpResponse CurlTransport::send(const HttpRequest &request, std::stop_token stop) {
    auto &session = *impl_;
    session.assertThread();
    checkCancelled(stop);
    if (!request.url.isValid() || request.url.host().isEmpty() ||
        (request.url.scheme() != "http" && request.url.scheme() != "https") || request.timeout.count() <= 0)
        throw Error(ErrorCode::InvalidInput, "Invalid HTTP request");
    if (request.method.isEmpty() || request.method.contains('\r') || request.method.contains('\n'))
        throw Error(ErrorCode::InvalidInput, "Invalid HTTP method");
    curl_easy_reset(session.easy);
    session.response = {};
    session.stop = stop;
    session.bodyLimitExceeded = false;
    session.error.fill(0);
    if (!session.options.browserProfile.isEmpty()) {
        const auto profile =
            (session.options.browserProfile == "chrome133"   ? QByteArray("chrome133a")
             : session.options.browserProfile == "chrome144" ? QByteArray("chrome142")
                                                             : session.options.browserProfile.toUtf8()) +
            ":no";
        requireCurl(curl_easy_setopt(session.easy, CURLOPT_IMPERSONATE, profile.constData()));
        // Chrome 144 shares the cipher/groups/ALPS surface of Chrome 142.
        // Preserve the baseline's fixed extension order for every native session.
        const char *order =
            session.options.browserProfile == "chrome131"   ? "13-65037-65281-18-27-16-5-10-17513-11-51-35-43-45-0-23"
            : session.options.browserProfile == "chrome133" ? "13-17613-51-18-11-43-5-16-0-65037-27-10-45-23-65281"
                                                            : "51-0-17613-65281-10-27-35-5-23-43-13-18-11-65037-16-45";
        requireCurl(curl_easy_setopt(session.easy, CURLOPT_SSL_PERMUTE_EXTENSIONS, 0L));
        requireCurl(curl_easy_setopt(session.easy, CURLOPT_TLS_EXTENSION_ORDER, order));
        requireCurl(curl_easy_setopt(session.easy, CURLOPT_HTTP2_PSEUDO_HEADERS_ORDER, "masp"));
    }
    const auto url = request.url.toEncoded();
    const auto proxy = session.options.proxy.toUtf8();
    const auto agent = session.options.userAgent.toUtf8();
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_URL, url.constData()));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_PROTOCOLS_STR, "http,https"));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_REDIR_PROTOCOLS_STR,
                                 request.url.scheme() == "https" ? "https" : "http,https"));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_PROXY, proxy.constData()));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_NOPROXY, ""));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_FOLLOWLOCATION, request.followRedirects ? 1L : 0L));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_MAXREDIRS, 10L));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_SSL_VERIFYPEER, 1L));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_SSL_VERIFYHOST, 2L));
#ifdef Q_OS_WIN
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_SSL_OPTIONS, static_cast<long>(CURLSSLOPT_NATIVE_CA)));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_PROXY_SSL_OPTIONS, static_cast<long>(CURLSSLOPT_NATIVE_CA)));
#endif
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_COOKIEFILE, ""));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_ACCEPT_ENCODING, ""));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_NOSIGNAL, 1L));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_TIMEOUT_MS,
                                 static_cast<long>(std::min<int64_t>(request.timeout.count(), INT_MAX))));
    if (!agent.isEmpty())
        requireCurl(curl_easy_setopt(session.easy, CURLOPT_USERAGENT, agent.constData()));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_ERRORBUFFER, session.error.data()));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_WRITEFUNCTION, &Impl::write));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_WRITEDATA, &session));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_HEADERFUNCTION, &Impl::header));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_HEADERDATA, &session));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_NOPROGRESS, 0L));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_XFERINFOFUNCTION, &Impl::progress));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_XFERINFODATA, &session));
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_CUSTOMREQUEST, request.method.constData()));
    if (!request.body.isEmpty() || request.method == "POST" || request.method == "PUT" || request.method == "PATCH") {
        requireCurl(curl_easy_setopt(session.easy, CURLOPT_POSTFIELDS, request.body.constData()));
        requireCurl(
            curl_easy_setopt(session.easy, CURLOPT_POSTFIELDSIZE_LARGE, static_cast<curl_off_t>(request.body.size())));
    }
    if (request.method == "HEAD")
        requireCurl(curl_easy_setopt(session.easy, CURLOPT_NOBODY, 1L));
    std::unique_ptr<curl_slist, decltype(&curl_slist_free_all)> headers(nullptr, &curl_slist_free_all);
    for (auto it = request.headers.cbegin(); it != request.headers.cend(); ++it) {
        if (it.key().isEmpty() || it.key().contains(':') || it.key().contains('\r') || it.key().contains('\n') ||
            it.value().contains('\r') || it.value().contains('\n'))
            throw Error(ErrorCode::InvalidInput, "Invalid HTTP header");
        auto *appended = curl_slist_append(headers.get(), (it.key() + ": " + it.value()).constData());
        if (!appended)
            throw Error(ErrorCode::Network, "Cannot allocate HTTP headers");
        (void)headers.release();
        headers.reset(appended);
    }
    requireCurl(curl_easy_setopt(session.easy, CURLOPT_HTTPHEADER, headers.get()));
    const auto outcome = detail::transfer(session.easy, session.multi, stop);
    if (outcome == CURLE_OPERATION_TIMEDOUT)
        throw Error(ErrorCode::Timeout, "Request timed out");
    if (session.bodyLimitExceeded)
        throw Error(ErrorCode::Protocol, "HTTP response exceeds 64 MiB");
    if (outcome != CURLE_OK)
        throw Error(ErrorCode::Network, QString::fromUtf8(curl_easy_strerror(outcome)));
    long status = 0;
    requireCurl(curl_easy_getinfo(session.easy, CURLINFO_RESPONSE_CODE, &status));
    session.response.status = static_cast<int>(status);
    return std::move(session.response);
}
void CurlTransport::setCookie(const QUrl &origin, const QByteArray &name, const QByteArray &value) {
    impl_->assertThread();
    if (origin.host().isEmpty() || !validCookieToken(name) || !validCookieToken(value))
        throw Error(ErrorCode::InvalidInput, "Invalid session cookie");
    const auto line = origin.host().toUtf8() + "\tFALSE\t/\t" + (origin.scheme() == "https" ? "TRUE" : "FALSE") +
                      "\t0\t" + name + "\t" + value;
    requireCurl(curl_easy_setopt(impl_->easy, CURLOPT_COOKIEFILE, ""));
    requireCurl(curl_easy_setopt(impl_->easy, CURLOPT_COOKIELIST, line.constData()));
}
QByteArray CurlTransport::cookie(const QUrl &origin, const QByteArray &name) const {
    impl_->assertThread();
    curl_slist *list = nullptr;
    requireCurl(curl_easy_getinfo(impl_->easy, CURLINFO_COOKIELIST, &list));
    const std::unique_ptr<curl_slist, decltype(&curl_slist_free_all)> owner(list, &curl_slist_free_all);
    QByteArray best;
    qsizetype bestPath = -1;
    for (auto *entry = list; entry; entry = entry->next) {
        auto line = QByteArray(entry->data);
        if (line.startsWith("#HttpOnly_"))
            line.remove(0, 10);
        const auto fields = line.split('\t');
        if (fields.size() != 7 || fields[5] != name)
            continue;
        auto domain = QString::fromUtf8(fields[0]).toLower();
        if (domain.startsWith('.'))
            domain.remove(0, 1);
        const auto host = origin.host().toLower();
        const auto path = origin.path().isEmpty() ? QString("/") : origin.path();
        const auto cookiePath = QString::fromUtf8(fields[2]);
        if (host != domain && !(fields[1] == "TRUE" && host.endsWith('.' + domain)))
            continue;
        if (!path.startsWith(cookiePath) ||
            (path.size() > cookiePath.size() && !cookiePath.endsWith('/') && path[cookiePath.size()] != '/'))
            continue;
        if (fields[3] == "TRUE" && origin.scheme() != "https")
            continue;
        const auto expiry = fields[4].toLongLong();
        if (expiry != 0 && expiry <= QDateTime::currentSecsSinceEpoch())
            continue;
        if (cookiePath.size() > bestPath) {
            best = fields[6];
            bestPath = cookiePath.size();
        }
    }
    return best;
}
std::unique_ptr<ITransport> CurlTransportFactory::create(const TransportOptions &options) const {
    return std::make_unique<CurlTransport>(options);
}
} // namespace kirox
