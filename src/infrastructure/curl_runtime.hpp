#pragma once
#include "kirox/domain/cancellation.hpp"
#include <curl/curl.h>

namespace kirox::detail {
inline void initializeCurl() {
    struct Runtime {
        Runtime() {
            if (curl_global_init(CURL_GLOBAL_DEFAULT) != CURLE_OK)
                throw Error(ErrorCode::Network, "Cannot initialize native TLS transport");
        }
        ~Runtime() {
            curl_global_cleanup();
        }
    };
    static Runtime runtime;
}
inline void requireCurl(CURLcode code) {
    if (code != CURLE_OK)
        throw Error(ErrorCode::Network, QString::fromUtf8(curl_easy_strerror(code)));
}
inline CURLcode transfer(CURL *easy, CURLM *multi, std::stop_token stop) {
    if (curl_multi_add_handle(multi, easy) != CURLM_OK)
        throw Error(ErrorCode::Network, "Cannot start native request");
    struct Guard {
        CURL *easy;
        CURLM *multi;
        ~Guard() {
            curl_multi_remove_handle(multi, easy);
        }
    } guard{easy, multi};
    int running = 0;
    do {
        checkCancelled(stop);
        if (curl_multi_perform(multi, &running) != CURLM_OK)
            throw Error(ErrorCode::Network, "Native transfer failed");
        if (running && curl_multi_poll(multi, nullptr, 0, 50, nullptr) != CURLM_OK)
            throw Error(ErrorCode::Network, "Native network polling failed");
    } while (running);
    checkCancelled(stop);
    int remaining = 0;
    CURLcode outcome = CURLE_FAILED_INIT;
    while (auto *message = curl_multi_info_read(multi, &remaining))
        if (message->msg == CURLMSG_DONE && message->easy_handle == easy)
            outcome = message->data.result;
    return outcome;
}
} // namespace kirox::detail
