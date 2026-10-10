import QtQuick

Rectangle {
    id: root
    property bool dark: false
    radius: 24
    color: dark ? "#232833" : "#fafbff"
    border.color: dark ? "#343b48" : "#e5e9f1"
    border.width: 1
}
