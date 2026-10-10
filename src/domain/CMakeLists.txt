add_library(kirox_domain STATIC settings.cpp mailbox.cpp proxy.cpp mime.cpp fingerprint_crypto.cpp browser_identity.cpp fingerprint.cpp version.cpp)
target_include_directories(kirox_domain PUBLIC "${PROJECT_SOURCE_DIR}/include")
target_link_libraries(kirox_domain PUBLIC Qt6::Core kirox_options)
