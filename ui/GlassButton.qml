import QtQuick
import QtQuick.Controls

Button {
    id: root
    property bool dark: false
    property bool primary: false
    property bool danger: false
    property bool reduceMotion: !!backend.settings.reduceMotion
    implicitHeight: 42
    implicitWidth: Math.max(88, contentItem.implicitWidth + 32)
    horizontalPadding: 16
    activeFocusOnTab: true
    opacity: enabled ? 1 : 0.4
    Accessible.role: Accessible.Button
    Accessible.name: text
    contentItem: Text {
        text: root.text
        color: root.primary ? "white" : root.danger ? "#d44758" : root.dark ? "#eef3ff" : "#263350"
        font { pixelSize: 13; weight: Font.DemiBold }
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }
    background: Rectangle {
        radius: height / 2
        color: root.primary ? (root.down ? "#375ee0" : "#557bf3") : root.down ? (root.dark ? "#555c6f86" : "#bbcdd8ee") : root.hovered ? (root.dark ? "#664a5d7a" : "#eeffffff") : (root.dark ? "#334c5f7f" : "#aaffffff")
        border.color: root.activeFocus ? "#7799ff" : root.primary ? "#668aff" : root.dark ? "#446e7e9c" : "#eeffffff"
        border.width: root.activeFocus ? 2 : 1
        Behavior on color { ColorAnimation { duration: root.reduceMotion ? 0 : 120 } }
    }
    scale: down ? 0.97 : 1
    Behavior on scale { NumberAnimation { duration: root.reduceMotion ? 0 : 120 } }
}
