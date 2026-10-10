import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ColumnLayout {
    id: root
    required property var shell
    spacing: 16
    RowLayout {
        Layout.fillWidth: true
        Text { text: root.shell.t("显示最近 500 条运行记录。", "The latest 500 activity entries.", "最新の 500 件のログを表示します。"); color: root.shell.muted; font.pixelSize: 13; Layout.fillWidth: true }
        GlassButton { text: root.shell.t("打开日志目录", "Log folder", "ログフォルダー"); dark: root.shell.dark; onClicked: backend.openDirectory("logs") }
        GlassButton { text: root.shell.t("清空显示", "Clear view", "表示をクリア"); dark: root.shell.dark; onClicked: backend.clearActivity() }
    }
    ContentCard {
        Layout.fillHeight: true; Layout.fillWidth: true; dark: root.shell.dark
        ListView {
            anchors { fill: parent; margins: 20 }
            clip: true; spacing: 6
            model: backend.activity
            ScrollBar.vertical: ScrollBar {}
            delegate: Rectangle {
                required property var modelData
                width: ListView.view.width; height: 48; radius: 12; color: root.shell.dark ? "#2b3446" : "#eef2f9"
                RowLayout {
                    anchors { fill: parent; margins: 12 }
                    Text { text: modelData.time?.substring(11,19) || ""; color: root.shell.muted; font { pixelSize: 11; family: "monospace" } Layout.preferredWidth: 64 }
                    Text { text: modelData.index ? "#" + modelData.index : ""; color: root.shell.muted; font.pixelSize: 11; Layout.preferredWidth: 38 }
                    Text { Layout.fillWidth: true; elide: Text.ElideRight; color: root.shell.ink; font.pixelSize: 12; text: modelData.event === "finished" ? root.shell.t("任务批次已结束", "Batch finished", "タスクが終了しました") : modelData.step ? (root.shell.uiLanguage && backend.stepLabel(modelData.step)) : root.shell.statusLabel(modelData.status || "") }
                    Text { text: modelData.errorCode || modelData.warning || ""; color: "#d76c79"; font.pixelSize: 11 }
                }
            }
            Text { anchors.centerIn: parent; visible: backend.activity.length === 0; text: root.shell.t("暂无运行日志", "No activity yet", "ログはありません"); color: root.shell.muted; font.pixelSize: 14 }
        }
    }
}
