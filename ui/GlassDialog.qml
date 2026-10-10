import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Dialog {
    id: root
    property bool dark: backend.settings.theme === "dark" || (backend.settings.theme === "system" && Qt.styleHints.colorScheme === Qt.Dark)
    property string acceptText: backend.language === "zh" ? "确定" : backend.language === "ja" ? "確認" : "OK"
    property string cancelText: backend.language === "zh" ? "取消" : backend.language === "ja" ? "キャンセル" : "Cancel"
    padding: 24
    modal: true
    background: Rectangle {
        radius: 24
        color: root.dark ? "#f52b3548" : "#faf9fbff"
        border.color: root.dark ? "#617791ab" : "#ffffff"
    }
    header: Text {
        text: root.title
        color: root.dark ? "#f1f5ff" : "#24334e"
        font {
            pixelSize: 18
            weight: Font.DemiBold
        }
        padding: 24
        wrapMode: Text.Wrap
    }
    footer: RowLayout {
        spacing: 12
        Item {
            Layout.fillWidth: true
        }
        GlassButton {
            Layout.bottomMargin: 20
            text: root.cancelText
            dark: root.dark
            onClicked: root.reject()
        }
        GlassButton {
            Layout.rightMargin: 24
            Layout.bottomMargin: 20
            text: root.acceptText
            dark: root.dark
            primary: true
            onClicked: root.accept()
        }
    }
}
