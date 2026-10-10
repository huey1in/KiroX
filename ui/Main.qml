import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ApplicationWindow {
    id: window
    width: 1180
    height: 780
    minimumWidth: 820
    minimumHeight: 580
    visible: true
    title: "KiroX"
    property bool dark: backend.settings.theme === "dark" || (backend.settings.theme === "system" && Qt.styleHints.colorScheme === Qt.Dark)
    property bool reduceMotion: !!backend.settings.reduceMotion
    property bool reduceTransparency: !!backend.settings.reduceTransparency
    property string page: initialPage
    property color ink: dark ? "#f1f5ff" : "#24334e"
    property color muted: dark ? "#a4b2c9" : "#79859b"
    property string uiLanguage: backend.language
    function t(zh, en, ja) {
        return uiLanguage === "zh" ? zh : uiLanguage === "ja" ? ja : en;
    }
    function statusLabel(status) {
        return status === "success" ? t("成功", "Succeeded", "成功") : status === "failed" ? t("失败", "Failed", "失敗") : status === "cancelled" ? t("已取消", "Cancelled", "キャンセル済み") : status === "running" ? t("运行中", "Running", "実行中") : status === "queued" ? t("等待中", "Queued", "待機中") : status;
    }
    property var navigation: [
        {
            key: "overview",
            label: t("概览", "Overview", "概要")
        },
        {
            key: "tasks",
            label: t("注册任务", "Tasks", "登録タスク")
        },
        {
            key: "accounts",
            label: t("邮箱池", "Mailboxes", "メールプール")
        },
        {
            key: "services",
            label: t("邮箱服务", "Mail services", "メールサービス")
        },
        {
            key: "proxies",
            label: t("IP 管理", "Proxies", "プロキシ")
        },
        {
            key: "logs",
            label: t("运行日志", "Activity", "ログ")
        },
        {
            key: "settings",
            label: t("设置", "Settings", "設定")
        },
        {
            key: "about",
            label: t("关于", "About", "このアプリについて")
        }
    ]
    property bool confirmedQuit: false
    onClosing: function (close) {
        if (backend.batchStatus.running && !confirmedQuit) {
            close.accepted = false;
            quitDialog.open();
        }
    }
    GlassDialog {
        id: quitDialog
        parent: Overlay.overlay
        anchors.centerIn: parent
        width: Math.min(440, window.width - 40)
        modal: true
        title: window.t("停止任务并退出？", "Stop the batch and quit?", "タスクを停止して終了しますか？")
        acceptText: window.t("停止并退出", "Stop and quit", "停止して終了")
        cancelText: window.t("继续运行", "Keep running", "実行を続ける")
        contentItem: Text {
            text: window.t("正在运行的任务将被取消，已保存的结果会保留。", "Running tasks will be cancelled. Saved results will be kept.", "実行中のタスクをキャンセルします。保存済みの結果は保持します。")
            color: window.ink
            wrapMode: Text.Wrap
            font.pixelSize: 13
        }
        onAccepted: {
            backend.stopBatch();
            window.confirmedQuit = true;
            window.close();
        }
    }
    color: dark ? "#161c28" : "#e7edf8"
    palette.window: dark ? "#283247" : "#fbfcff"
    palette.windowText: ink
    palette.text: ink
    palette.buttonText: ink
    palette.button: dark ? "#354156" : "#f0f4fb"
    palette.base: dark ? "#354156" : "#f0f4fb"
    palette.highlight: "#557bf3"
    palette.highlightedText: "white"
    Item {
        id: backdrop
        anchors.fill: parent
        Rectangle {
            anchors.fill: parent
            gradient: Gradient {
                GradientStop {
                    position: 0
                    color: window.dark ? "#182234" : "#e2eafd"
                }
                GradientStop {
                    position: 0.55
                    color: window.dark ? "#222537" : "#f5f0ff"
                }
                GradientStop {
                    position: 1
                    color: window.dark ? "#152c33" : "#e5f4f4"
                }
            }
        }
        Rectangle {
            x: parent.width * 0.48
            y: -160
            width: 680
            height: 680
            radius: 340
            color: window.dark ? "#1328396c" : "#20a6bbff"
        }
        Rectangle {
            x: -260
            y: parent.height * 0.4
            width: 680
            height: 680
            radius: 340
            color: window.dark ? "#0f426b64" : "#28a9e3da"
        }
    }
    RowLayout {
        anchors {
            fill: parent
            margins: 20
        }
        spacing: 20
        GlassPane {
            Layout.fillHeight: true
            Layout.preferredWidth: 212
            backgroundSource: backdrop
            dark: window.dark
            reduceTransparency: window.reduceTransparency
            ColumnLayout {
                anchors {
                    fill: parent
                    margins: 16
                }
                spacing: 6
                RowLayout {
                    Layout.topMargin: 12
                    Layout.bottomMargin: window.height < 650 ? 10 : 30
                    spacing: 12
                    Image {
                        source: "assets/kirox-light.svg"
                        Layout.preferredWidth: 42
                        Layout.preferredHeight: 42
                    }
                    ColumnLayout {
                        spacing: 1
                        Text {
                            text: "KiroX"
                            color: window.ink
                            font {
                                pixelSize: 21
                                weight: Font.Bold
                            }
                        }
                        Text {
                            text: window.t("工作空间", "Workspace", "ワークスペース")
                            color: window.muted
                            font.pixelSize: 11
                        }
                    }
                }
                Repeater {
                    model: window.navigation
                    delegate: Button {
                        required property var modelData
                        Layout.fillWidth: true
                        implicitHeight: window.height < 650 ? 34 : 46
                        onClicked: window.page = modelData.key
                        Accessible.name: modelData.label
                        contentItem: RowLayout {
                            spacing: 12
                            LineIcon {
                                name: modelData.key
                                ink: window.page === modelData.key ? "#587df5" : window.muted
                                Layout.preferredWidth: 22
                                Layout.preferredHeight: 22
                            }
                            Text {
                                text: modelData.label
                                color: window.ink
                                font {
                                    pixelSize: 13
                                    weight: window.page === modelData.key ? Font.DemiBold : Font.Normal
                                }
                                Layout.fillWidth: true
                            }
                        }
                        background: Rectangle {
                            radius: 15
                            color: window.page === modelData.key ? (window.dark ? "#554d638f" : "#ddffffff") : parent.hovered ? "#22ffffff" : "transparent"
                            border.width: parent.activeFocus ? 2 : window.page === modelData.key ? 1 : 0
                            border.color: parent.activeFocus ? "#7799ff" : window.dark ? "#55687b9d" : "white"
                        }
                    }
                }
                Item {
                    Layout.fillHeight: true
                }
                Rectangle {
                    Layout.fillWidth: true
                    height: 1
                    color: window.dark ? "#33536786" : "#66a6b2c8"
                }
                RowLayout {
                    Layout.topMargin: 12
                    GlassButton {
                        dark: window.dark
                        implicitWidth: 48
                        Accessible.name: window.t("切换主题", "Toggle theme", "テーマを切り替え")
                        contentItem: LineIcon {
                            name: window.dark ? "sun" : "moon"
                            ink: window.ink
                        }
                        onClicked: backend.patchSettings({
                            theme: window.dark ? "light" : "dark"
                        })
                    }
                    GlassButton {
                        text: window.uiLanguage === "zh" ? "中文" : window.uiLanguage === "ja" ? "日本語" : "EN"
                        dark: window.dark
                        Layout.fillWidth: true
                        onClicked: backend.patchSettings({
                            language: window.uiLanguage === "zh" ? "en" : window.uiLanguage === "en" ? "ja" : "zh"
                        })
                    }
                }
            }
        }
        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 16
            GlassPane {
                Layout.fillWidth: true
                Layout.preferredHeight: 68
                cornerRadius: 24
                backgroundSource: backdrop
                dark: window.dark
                reduceTransparency: window.reduceTransparency
                RowLayout {
                    anchors {
                        fill: parent
                        leftMargin: 24
                        rightMargin: 24
                    }
                    Text {
                        text: window.navigation.filter(n => n.key === window.page)[0]?.label || "KiroX"
                        color: window.ink
                        font {
                            pixelSize: 20
                            weight: Font.DemiBold
                        }
                        Layout.fillWidth: true
                    }
                    Text {
                        text: window.t("你的注册工作空间", "Your registration workspace", "登録ワークスペース")
                        color: window.muted
                        font.pixelSize: 12
                    }
                }
            }
            Loader {
                Layout.fillHeight: true
                Layout.fillWidth: true
                sourceComponent: window.page === "tasks" ? tasksView : window.page === "accounts" ? accountsView : window.page === "services" ? servicesView : window.page === "proxies" ? proxyView : window.page === "settings" ? settingsView : window.page === "about" ? aboutView : window.page === "logs" ? logsView : overviewView
            }
        }
    }
    Component {
        id: overviewView
        OverviewPage {
            shell: window
        }
    }
    Component {
        id: tasksView
        TasksPage {
            shell: window
        }
    }
    Component {
        id: accountsView
        AccountsPage {
            shell: window
        }
    }
    Component {
        id: servicesView
        ServicesPage {
            shell: window
        }
    }
    Component {
        id: proxyView
        ProxyPage {
            shell: window
        }
    }
    Component {
        id: settingsView
        SettingsPage {
            shell: window
        }
    }
    Component {
        id: aboutView
        AboutPage {
            shell: window
        }
    }
    Component {
        id: logsView
        ActivityPage {
            shell: window
        }
    }
    Popup {
        id: toast
        property string message
        property bool failure: false
        x: (window.width - width) / 2
        y: window.height - height - 32
        width: Math.min(580, label.implicitWidth + 48)
        height: label.implicitHeight + 28
        modal: false
        closePolicy: Popup.NoAutoClose
        background: Rectangle {
            radius: 18
            color: window.dark ? "#ed2d384d" : "#faffffff"
            border.color: toast.failure ? "#e58b97" : "#dce4ef"
        }
        contentItem: Text {
            id: label
            text: toast.message
            wrapMode: Text.Wrap
            color: toast.failure ? "#d15c70" : window.ink
            font.pixelSize: 13
        }
        Timer {
            id: toastTimer
            interval: 5000
            onTriggered: toast.close()
        }
    }
    Connections {
        target: backend
        function onOperationFailed(message) {
            toast.message = message;
            toast.failure = true;
            toast.open();
            toastTimer.restart();
        }
        function onOperationSucceeded(message) {
            toast.message = message;
            toast.failure = false;
            toast.open();
            toastTimer.restart();
        }
    }
}
