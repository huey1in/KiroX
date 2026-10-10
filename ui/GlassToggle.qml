import QtQuick
import QtQuick.Controls

Switch {
    id: control
    property bool dark: false
    property bool reduceMotion: !!backend.settings.reduceMotion
    implicitWidth: 48
    implicitHeight: 32
    padding: 0
    activeFocusOnTab: true
    indicator: Rectangle {
        width: 44; height: 26; radius: 13
        x: (control.width - width) / 2; y: (control.height - height) / 2
        color: control.checked ? "#557bf3" : control.dark ? "#4b566b" : "#d3dce9"
        border.color: control.activeFocus ? "#8dacff" : "transparent"
        border.width: control.activeFocus ? 2 : 0
        Rectangle {
            width: 22; height: 22; radius: 11
            x: control.checked ? parent.width - width - 2 : 2; y: 2
            color: "#ffffff"
            Behavior on x { NumberAnimation { duration: control.reduceMotion ? 0 : 160; easing.type: Easing.OutCubic } }
        }
    }
    contentItem: Item {}
}
