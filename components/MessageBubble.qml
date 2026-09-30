import QtQuick
import 'MessageLinks.js' as MessageLinks

Item {
    id: root
    property var message: ({})
    property string imageData: ''
    property string stickerImage: ''
    property string replyText: ''
    signal actionsRequested()
    signal linkActivated(string url)
    readonly property string messageText: root.message.text || ''
    readonly property var messageLinks: MessageLinks.spans(messageText)
    function activateLink(url) {
        if (MessageLinks.valid(url) && messageLinks.some(link => link.url === url)) root.linkActivated(url)
    }
    LinePalette { id: ink }
    implicitHeight: bubble.height + 22
    Avatar {
        id: avatar; objectName:'messageAvatar-'+(root.message.id || '')
        visible: !root.message.own
        anchors.left: parent.left; anchors.top: bubble.top
        width: 32; name: root.message.sender || '送信者不明'; imageData:root.imageData
    }
    Rectangle {
        id: bubble
        anchors.left: root.message.own ? undefined : avatar.right
        anchors.leftMargin: 8
        anchors.right: root.message.own ? parent.right : undefined
        width: Math.min(root.width - (root.message.own ? 0 : 40), Math.max(root.message.sticker ? 224 : 150, body.implicitWidth + 30))
        height: content.height + 20
        radius: ink.radius; color: root.message.own ? ink.outgoing : ink.incoming
        Column {
            id:content; x:12;y:10;width:parent.width-24;spacing:6
            Text { width:parent.width;visible:text!=='';text:root.replyText;textFormat:Text.PlainText;wrapMode:Text.Wrap;maximumLineCount:2;elide:Text.ElideRight;color:ink.muted;font.family:ink.family;font.pixelSize:12 }
            StickerImage {
                id:stickerPoster;objectName:'receivedSticker-'+(root.message.id||'');width:Math.min(root.width<600?144:200,parent.width);height:visible?width:0;visible:!!root.message.sticker;imageData:root.stickerImage
                anchors.horizontalCenter:parent.horizontalCenter
            }
            Text {objectName:'stickerFallback-'+(root.message.id||'');width:parent.width;visible:!!root.message.sticker && stickerPoster.status!==Image.Ready;text:'スタンプ画像を表示できません（読み込み中または未対応）';textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.pixelSize:12}
            TextEdit {
                id:body;objectName:'messageText-'+(root.message.id||'')
                width:parent.width;readOnly:true;selectByMouse:true;activeFocusOnTab:true
                // Raw message text never reaches RichText; see MessageLinks.markup.
                text:MessageLinks.markup(root.messageText,root.messageLinks,String(ink.accent))
                textFormat:TextEdit.RichText
                wrapMode:TextEdit.Wrap
                font.family:ink.family;font.pixelSize:ink.body+1
                color:root.message.status==='decryption_failed'?ink.error:ink.foreground
                selectionColor:ink.selected
                selectByKeyboard:true
                onLinkActivated: link => root.activateLink(link)
                Keys.onPressed: event => {
                    if ((event.key===Qt.Key_Return || event.key===Qt.Key_Enter) && (event.modifiers & Qt.ControlModifier)) {
                        var link=root.messageLinks.find(span => body.cursorPosition>=span.start && body.cursorPosition<span.end)
                        if(link)root.activateLink(link.url)
                        event.accepted=true
                    }
                }
                HoverHandler { cursorShape:body.hoveredLink ? Qt.PointingHandCursor : Qt.IBeamCursor }
                Accessible.description:'リンク上でクリック、またはカーソルを合わせて Ctrl+Enter でブラウザを開く'
            }
            Text {
                width:parent.width;visible:text!==''
                text:(root.message.reactions||[]).map(r=>({'like':'👍','love':'♥','laugh':'😆','surprise':'😮','sad':'😢','angry':'😡'}[r.name]||'')+' '+r.count+(r.own?' 自分':'')).join('  ')
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.foreground;font.pixelSize:12
            }
            Item {
                width:parent.width;height:26
                Text {anchors.left:parent.left;anchors.verticalCenter:parent.verticalCenter;text:root.message.time||'';textFormat:Text.PlainText;color:ink.muted;font.pixelSize:11}
                ActionButton {objectName:'messageActions-'+(root.message.id||'');anchors.right:parent.right;height:26;implicitWidth:44;text:'⋯';hint:'返信・コピー・リアクション';onClicked:root.actionsRequested()}
            }
        }
    }
}
