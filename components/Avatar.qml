import QtQuick
import 'Locale.js' as Locale

Rectangle {
    id: root
    property string language: 'ja'
    function tr(value) { return Locale.text(language,value) }
    property string name: ''
    property bool group: false
    property bool self: false
    property string imageData: ''
    // Only bounded raster data from the adapter may reach Qt's image decoder.
    // No remote URL or local filename can become an Image.source.
    readonly property string safeSource: !group && imageData.length <= 87407 &&
        /^data:image\/(png|jpeg);base64,[A-Za-z0-9+/]+={0,2}$/.test(imageData) &&
        (imageData.indexOf('data:image/png;base64,iVBORw0KGgo') === 0 || imageData.indexOf('data:image/jpeg;base64,/9j/') === 0) ? imageData : ''
    readonly property bool imageReady: safeSource !== '' && photo.status === Image.Ready && crop.loaded
    width: 50; height: width; radius: width/2
    color: self ? '#b8bfba' : group ? '#b5bfbe' : '#9eacad'
    Image {
        id: photo; objectName:'avatarImage'
        visible: false
        source: root.safeSource
        sourceSize: Qt.size(512,512)
        asynchronous: true; cache: false
        onStatusChanged: crop.requestPaint()
    }
    // drawImage(Image) looks up the source in Canvas's separate pixmap cache.
    // Image.Ready does not mean that cache is ready. Own the bounded load and
    // repaint on its completion, including when the source changes at one ID.
    // Canvas clipping works with both the software QtTest backend and the shell.
    Canvas {
        id: crop
        property string requestedSource: root.safeSource
        property string loadedSource: ''
        property bool loaded: false
        function loadSource() {
            loaded = false
            if (loadedSource) unloadImage(loadedSource)
            loadedSource = requestedSource
            if (loadedSource) {
                loadImage(loadedSource, Qt.size(512,512))
                loaded = isImageLoaded(loadedSource)
            }
            requestPaint()
        }
        onRequestedSourceChanged: loadSource()
        Component.onCompleted: loadSource()
        onImageLoaded: {
            loaded = loadedSource !== '' && isImageLoaded(loadedSource)
            requestPaint()
        }
        onAvailableChanged: if (available) requestPaint()
        anchors.fill: parent
        visible: root.imageReady
        onVisibleChanged: requestPaint()
        onWidthChanged: requestPaint()
        onHeightChanged: requestPaint()
        onPaint: {
            var ctx=getContext('2d')
            ctx.reset()
            if (!root.imageReady) return
            ctx.beginPath();ctx.arc(width/2,height/2,width/2,0,Math.PI*2);ctx.clip()
            var scale=Math.max(width/photo.implicitWidth,height/photo.implicitHeight)
            var w=photo.implicitWidth*scale, h=photo.implicitHeight*scale
            ctx.drawImage(root.safeSource,(width-w)/2,(height-h)/2,w,h)
        }
    }
    Text {
        objectName:'avatarInitial'
        anchors.centerIn: parent
        visible: !root.imageReady
        text: root.group ? '󰡉' : root.self ? root.tr('自') : root.name.trim().slice(0,1)
        textFormat: Text.PlainText
        color: '#35413e'
        font.family: root.group ? 'Symbols Nerd Font' : 'sans-serif'
        font.pixelSize: root.width * 0.42; font.weight: Font.Medium
    }
}
