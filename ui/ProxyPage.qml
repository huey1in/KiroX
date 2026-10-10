import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ColumnLayout {
    id: root
    required property var shell
    spacing: 16
    RowLayout {
        Layout.fillWidth: true
        Text { text: root.shell.t("为不同账号配置独立网络。", "An independent connection for every account.", "アカウントごとに独立した接続を。"); color: root.shell.muted; font.pixelSize: 13; Layout.fillWidth: true }
        GlassButton { text: root.shell.t("批量导入", "Bulk import", "一括インポート"); dark: root.shell.dark; onClicked: bulkDialog.open() }
        GlassButton { text: root.shell.t("添加代理", "Add proxy", "プロキシを追加"); primary: true; dark: root.shell.dark; onClicked: { addDialog.editId = ""; nameInput.text = ""; urlInput.text = ""; weightInput.value = 50; addDialog.open() } }
    }
    ContentCard {
        Layout.fillHeight: true
        Layout.fillWidth: true
        dark: root.shell.dark
        ListView {
            anchors { fill: parent; margins: 20 }
            model: backend.proxies
            spacing: 12
            clip: true
            ScrollBar.vertical: ScrollBar {}
            delegate: Rectangle {
                required property var modelData
                width: ListView.view.width
                height: 96
                radius: 18
                color: root.shell.dark ? "#2b3241" : "#eff3fa"
                RowLayout {
                    anchors { fill: parent; margins: 16 }
                    GlassToggle { dark: root.shell.dark; checked: modelData.enabled; Accessible.name: modelData.name; onToggled: backend.enableProxy(modelData.id, checked) }
                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 5
                        Text { text: modelData.name; color: root.shell.ink; font { pixelSize: 14; weight: Font.DemiBold } elide: Text.ElideRight; Layout.fillWidth: true }
                        Text { text: modelData.url; color: root.shell.muted; font.pixelSize: 12; elide: Text.ElideMiddle; Layout.fillWidth: true }
                        Text { text: modelData.probeIp ? modelData.probeIp + " · " + (modelData.probeCountry || "") + " · " + (modelData.probeMs || 0) + "ms" : modelData.probeError || root.shell.t("尚未检测", "Not tested", "未テスト"); color: modelData.probeOk ? "#63b69e" : root.shell.muted; font.pixelSize: 11; Layout.fillWidth: true; elide: Text.ElideRight }
                    }
                    GlassButton { text: root.shell.t("检测", "Test", "テスト"); dark: root.shell.dark; enabled: !backend.busy; implicitWidth: 74; onClicked: backend.testProxy(modelData.id) }
                    GlassButton { text: root.shell.t("编辑", "Edit", "編集"); dark: root.shell.dark; implicitWidth: 66; onClicked: {
                        const config = backend.proxyConfiguration(modelData.id)
                        addDialog.editId = modelData.id
                        nameInput.text = config.name
                        urlInput.text = config.url
                        weightInput.value = config.weight
                        addDialog.open()
                    } }
                    GlassButton { text: root.shell.t("删除", "Remove", "削除"); dark: root.shell.dark; danger: true; implicitWidth: 74; onClicked: backend.deleteProxy(modelData.id) }
                }
            }
            Text { anchors.centerIn: parent; visible: backend.proxies.length === 0; text: root.shell.t("添加一个代理，或每行一个 URL 批量导入。", "Add a proxy, or import one URL per line.", "プロキシを追加、または URL を 1 行ずつインポートしてください。"); color: root.shell.muted; font.pixelSize: 13 }
        }
    }
    GlassDialog {
        id: addDialog
        property string editId: ""
        parent: Overlay.overlay
        anchors.centerIn: parent
        width: Math.min(520, root.shell.width - 40)
        modal: true
        title: editId ? root.shell.t("编辑代理", "Edit proxy", "プロキシを編集") : root.shell.t("添加代理", "Add proxy", "プロキシを追加")

        onAccepted: {
            if (editId) {
                if (!backend.saveProxy(editId, nameInput.text, urlInput.text, weightInput.value))
                    Qt.callLater(addDialog.open)
            } else backend.addProxy(nameInput.text, urlInput.text, weightInput.value)
        }
        onRejected: urlInput.text = ""
        contentItem: ColumnLayout {
            GlassTextField { dark: root.shell.dark; id: nameInput; Layout.fillWidth: true; placeholderText: root.shell.t("名称（可选）", "Name (optional)", "名前（任意）") }
            GlassTextField { dark: root.shell.dark; id: urlInput; Layout.fillWidth: true; placeholderText: "http://user:password@host:port"; Accessible.name: root.shell.t("代理地址", "Proxy URL", "プロキシ URL") }
            RowLayout { Label { text: root.shell.t("权重", "Weight", "重み") } SpinBox { id: weightInput; from: 1; to: 100; value: 50; editable: true } }
        }
    }
    GlassDialog {
        id: bulkDialog
        parent: Overlay.overlay
        anchors.centerIn: parent
        width: Math.min(580, root.shell.width - 40)
        modal: true
        title: root.shell.t("批量导入代理", "Import proxies", "プロキシをインポート")

        onAccepted: { backend.importProxies(bulkInput.text); bulkInput.clear() }
        contentItem: ScrollView { implicitHeight: 220; TextArea { id: bulkInput; placeholderText: "http://user:password@host:port\nsocks5://host:1080"; wrapMode: TextEdit.Wrap; selectByMouse: true } }
    }
}
