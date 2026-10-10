import QtQuick
import QtQuick.Effects

Item {
    id: root
    property Item backgroundSource
    property bool dark: false
    property bool reduceTransparency: false
    property real cornerRadius: 28
    default property alias content: body.data
    ShaderEffectSource {
        id: capture
        anchors.fill: parent
        sourceItem: root.backgroundSource
        sourceRect: {
            if (!root.backgroundSource) return Qt.rect(0, 0, width, height)
            const point = root.mapToItem(root.backgroundSource, 0, 0)
            return Qt.rect(point.x, point.y, width, height)
        }
        live: true
        visible: false
    }
    Rectangle {
        id: mask
        anchors.fill: parent
        radius: root.cornerRadius
        color: "white"
        visible: false
        layer.enabled: true
    }
    MultiEffect {
        anchors.fill: parent
        source: capture
        visible: !!root.backgroundSource && !root.reduceTransparency
        blurEnabled: true
        blurMax: 64
        blur: 0.8
        saturation: 0.18
        maskEnabled: true
        maskSource: mask
        autoPaddingEnabled: false
    }
    Rectangle {
        anchors.fill: parent
        radius: root.cornerRadius
        color: root.reduceTransparency ? (root.dark ? "#272f40" : "#eff3fc") : (root.dark ? "#aa253045" : "#bbffffff")
        border.width: 1
        border.color: root.dark ? "#556f809c" : "#eeffffff"
        Rectangle {
            anchors { top: parent.top; left: parent.left; right: parent.right; margins: 1 }
            height: Math.min(72, parent.height / 2)
            radius: parent.radius
            gradient: Gradient {
                GradientStop { position: 0; color: root.dark ? "#1affffff" : "#44ffffff" }
                GradientStop { position: 1; color: "transparent" }
            }
        }
    }
    Item { id: body; anchors.fill: parent }
}
