import QtQuick
import QtQuick.Controls

SpinBox {
    id: root
    property bool dark: backend.settings.theme === "dark" || (backend.settings.theme === "system" && Qt.styleHints.colorScheme === Qt.Dark)
    implicitWidth: 140
    implicitHeight: 42
    leftPadding: 40
    rightPadding: 40
    opacity: enabled ? 1 : 0.45
    activeFocusOnTab: true
    contentItem: TextInput {
        text: root.displayText
        font.pixelSize: 13
        color: root.dark ? "#eef3ff" : "#263350"
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        readOnly: !root.editable
        validator: root.validator
        inputMethodHints: root.inputMethodHints
        selectByMouse: true
        selectionColor: "#557bf3"
        selectedTextColor: "white"
        clip: true
    }
    up.indicator: Rectangle {
        x: root.width - width - 3
        y: 3
        width: 36
        height: root.height - 6
        radius: height / 2
        color: root.up.pressed ? (root.dark ? "#54719b" : "#d6e1fc") : "transparent"
        opacity: root.up.enabled ? 1 : 0.3
        Rectangle {
            anchors.centerIn: parent
            width: 12
            height: 2
            radius: 1
            color: root.dark ? "#eef3ff" : "#263350"
        }
        Rectangle {
            anchors.centerIn: parent
            width: 2
            height: 12
            radius: 1
            color: root.dark ? "#eef3ff" : "#263350"
        }
    }
    down.indicator: Rectangle {
        x: 3
        y: 3
        width: 36
        height: root.height - 6
        radius: height / 2
        color: root.down.pressed ? (root.dark ? "#54719b" : "#d6e1fc") : "transparent"
        opacity: root.down.enabled ? 1 : 0.3
        Rectangle {
            anchors.centerIn: parent
            width: 12
            height: 2
            radius: 1
            color: root.dark ? "#eef3ff" : "#263350"
        }
    }
    background: Rectangle {
        radius: height / 2
        color: root.dark ? "#39465d" : "#f0f4fc"
        border.color: root.activeFocus ? "#7799ff" : root.dark ? "#52627c" : "#d6e1f7"
        border.width: root.activeFocus ? 2 : 1
    }
}
