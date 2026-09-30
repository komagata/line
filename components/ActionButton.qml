import QtQuick
import QtQuick.Controls

AbstractButton {
    id: root
    property bool accent: false
    property bool glyphOnly: false
    property string hint: ''
    LinePalette { id: ink }
    implicitWidth: Math.max(38, label.implicitWidth + 26)
    implicitHeight: 38
    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    background: Rectangle {
        radius: ink.radius
        color: root.accent ? ink.outgoing : root.hovered || root.activeFocus ? ink.hover : 'transparent'
        border.color: root.activeFocus ? ink.accent : 'transparent'
        opacity: root.enabled ? 1 : 0.45
    }
    contentItem: Text {
        id: label
        text: root.text
        textFormat: Text.PlainText
        elide: Text.ElideRight
        color: ink.foreground
        font.family: root.glyphOnly ? 'Symbols Nerd Font' : ink.family; font.pixelSize: root.glyphOnly ? 23 : ink.body
        opacity: root.enabled ? 1 : 0.35
        horizontalAlignment: Text.AlignHCenter; verticalAlignment: Text.AlignVCenter
    }
    ToolTip.visible: hovered && hint !== ''
    ToolTip.delay: 600
    ToolTip.text: hint // authored static strings only
    Accessible.name: hint || text
}
