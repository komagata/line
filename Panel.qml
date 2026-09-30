import QtQuick
import QtQuick.Window
import Quickshell
import Quickshell.Hyprland
import 'components'

Item {
    id:root
    property var shell:null
    property var manifest:null
    property var service:null
    property bool opened:false
    property var targetScreen:null
    property bool demoRequested:false
    property bool settingsRequested:false
    function resolveService() {
        service=shell?shell.serviceFor('io.github.komagata.line'):null
        if(service && demoRequested){service.setMode('demo');demoRequested=false}
    }
    onShellChanged:resolveService()
    function open(payloadJson) {
        var payload=({})
        try { if(String(payloadJson||'').length<=4096) payload=JSON.parse(payloadJson||'{}')||({}) } catch(error) {}
        demoRequested=payload.demo===true;settingsRequested=payload.settings===true
        resolveService()
        var monitor=Hyprland.focusedMonitor
        targetScreen=Quickshell.screens.find(s=>monitor && s.name===monitor.name)||Quickshell.screens[0]
        opened=true;chat.settingsOpen=settingsRequested
        Qt.callLater(function(){chat.forceActiveFocus()})
    }
    function close(){chat.closeTransient();if(service) service.cancelLogin();opened=false}
    Timer {interval:200;repeat:true;running:root.opened&&!root.service;onTriggered:root.resolveService()}
    Component.onDestruction:{chat.closeTransient();if(service) {if(typeof service.feature==='function')service.feature('focus',{focused:false});service.cancelLogin()}}
    FloatingWindow {
        id:window
        screen:root.targetScreen
        visible:root.opened
        title:'LINE for Omarchy'
        implicitWidth:Math.min(1200,screen?screen.width-48:1200)
        implicitHeight:Math.min(760,screen?screen.height-64:760)
        minimumSize:Qt.size(720,540)
        color:'transparent'
        onVisibleChanged:if(!visible && root.opened) root.close()
        ChatView {id:chat;anchors.fill:parent;service:root.service;appVisible:root.opened && window.visible;windowFocused:root.opened && window.contentItem.Window.active;onDismiss:root.close()}
    }
}
