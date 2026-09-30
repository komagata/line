import QtQuick
QtObject {
    objectName: 'bridgeProcess'
    property var command: []
    property bool stdinEnabled: false
    property bool running: false
    property var stdout: null
    property var stderr: null
    property var writes: []
    signal started()
    signal exited(int exitCode, int exitStatus)
    function write(data) { writes.push(data) }
}
