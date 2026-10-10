import QtQuick
import QtQuick.Controls

ComboBox {
    id: control
    property bool dark: false
    implicitHeight: 38
    implicitWidth: 160
    leftPadding: 14
    rightPadding: 32
    activeFocusOnTab: true
    palette.text: dark ? "#eef3ff" : "#24334e"
    contentItem: Text {
        text: control.displayText
        color: control.palette.text
        font {
            pixelSize: 13
        }
        elide: Text.ElideRight
        verticalAlignment: Text.AlignVCenter
    }
    indicator: Canvas {
        x: control.width - width - 12
        y: (control.height - height) / 2
        width: 10
        height: 6
        property color ink: control.palette.text
        onInkChanged: requestPaint()
        onPaint: {
            const c = getContext("2d");
            c.reset();
            c.strokeStyle = ink;
            c.lineWidth = 1.5;
            c.beginPath();
            c.moveTo(1, 1);
            c.lineTo(5, 5);
            c.lineTo(9, 1);
            c.stroke();
        }
    }
    background: Rectangle {
        radius: 12
        color: control.dark ? "#354156" : "#f0f4fb"
        border.color: control.activeFocus ? "#7397fb" : control.dark ? "#526078" : "#dce4f2"
        border.width: control.activeFocus ? 2 : 1
    }
    delegate: ItemDelegate {
        required property var modelData
        required property int index
        width: control.width
        text: modelData
        highlighted: control.highlightedIndex === index
        contentItem: Text {
            text: parent.text
            color: control.palette.text
            font {
                pixelSize: 13
            }
            verticalAlignment: Text.AlignVCenter
        }
        background: Rectangle {
            radius: 9
            color: parent.highlighted ? (control.dark ? "#485c85" : "#e2eafb") : "transparent"
        }
    }
    popup: Popup {
        y: control.height + 6
        width: control.width
        padding: 6
        implicitHeight: Math.min(contentItem.implicitHeight + 12, 280)
        contentItem: ListView {
            clip: true
            implicitHeight: contentHeight
            model: control.popup.visible ? control.delegateModel : null
            currentIndex: control.highlightedIndex
            ScrollIndicator.vertical: ScrollIndicator {}
        }
        background: Rectangle {
            radius: 15
            color: control.dark ? "#2c374b" : "#fcfdff"
            border.color: control.dark ? "#526078" : "#dce4f2"
        }
    }
}
