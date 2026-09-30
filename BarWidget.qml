import QtQuick
import qs.Ui as Ui

Ui.BarWidget {
    id:root
    moduleName:'io.github.komagata.line'
    readonly property var shell:bar?bar.shell:null
    property var service:null
    implicitWidth:button.implicitWidth
    implicitHeight:button.implicitHeight
    Timer {interval:1000;running:true;repeat:true;triggeredOnStart:true;onTriggered:root.service=root.shell?root.shell.serviceFor(root.moduleName):null}
    Ui.WidgetButton {
        id:button
        anchors.fill:parent;bar:root.bar
        text:'󰍩'+(root.service && root.service.unread>0?' '+Math.min(root.service.unread,99):'')
        tooltipText:root.service?('LINE · '+(root.service.view.mode==='demo'?'デモ':root.service.view.statusText)).replace(/[<>&\u0000-\u001f\u007f-\u009f\u202a-\u202e\u2066-\u2069]/g,'').slice(0,180):'LINE · 準備中'
        onPressed:mouseButton=>{if(root.shell) root.shell.toggle(root.moduleName,mouseButton===Qt.RightButton?'{"settings":true}':'{}')}
    }
}
