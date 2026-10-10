import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ScrollView {
    id: root
    required property var shell
    contentWidth: availableWidth
    clip: true
    ColumnLayout {
        width: root.availableWidth
        spacing: 18
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: appearance.implicitHeight + 48
            dark: root.shell.dark
            ColumnLayout {
                id: appearance
                anchors {
                    left: parent.left
                    right: parent.right
                    top: parent.top
                    margins: 24
                }
                spacing: 16
                Text {
                    text: root.shell.t("外观", "Appearance", "外観")
                    color: root.shell.ink
                    font {
                        pixelSize: 17
                        weight: Font.DemiBold
                    }
                }
                RowLayout {
                    Text {
                        text: root.shell.t("主题", "Theme", "テーマ")
                        color: root.shell.ink
                        Layout.fillWidth: true
                    }
                    GlassComboBox {
                        dark: root.shell.dark
                        model: [root.shell.t("跟随系统", "System", "システム"), root.shell.t("浅色", "Light", "ライト"), root.shell.t("深色", "Dark", "ダーク")]
                        currentIndex: ["system", "light", "dark"].indexOf(backend.settings.theme)
                        onActivated: backend.patchSettings({
                            theme: ["system", "light", "dark"][currentIndex]
                        })
                        Layout.preferredWidth: 200
                    }
                }
                RowLayout {
                    Text {
                        text: root.shell.t("语言", "Language", "言語")
                        color: root.shell.ink
                        Layout.fillWidth: true
                    }
                    GlassComboBox {
                        dark: root.shell.dark
                        model: ["简体中文", "English", "日本語"]
                        currentIndex: ["zh", "en", "ja"].indexOf(backend.language)
                        onActivated: backend.patchSettings({
                            language: ["zh", "en", "ja"][currentIndex]
                        })
                        Layout.preferredWidth: 200
                    }
                }
                Repeater {
                    model: [
                        {
                            key: "reduceMotion",
                            label: root.shell.t("减少动态效果", "Reduce motion", "視差効果を減らす")
                        },
                        {
                            key: "reduceTransparency",
                            label: root.shell.t("减少透明度", "Reduce transparency", "透明度を下げる")
                        }
                    ]
                    delegate: RowLayout {
                        required property var modelData
                        Layout.fillWidth: true
                        Text {
                            text: modelData.label
                            color: root.shell.ink
                            Layout.fillWidth: true
                        }
                        GlassToggle {
                            dark: root.shell.dark
                            reduceMotion: root.shell.reduceMotion
                            Accessible.name: modelData.label
                            checked: backend.settings[modelData.key]
                            onToggled: {
                                const patch = {};
                                patch[modelData.key] = checked;
                                backend.patchSettings(patch);
                            }
                        }
                    }
                }
            }
        }
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: behavior.implicitHeight + 48
            dark: root.shell.dark
            ColumnLayout {
                id: behavior
                anchors {
                    left: parent.left
                    right: parent.right
                    top: parent.top
                    margins: 24
                }
                spacing: 14
                Text {
                    text: root.shell.t("任务与通知", "Tasks and notifications", "タスクと通知")
                    color: root.shell.ink
                    font {
                        pixelSize: 17
                        weight: Font.DemiBold
                    }
                }
                Repeater {
                    model: [
                        {
                            key: "stopOnRisk",
                            label: root.shell.t("遇到风险响应时停止", "Stop on risk responses", "リスク応答で停止")
                        },
                        {
                            key: "soundEnabled",
                            label: root.shell.t("完成提示音", "Completion sound", "完了音")
                        },
                        {
                            key: "desktopNotifications",
                            label: root.shell.t("桌面通知", "Desktop notifications", "デスクトップ通知")
                        },
                        {
                            key: "autoCheckUpdates",
                            label: root.shell.t("自动检查更新", "Check for updates automatically", "更新を自動確認")
                        },
                        {
                            key: "autoProbeProxies",
                            label: root.shell.t("自动检测代理", "Probe proxies automatically", "プロキシを自動テスト")
                        },
                        {
                            key: "persistentLogs",
                            label: root.shell.t("保存运行日志", "Keep activity logs", "ログを保存")
                        }
                    ]
                    delegate: RowLayout {
                        required property var modelData
                        Layout.fillWidth: true
                        Text {
                            text: modelData.label
                            color: root.shell.ink
                            Layout.fillWidth: true
                        }
                        GlassToggle {
                            dark: root.shell.dark
                            reduceMotion: root.shell.reduceMotion
                            Accessible.name: modelData.label
                            checked: backend.settings[modelData.key]
                            onToggled: {
                                const patch = {};
                                patch[modelData.key] = checked;
                                backend.patchSettings(patch);
                            }
                        }
                    }
                }
                RowLayout {
                    Text {
                        text: root.shell.t("提示音音量", "Sound volume", "音量")
                        color: root.shell.ink
                        Layout.fillWidth: true
                    }
                    Slider {
                        from: 0
                        to: 100
                        value: backend.settings.soundVolume
                        Layout.preferredWidth: 180
                        onMoved: backend.patchSettings({
                            soundVolume: Math.round(value)
                        })
                    }
                    Text {
                        text: backend.settings.soundVolume + "%"
                        color: root.shell.muted
                        Layout.preferredWidth: 40
                    }
                }
                RowLayout {
                    Text {
                        text: root.shell.t("验证码等待时间", "Verification timeout", "認証コードの待機時間")
                        color: root.shell.ink
                        Layout.fillWidth: true
                    }
                    GlassComboBox {
                        dark: root.shell.dark
                        model: ["60s", "120s", "180s", "300s"]
                        currentIndex: [60, 120, 180, 300].indexOf(backend.settings.otpTimeoutSeconds)
                        onActivated: backend.patchSettings({
                            otpTimeoutSeconds: [60, 120, 180, 300][currentIndex]
                        })
                    }
                }
                RowLayout {
                    Text {
                        text: root.shell.t("重试策略", "Retry policy", "再試行ポリシー")
                        color: root.shell.ink
                        Layout.fillWidth: true
                    }
                    GlassComboBox {
                        dark: root.shell.dark
                        model: [root.shell.t("快速", "Fast", "高速"), root.shell.t("标准", "Standard", "標準"), root.shell.t("稳定", "Stable", "安定")]
                        currentIndex: ["fast", "standard", "stable"].indexOf(backend.settings.retryProfile)
                        onActivated: backend.patchSettings({
                            retryProfile: ["fast", "standard", "stable"][currentIndex]
                        })
                    }
                }
                RowLayout {
                    Text {
                        text: root.shell.t("邮箱代理", "Mailbox proxy", "メール用プロキシ")
                        color: root.shell.ink
                        Layout.fillWidth: true
                    }
                    GlassComboBox {
                        dark: root.shell.dark
                        model: [root.shell.t("跟随任务", "Follow task", "タスクに従う"), root.shell.t("直连", "Direct", "直接接続"), root.shell.t("自定义", "Custom", "カスタム")]
                        currentIndex: ["follow-task", "direct", "custom"].indexOf(backend.settings.emailProxyMode)
                        onActivated: backend.patchSettings({
                            emailProxyMode: ["follow-task", "direct", "custom"][currentIndex]
                        })
                    }
                }
                TextField {
                    visible: backend.settings.emailProxyMode === "custom"
                    Layout.fillWidth: true
                    text: backend.settings.emailProxy
                    placeholderText: "http://user:password@host:port"
                    onEditingFinished: backend.patchSettings({
                        emailProxy: text
                    })
                }
                RowLayout {
                    Text {
                        text: root.shell.t("日志保留天数", "Log retention (days)", "ログ保持日数")
                        color: root.shell.ink
                        Layout.fillWidth: true
                    }
                    GlassSpinBox {
                        from: 1
                        to: 90
                        value: backend.settings.logRetentionDays
                        editable: true
                        onValueModified: backend.patchSettings({
                            logRetentionDays: value
                        })
                    }
                }
                RowLayout {
                    Text {
                        text: root.shell.t("MoeMail 有效期（分钟）", "MoeMail expiry (minutes)", "MoeMail 有効期間（分）")
                        color: root.shell.ink
                        Layout.fillWidth: true
                    }
                    GlassSpinBox {
                        from: 10
                        to: 1440
                        value: backend.settings.moeMailExpiryMinutes
                        editable: true
                        onValueModified: backend.patchSettings({
                            moeMailExpiryMinutes: value
                        })
                    }
                }
            }
        }
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: folders.implicitHeight + 48
            dark: root.shell.dark
            ColumnLayout {
                id: folders
                anchors {
                    left: parent.left
                    right: parent.right
                    top: parent.top
                    margins: 24
                }
                spacing: 18
                Text {
                    text: root.shell.t("数据与目录", "Data and folders", "データとフォルダー")
                    color: root.shell.ink
                    font {
                        pixelSize: 17
                        weight: Font.DemiBold
                    }
                }
                Repeater {
                    model: [
                        {
                            key: "data",
                            label: root.shell.t("数据目录", "Data folder", "データフォルダー")
                        },
                        {
                            key: "results",
                            label: root.shell.t("结果输出目录", "Results folder", "結果フォルダー")
                        }
                    ]
                    delegate: ColumnLayout {
                        required property var modelData
                        Layout.fillWidth: true
                        spacing: 7
                        Text {
                            text: modelData.label
                            color: root.shell.ink
                            font.pixelSize: 13
                        }
                        Text {
                            text: backend.directories[modelData.key]
                            color: root.shell.muted
                            font.pixelSize: 11
                            Layout.fillWidth: true
                            elide: Text.ElideMiddle
                        }
                        RowLayout {
                            GlassButton {
                                text: root.shell.t("选择目录", "Choose folder", "フォルダーを選択")
                                dark: root.shell.dark
                                onClicked: backend.selectDirectory(modelData.key)
                            }
                            GlassButton {
                                text: root.shell.t("恢复默认", "Reset", "既定に戻す")
                                dark: root.shell.dark
                                onClicked: backend.resetDirectory(modelData.key)
                            }
                        }
                    }
                }
            }
        }
    }
}
