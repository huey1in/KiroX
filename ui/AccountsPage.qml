import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ColumnLayout {
    id: root
    required property var shell
    property string provider: "outlook"
    property var rows: backend.mailboxes.filter(a => a.provider === provider)
    spacing: 16
    RowLayout {
        Layout.fillWidth: true
        GlassButton {
            text: "Outlook"
            primary: root.provider === "outlook"
            dark: root.shell.dark
            onClicked: root.provider = "outlook"
        }
        GlassButton {
            text: "iCloud"
            primary: root.provider === "icloud"
            dark: root.shell.dark
            onClicked: root.provider = "icloud"
        }
        Item {
            Layout.fillWidth: true
        }
        GlassButton {
            text: root.shell.t("导入文件", "Import file", "ファイルをインポート")
            dark: root.shell.dark
            onClicked: backend.importAccountsFile(root.provider)
        }
        GlassButton {
            text: root.shell.t("添加邮箱", "Add mailboxes", "メールを追加")
            primary: true
            dark: root.shell.dark
            onClicked: importDialog.open()
        }
    }
    ContentCard {
        Layout.fillWidth: true
        Layout.fillHeight: true
        dark: root.shell.dark
        ColumnLayout {
            anchors {
                fill: parent
                margins: 24
            }
            spacing: 14
            RowLayout {
                Text {
                    text: root.provider === "outlook" ? "Outlook" : "iCloud"
                    color: root.shell.ink
                    font {
                        pixelSize: 18
                        weight: Font.DemiBold
                    }
                    Layout.fillWidth: true
                }
                Text {
                    text: root.rows.length + root.shell.t(" 个邮箱", " mailboxes", " 件のメール")
                    color: root.shell.muted
                    font.pixelSize: 12
                }
            }
            Rectangle {
                height: 1
                Layout.fillWidth: true
                color: root.shell.dark ? "#364155" : "#e4e9f2"
            }
            ListView {
                Layout.fillHeight: true
                Layout.fillWidth: true
                clip: true
                spacing: 5
                model: root.rows
                ScrollBar.vertical: ScrollBar {}
                delegate: Rectangle {
                    required property var modelData
                    required property int index
                    width: ListView.view.width
                    height: 64
                    radius: 14
                    color: index % 2 === 0 ? (root.shell.dark ? "#292f3d" : "#f0f3fa") : "transparent"
                    RowLayout {
                        anchors {
                            fill: parent
                            leftMargin: 16
                            rightMargin: 12
                        }
                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 4
                            Text {
                                text: modelData.email
                                color: root.shell.ink
                                font.pixelSize: 13
                                elide: Text.ElideMiddle
                                Layout.fillWidth: true
                            }
                            Text {
                                text: (modelData.mode || "iCloud") + " · " + (modelData.addedAt || "")
                                color: root.shell.muted
                                font.pixelSize: 11
                            }
                        }
                        Text {
                            text: modelData.registered ? (modelData.success ? root.shell.t("成功", "Success", "成功") : root.shell.t("已处理", "Processed", "処理済み")) : root.shell.t("可用", "Available", "利用可能")
                            color: modelData.registered ? "#7298a3" : "#5aaf98"
                            font.pixelSize: 12
                        }
                        GlassButton {
                            text: root.shell.t("删除", "Remove", "削除")
                            danger: true
                            dark: root.shell.dark
                            implicitWidth: 74
                            implicitHeight: 34
                            onClicked: backend.deleteAccount(root.provider, modelData.email)
                        }
                    }
                }
                Text {
                    anchors.centerIn: parent
                    visible: root.rows.length === 0
                    text: root.shell.t("还没有邮箱，导入或粘贴账号开始使用。", "Import or paste accounts to build your mailbox pool.", "アカウントをインポート、または貼り付けてください。")
                    color: root.shell.muted
                    font.pixelSize: 13
                }
            }
            RowLayout {
                GlassButton {
                    text: root.shell.t("清除已注册", "Remove registered", "登録済みを削除")
                    dark: root.shell.dark
                    onClicked: {
                        confirmation.registeredOnly = true;
                        confirmation.open();
                    }
                }
                Item {
                    Layout.fillWidth: true
                }
                GlassButton {
                    text: root.shell.t("清空邮箱池", "Clear pool", "プールを空にする")
                    danger: true
                    dark: root.shell.dark
                    onClicked: {
                        confirmation.registeredOnly = false;
                        confirmation.open();
                    }
                }
            }
        }
    }
    GlassDialog {
        id: importDialog
        parent: Overlay.overlay
        anchors.centerIn: parent
        width: Math.min(600, parent.width - 60)
        modal: true
        title: root.shell.t("添加邮箱", "Add mailboxes", "メールを追加")

        onAccepted: {
            backend.importAccounts(root.provider, input.text);
            input.clear();
        }
        contentItem: ColumnLayout {
            spacing: 12
            Label {
                text: root.provider === "outlook" ? "email----password----clientId----refreshToken----imap/graph" : "email----https://host/messages/..."
                wrapMode: Text.Wrap
                Layout.fillWidth: true
                font.pixelSize: 12
            }
            ScrollView {
                Layout.fillWidth: true
                Layout.preferredHeight: 200
                TextArea {
                    id: input
                    placeholderText: root.shell.t("每行一个账号", "One account per line", "1 行に 1 アカウント")
                    wrapMode: TextEdit.Wrap
                    selectByMouse: true
                }
            }
        }
    }
    GlassDialog {
        id: confirmation
        property bool registeredOnly: false
        parent: Overlay.overlay
        anchors.centerIn: parent
        modal: true
        title: root.shell.t("确认删除", "Confirm removal", "削除の確認")

        Label {
            text: root.shell.t("此操作会删除所选邮箱池中的账号。", "This removes accounts from the selected mailbox pool.", "選択したプールのアカウントを削除します。")
        }
        onAccepted: backend.clearAccounts(root.provider, registeredOnly)
    }
}
