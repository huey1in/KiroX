import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ScrollView {
    id: root
    required property var shell
    clip: true
    contentWidth: availableWidth
    ColumnLayout {
        width: root.availableWidth
        spacing: 18
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: 192
            dark: root.shell.dark
            ColumnLayout {
                anchors {
                    fill: parent
                    margins: 28
                }
                spacing: 10
                Text {
                    text: "KIROX"
                    color: "#6686ef"
                    font {
                        pixelSize: 11
                        letterSpacing: 3
                        weight: Font.Bold
                    }
                }
                Text {
                    text: root.shell.t("让注册工作，井然有序。", "A clear space for your workflow.", "登録作業を、もっと整然と。")
                    color: root.shell.ink
                    font {
                        pixelSize: 27
                        weight: Font.DemiBold
                    }
                    Layout.fillWidth: true
                    wrapMode: Text.Wrap
                }
                Text {
                    text: root.shell.t("管理邮箱、代理与任务，所有数据保存在本机。", "Mailboxes, proxies and tasks. Your data stays on this device.", "メール、プロキシ、タスク。データはこの端末に保存されます。")
                    color: root.shell.muted
                    font.pixelSize: 13
                    Layout.fillWidth: true
                    wrapMode: Text.Wrap
                }
                Item {
                    Layout.fillHeight: true
                }
                GlassButton {
                    text: root.shell.t("新建任务", "Create a batch", "タスクを作成")
                    primary: true
                    dark: root.shell.dark
                    reduceMotion: root.shell.reduceMotion
                    onClicked: root.shell.page = "tasks"
                }
            }
        }
        RowLayout {
            Layout.fillWidth: true
            spacing: 16
            Repeater {
                model: [
                    {
                        label: root.shell.t("可用邮箱", "Available mailboxes", "利用可能なメール"),
                        value: backend.mailboxes.filter(a => !a.registered).length,
                        detail: root.shell.t("未注册的账号", "Ready for registration", "未登録のアカウント")
                    },
                    {
                        label: root.shell.t("已注册", "Registered", "登録済み"),
                        value: backend.mailboxes.filter(a => a.registered).length,
                        detail: root.shell.t("已处理的邮箱", "Processed mailboxes", "処理済みのメール")
                    },
                    {
                        label: root.shell.t("已启用代理", "Active proxies", "有効なプロキシ"),
                        value: backend.proxies.filter(p => p.enabled).length,
                        detail: root.shell.t("独立 IP 配置", "Independent IP configurations", "独立した IP 設定")
                    }
                ]
                delegate: ContentCard {
                    required property var modelData
                    Layout.fillWidth: true
                    implicitHeight: 146
                    dark: root.shell.dark
                    ColumnLayout {
                        anchors {
                            fill: parent
                            margins: 24
                        }
                        Text {
                            text: modelData.label
                            color: root.shell.muted
                            font.pixelSize: 12
                        }
                        Text {
                            text: modelData.value
                            color: root.shell.ink
                            font {
                                pixelSize: 36
                                weight: Font.DemiBold
                            }
                        }
                        Text {
                            text: modelData.detail
                            color: root.shell.muted
                            font.pixelSize: 11
                        }
                    }
                }
            }
        }
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: 180
            dark: root.shell.dark
            ColumnLayout {
                anchors {
                    fill: parent
                    margins: 26
                }
                Text {
                    text: root.shell.t("工作空间", "Your workspace", "ワークスペース")
                    color: root.shell.ink
                    font {
                        pixelSize: 16
                        weight: Font.DemiBold
                    }
                }
                Text {
                    text: root.shell.t("邮箱凭证和代理配置保存在数据目录，成功结果写入输出目录。", "Mailbox credentials and proxy configurations live in your data folder. Successful results are written to the output folder.", "メール認証情報とプロキシ設定はデータフォルダーに、成功結果は出力フォルダーに保存します。")
                    color: root.shell.muted
                    font.pixelSize: 13
                    wrapMode: Text.Wrap
                    Layout.fillWidth: true
                }
                RowLayout {
                    GlassButton {
                        text: root.shell.t("打开数据目录", "Data folder", "データフォルダー")
                        dark: root.shell.dark
                        onClicked: backend.openDirectory("data")
                    }
                    GlassButton {
                        text: root.shell.t("打开结果目录", "Results folder", "結果フォルダー")
                        dark: root.shell.dark
                        onClicked: backend.openDirectory("results")
                    }
                }
            }
        }
    }
}
