import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ColumnLayout {
    id: root
    required property var shell
    property string provider: "moemail"
    property var rows: backend.providerConfigurations.filter(c => c.provider === provider)
    spacing: 16
    RowLayout {
        Repeater {
            model: [{key:"moemail", label:"MoeMail"}, {key:"cloudmail", label:"Cloud-Mail"}, {key:"mailnest", label:"MailNest"}]
            delegate: GlassButton {
                required property var modelData
                text: modelData.label; dark: root.shell.dark; primary: root.provider === modelData.key
                onClicked: root.provider = modelData.key
            }
        }
        Item { Layout.fillWidth: true }
        GlassButton { text: root.shell.t("添加服务", "Add service", "サービスを追加"); primary: true; dark: root.shell.dark; onClicked: editor.edit("") }
    }
    ContentCard {
        Layout.fillWidth: true; Layout.fillHeight: true; dark: root.shell.dark
        ColumnLayout {
            anchors { fill: parent; margins: 24 }
            spacing: 16
            Text { text: root.shell.t("邮箱服务", "Mailbox services", "メールサービス"); color: root.shell.ink; font { pixelSize: 18; weight: Font.DemiBold } }
            Text { text: root.shell.t("管理连接配置，检查可用域名或账户余额。", "Manage connections and check available domains or account balance.", "接続設定、利用可能なドメインと残高を管理します。"); color: root.shell.muted; wrapMode: Text.Wrap; Layout.fillWidth: true; font.pixelSize: 13 }
            ListView {
                Layout.fillWidth: true; Layout.fillHeight: true
                model: root.rows; spacing: 8; clip: true
                ScrollBar.vertical: ScrollBar {}
                delegate: Rectangle {
                    required property var modelData
                    width: ListView.view.width; height: 88; radius: 16
                    color: root.shell.dark ? "#30394c" : "#f0f3fa"
                    RowLayout {
                        anchors { fill: parent; margins: 16 }
                        ColumnLayout {
                            Layout.fillWidth: true; spacing: 5
                            Text { text: modelData.name || "MailNest"; color: root.shell.ink; font { pixelSize: 14; weight: Font.DemiBold } }
                            Text { text: modelData.url || modelData.projectCode || ""; color: root.shell.muted; font.pixelSize: 12; elide: Text.ElideMiddle; Layout.fillWidth: true }
                        }
                        GlassButton { text: root.shell.t("检查连接", "Check", "接続確認"); dark: root.shell.dark; enabled: !backend.busy; onClicked: backend.inspectProvider(root.provider, modelData.name || "") }
                        GlassButton { text: root.shell.t("编辑", "Edit", "編集"); dark: root.shell.dark; onClicked: editor.edit(modelData.name || "") }
                        GlassButton { text: root.shell.t("删除", "Remove", "削除"); danger: true; dark: root.shell.dark; onClicked: { removal.name = modelData.name || ""; removal.open() } }
                    }
                }
                Text { anchors.centerIn: parent; visible: root.rows.length === 0; text: root.shell.t("添加邮箱服务以开始使用。", "Add a mailbox service to get started.", "メールサービスを追加してください。"); color: root.shell.muted; font.pixelSize: 13 }
            }
            RowLayout {
                Text { id: inspection; Layout.fillWidth: true; color: root.shell.muted; font.pixelSize: 13; wrapMode: Text.Wrap }
                BusyIndicator { running: backend.busy; visible: running; Layout.preferredWidth: 32; Layout.preferredHeight: 32 }
                GlassButton { visible: backend.busy; text: root.shell.t("取消", "Cancel", "キャンセル"); dark: root.shell.dark; onClicked: backend.cancelOperation() }
            }
        }
    }
    GlassDialog {
        id: editor
        property string oldName: ""
        property string kind: ""
        parent: Overlay.overlay; anchors.centerIn: parent; modal: true
        width: Math.min(540, parent.width - 60)
        title: root.shell.t("服务配置", "Service configuration", "サービス設定")
        function edit(name) {
            kind = root.provider; oldName = name
            const value = name || kind === "mailnest" ? backend.providerConfiguration(kind, name) : {}
            serviceName.text = value.name || ""; baseUrl.text = value.url || ""
            apiKey.text = value.apiKey || ""; adminEmail.text = value.email || ""
            password.text = value.password || ""; domains.text = (value.domains || []).join(", ")
            project.text = value.projectCode || ""; open()
        }
        contentItem: ColumnLayout {
            spacing: 12
            GlassTextField { id: serviceName; visible: editor.kind !== "mailnest"; Layout.fillWidth: true; dark: root.shell.dark; placeholderText: root.shell.t("配置名称", "Configuration name", "設定名") }
            GlassTextField { id: baseUrl; visible: editor.kind !== "mailnest"; Layout.fillWidth: true; dark: root.shell.dark; placeholderText: "https://mail.example.com" }
            GlassTextField { id: apiKey; visible: editor.kind !== "cloudmail"; Layout.fillWidth: true; dark: root.shell.dark; placeholderText: "API Key"; echoMode: TextInput.Password }
            GlassTextField { id: adminEmail; visible: editor.kind === "cloudmail"; Layout.fillWidth: true; dark: root.shell.dark; placeholderText: root.shell.t("管理员邮箱", "Administrator email", "管理者メール") }
            GlassTextField { id: password; visible: editor.kind === "cloudmail"; Layout.fillWidth: true; dark: root.shell.dark; placeholderText: root.shell.t("管理员密码", "Administrator password", "管理者パスワード"); echoMode: TextInput.Password }
            GlassTextField { id: domains; visible: editor.kind === "cloudmail"; Layout.fillWidth: true; dark: root.shell.dark; placeholderText: root.shell.t("域名，逗号分隔（可选）", "Domains, separated by commas (optional)", "ドメイン、コンマ区切り（任意）") }
            GlassTextField { id: project; visible: editor.kind === "mailnest"; Layout.fillWidth: true; dark: root.shell.dark; placeholderText: "Project Code" }
        }
        footer: RowLayout {
            spacing: 12
            Item { Layout.fillWidth: true }
            GlassButton { text: root.shell.t("取消", "Cancel", "キャンセル"); dark: root.shell.dark; onClicked: editor.close() }
            GlassButton {
                text: root.shell.t("保存", "Save", "保存"); primary: true; dark: root.shell.dark
                onClicked: {
                    let value = editor.kind === "mailnest" ? {apiKey: apiKey.text, projectCode: project.text} : editor.kind === "moemail" ? {name: serviceName.text, url: baseUrl.text, apiKey: apiKey.text} : {name: serviceName.text, url: baseUrl.text, email: adminEmail.text, password: password.text, domains: domains.text.split(",").map(d => d.trim()).filter(d => d.length > 0)}
                    if (backend.saveProviderConfiguration(editor.kind, value, editor.oldName)) editor.close()
                }
            }
        }
    }
    GlassDialog {
        id: removal
        property string name: ""
        parent: Overlay.overlay; anchors.centerIn: parent; modal: true
        title: root.shell.t("删除配置", "Remove configuration", "設定を削除")

        Label { text: root.shell.t("确认删除此服务配置？", "Remove this service configuration?", "このサービス設定を削除しますか？") }
        onAccepted: backend.removeProviderConfiguration(root.provider, name)
    }
    Connections {
        target: backend
        function onProviderInspected(provider, result) {
            inspection.text = result.balance !== undefined ? root.shell.t("账户余额：", "Balance: ", "残高：") + result.balance : root.shell.t("可用域名：", "Available domains: ", "利用可能なドメイン：") + (result.domains || []).join(", ")
        }
    }
}
