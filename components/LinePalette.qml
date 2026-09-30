import QtQuick
import qs.Commons

QtObject {
    readonly property color surface: Qt.tint(Color.background, '#dc252928')
    readonly property color foreground: Qt.tint(Color.foreground, '#def0f1ec')
    readonly property color muted: '#a6aeac'
    readonly property color line: '#454c49'
    readonly property color hover: '#343b37'
    readonly property color selected: '#424f44'
    readonly property color incoming: '#3b3f3d'
    readonly property color outgoing: '#566e59'
    readonly property color accent: '#83b487'
    readonly property color error: '#e5b5a3'
    readonly property string family: Style.font.family
    readonly property int body: Math.max(15, Style.font.body + 3)
    readonly property int radius: 0
}
