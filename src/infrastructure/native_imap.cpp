#include "curl_runtime.hpp"
#include "kirox/domain/proxy.hpp"
#include "kirox/infrastructure/imap_mailbox.hpp"
#include <climits>
#include <thread>

namespace kirox {
namespace {
class NativeImapConnection final : public IImapConnection {
  public:
    NativeImapConnection(QString endpoint, QString email, QString token, const TransportOptions &options,
                         QString caFile)
        : endpoint_(std::move(endpoint)), email_(std::move(email)), token_(std::move(token)),
          proxy_(options.proxy.isEmpty() ? QString{} : normalizeProxy(options.proxy)), caFile_(std::move(caFile)) {
        detail::initializeCurl();
        easy_.reset(curl_easy_init());
        multi_.reset(curl_multi_init());
        if (!easy_ || !multi_)
            throw Error(ErrorCode::Network, "Cannot allocate IMAP session");
        if (QUrl(endpoint_).scheme() != "imaps")
            throw Error(ErrorCode::InvalidInput, "IMAP requires a TLS endpoint");
        while (endpoint_.endsWith('/'))
            endpoint_.chop(1);
    }
    QByteArray command(const QString &folder, const QByteArray &command, std::stop_token stop,
                       std::chrono::milliseconds timeout) override {
        if (command.isEmpty() || command.contains('\r') || command.contains('\n'))
            throw Error(ErrorCode::InvalidInput, "Invalid IMAP command");
        return send(endpoint_ + "/" + QString::fromUtf8(QUrl::toPercentEncoding(folder)), command, stop, timeout);
    }
    QByteArray message(const QString &folder, quint32 uid, std::stop_token stop,
                       std::chrono::milliseconds timeout) override {
        if (uid == 0)
            throw Error(ErrorCode::InvalidInput, "Invalid message UID");
        return send(endpoint_ + "/" + QString::fromUtf8(QUrl::toPercentEncoding(folder)) +
                        "/;UID=" + QString::number(uid),
                    {}, stop, timeout);
    }

  private:
    QByteArray send(const QString &url, const QByteArray &command, std::stop_token stop,
                    std::chrono::milliseconds timeout) {
        if (owner_ != std::this_thread::get_id())
            throw Error(ErrorCode::Conflict, "IMAP session used from another thread");
        checkCancelled(stop);
        if (timeout.count() <= 0)
            throw Error(ErrorCode::Timeout, "IMAP deadline exceeded");
        curl_easy_reset(easy_.get());
        body_.clear();
        overflow_ = false;
        const auto address = QUrl(url).toEncoded();
        const auto user = email_.toUtf8();
        const auto token = token_.toUtf8();
        const auto proxy = proxy_.toUtf8();
        const auto ca = caFile_.toUtf8();
        const auto set = [&](auto option, auto value) {
            detail::requireCurl(curl_easy_setopt(easy_.get(), option, value));
        };
        set(CURLOPT_URL, address.constData());
        set(CURLOPT_PROTOCOLS_STR, "imaps");
        set(CURLOPT_USERNAME, user.constData());
        set(CURLOPT_XOAUTH2_BEARER, token.constData());
        set(CURLOPT_LOGIN_OPTIONS, "AUTH=XOAUTH2");
        set(CURLOPT_PROXY, proxy.constData());
        set(CURLOPT_NOPROXY, "");
        set(CURLOPT_SSL_VERIFYPEER, 1L);
        set(CURLOPT_SSL_VERIFYHOST, 2L);
        if (!ca.isEmpty())
            set(CURLOPT_CAINFO, ca.constData());
#ifdef Q_OS_WIN
        set(CURLOPT_SSL_OPTIONS, static_cast<long>(CURLSSLOPT_NATIVE_CA));
        set(CURLOPT_PROXY_SSL_OPTIONS, static_cast<long>(CURLSSLOPT_NATIVE_CA));
#endif
        set(CURLOPT_NOSIGNAL, 1L);
        set(CURLOPT_TIMEOUT_MS, static_cast<long>(std::min<int64_t>(timeout.count(), INT_MAX)));
        set(CURLOPT_WRITEFUNCTION, &NativeImapConnection::write);
        set(CURLOPT_WRITEDATA, this);
        if (!command.isEmpty())
            set(CURLOPT_CUSTOMREQUEST, command.constData());
        const auto result = detail::transfer(easy_.get(), multi_.get(), stop);
        if (result == CURLE_OPERATION_TIMEDOUT)
            throw Error(ErrorCode::Timeout, "IMAP request timed out");
        if (overflow_)
            throw Error(ErrorCode::Protocol, "IMAP message exceeds 16 MiB");
        detail::requireCurl(result);
        return body_;
    }
    static size_t write(char *data, size_t size, size_t count, void *context) noexcept {
        auto &self = *static_cast<NativeImapConnection *>(context);
        const auto length = size * count;
        constexpr size_t maximum = 16 * 1024 * 1024;
        if (length > maximum || static_cast<size_t>(self.body_.size()) > maximum - length) {
            self.overflow_ = true;
            return 0;
        }
        try {
            self.body_.append(data, static_cast<qsizetype>(length));
            return length;
        } catch (...) {
            return 0;
        }
    }
    QString endpoint_, email_, token_, proxy_, caFile_;
    const std::thread::id owner_ = std::this_thread::get_id();
    std::unique_ptr<CURL, decltype(&curl_easy_cleanup)> easy_{nullptr, &curl_easy_cleanup};
    std::unique_ptr<CURLM, decltype(&curl_multi_cleanup)> multi_{nullptr, &curl_multi_cleanup};
    QByteArray body_;
    bool overflow_ = false;
};
} // namespace
std::unique_ptr<IImapConnection> NativeImapFactory::create(const QString &email, const QString &token,
                                                           const TransportOptions &options) const {
    return std::make_unique<NativeImapConnection>(endpoint_, email, token, options, caFile_);
}
} // namespace kirox
