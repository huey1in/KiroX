qt_add_executable(kirox main.cpp controller.cpp controller.hpp desktop_instance.cpp desktop_instance.hpp desktop_feedback.cpp desktop_feedback.hpp)
if(WIN32)
  target_sources(kirox PRIVATE ../../packaging/windows/kirox.rc)
  add_custom_command(TARGET kirox POST_BUILD COMMAND ${CMAKE_COMMAND} -E copy_if_different
    "$<TARGET_FILE:KiroX::Curl>" "$<TARGET_FILE_DIR:kirox>")
  install(FILES "$<TARGET_FILE:KiroX::Curl>" DESTINATION bin)
endif()
if(APPLE)
  # macdeployqt excludes offscreen by default. Include it before dependency
  # deployment so its install names are rewritten with the other bundle dylibs.
  install(FILES "$<TARGET_FILE:Qt6::QOffscreenIntegrationPlugin>" DESTINATION KiroX.app/Contents/PlugIns/platforms)
  target_sources(kirox PRIVATE ../../assets/kirox.icns)
  set_source_files_properties(../../assets/kirox.icns PROPERTIES MACOSX_PACKAGE_LOCATION Resources)
  set_target_properties(kirox PROPERTIES MACOSX_BUNDLE_ICON_FILE kirox.icns OUTPUT_NAME KiroX)
endif()
if(UNIX AND NOT APPLE)
  install(FILES "$<TARGET_FILE:Qt6::QOffscreenIntegrationPlugin>" DESTINATION plugins/platforms)
  install(IMPORTED_RUNTIME_ARTIFACTS KiroX::Curl LIBRARY DESTINATION lib)
  set_target_properties(kirox PROPERTIES INSTALL_RPATH "$ORIGIN/../lib")
  install(FILES ../../packaging/linux/kirox.desktop DESTINATION share/applications)
  install(FILES ../../assets/kirox-light.svg DESTINATION share/icons/hicolor/scalable/apps RENAME kirox.svg)
endif()
target_link_libraries(kirox PRIVATE kirox_application kirox_infrastructure Qt6::Gui Qt6::Quick Qt6::QuickControls2 Qt6::Svg Qt6::Widgets Qt6::Multimedia)
qt_add_resources(kirox feedback PREFIX "/kirox" BASE "../../assets" FILES "../../assets/completion.wav")
set_target_properties(kirox PROPERTIES WIN32_EXECUTABLE TRUE MACOSX_BUNDLE TRUE
    MACOSX_BUNDLE_BUNDLE_NAME KiroX MACOSX_BUNDLE_GUI_IDENTIFIER com.huey1in.kirox)
foreach(qml Main GlassPane GlassButton GlassDialog GlassSpinBox GlassComboBox GlassToggle GlassTextField ServicesPage LineIcon ContentCard OverviewPage TasksPage ActivityPage AccountsPage ProxyPage SettingsPage AboutPage)
    set_source_files_properties("../../ui/${qml}.qml" PROPERTIES QT_RESOURCE_ALIAS "${qml}.qml")
endforeach()
set_source_files_properties("../../assets/kirox-light.svg" PROPERTIES QT_RESOURCE_ALIAS "assets/kirox-light.svg")
set_source_files_properties("../../assets/wx.jpg" PROPERTIES QT_RESOURCE_ALIAS "assets/wx.jpg")
set_source_files_properties("../../assets/zfb.jpg" PROPERTIES QT_RESOURCE_ALIAS "assets/zfb.jpg")
qt_add_qml_module(kirox
    URI KiroX VERSION 1.0
    QML_FILES
        ../../ui/Main.qml ../../ui/GlassPane.qml ../../ui/GlassButton.qml ../../ui/GlassDialog.qml ../../ui/GlassSpinBox.qml
        ../../ui/GlassComboBox.qml ../../ui/GlassToggle.qml ../../ui/LineIcon.qml ../../ui/GlassTextField.qml ../../ui/ServicesPage.qml
        ../../ui/ContentCard.qml ../../ui/OverviewPage.qml ../../ui/TasksPage.qml ../../ui/ActivityPage.qml ../../ui/AccountsPage.qml
        ../../ui/ProxyPage.qml ../../ui/SettingsPage.qml ../../ui/AboutPage.qml
    RESOURCES ../../assets/kirox-light.svg ../../assets/wx.jpg ../../assets/zfb.jpg
)
install(TARGETS kirox BUNDLE DESTINATION . RUNTIME DESTINATION bin)
if(WIN32)
  qt_generate_deploy_qml_app_script(TARGET kirox OUTPUT_SCRIPT deploy_script
    NO_UNSUPPORTED_PLATFORM_ERROR NO_TRANSLATIONS
    DEPLOY_TOOL_OPTIONS --include-plugins qoffscreen --no-translations --skip-plugin-types qmltooling)
else()
  qt_generate_deploy_qml_app_script(TARGET kirox OUTPUT_SCRIPT deploy_script
    NO_UNSUPPORTED_PLATFORM_ERROR NO_TRANSLATIONS)
endif()
install(SCRIPT ${deploy_script})
