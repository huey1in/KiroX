import QtQuick
import QtQuick.Controls

TextField {
    id: control
    property bool dark: false
    implicitHeight: 40
    leftPadding: 14
    rightPadding: 14
    color: dark ? "#eef3ff" : "#24334e"
    placeholderTextColor: dark ? "#a4b2c9" : "#79859b"
    selectionColor: "#557bf3"
    selectedTextColor: "white"
    font {
        pixelSize: 13
    }
    selectByMouse: true
    Accessible.name: placeholderText
    background: Rectangle {
        radius: 12
        color: control.dark ? "#354156" : "#f0f4fb"
        border.color: control.activeFocus ? "#7397fb" : control.dark ? "#526078" : "#dce4f2"
        border.width: control.activeFocus ? 2 : 1
    }
}
