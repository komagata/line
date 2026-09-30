import QtQuick
import QtQuick.Controls

Item {
    id: root
    objectName: 'composer'
    property string draft: ''
    property var reply: null
    property var attachmentInfo: null
    property var stickerInfo: null
    property string stickerImage: ''
    signal stickerRequested()
    signal cancelSticker()
    readonly property int summaryHeight: (reply ? 38 : 0) + (attachmentInfo ? 40 : 0) + (stickerInfo ? 88 : 0)
    readonly property bool canSend: available && !pending && (stickerInfo ? editor.text.length===0 && !attachmentInfo : attachmentInfo ? editor.text.length === 0 : editor.text.trim().length > 0)
    signal attachRequested()
    signal cancelAttachment()
    signal cancelReply()
    property bool pending: false
    property bool available: false
    property bool updating: false
    property bool imeGuard: false
    signal edited(string text)
    signal sendRequested(string text)
    LinePalette { id: ink }
    implicitHeight: summaryHeight + Math.max(110, Math.min(184, editor.contentHeight + 60))
    function syncDraft() {
        if (editor.text === draft) return
        updating = true; editor.text = draft; updating = false
    }
    onDraftChanged: syncDraft()
    Component.onCompleted: syncDraft()
    function handleKey(event, composing) {
        if (event.key !== Qt.Key_Return && event.key !== Qt.Key_Enter) return
        if (composing || imeGuard) return
        if (event.modifiers & Qt.ShiftModifier) return
        event.accepted = true
        if (canSend) sendRequested(editor.text)
    }
    Rectangle { width: parent.width; height: 1; color: ink.line }
    Column {
        x:16;y:4;width:parent.width-32
        Item {
            objectName:'replySummary';width:parent.width;height:visible?38:0;visible:!!root.reply
            Text {anchors.left:parent.left;anchors.right:cancelReplyButton.left;anchors.verticalCenter:parent.verticalCenter;text:root.reply?'返信: '+root.reply.text:'';textFormat:Text.PlainText;elide:Text.ElideRight;color:ink.muted;font.pixelSize:13}
            ActionButton {id:cancelReplyButton;anchors.right:parent.right;text:'×';hint:'返信を解除';enabled:!root.pending;onClicked:root.cancelReply()}
        }
        Item {
            objectName:'attachmentSummary';width:parent.width;height:visible?40:0;visible:!!root.attachmentInfo
            Text {anchors.left:parent.left;anchors.right:cancelFileButton.left;anchors.verticalCenter:parent.verticalCenter;text:root.attachmentInfo?root.attachmentInfo.name+' ('+Math.ceil(root.attachmentInfo.size/1024)+' KiB)':'';textFormat:Text.PlainText;elide:Text.ElideRight;color:ink.foreground;font.pixelSize:13}
            ActionButton {id:cancelFileButton;anchors.right:parent.right;text:'×';hint:'添付を解除';enabled:!root.pending;onClicked:root.cancelAttachment()}
        }
        Item {
            objectName:'stickerSummary';width:parent.width;height:visible?88:0;visible:!!root.stickerInfo
            StickerImage {id:stage;objectName:'stagedStickerImage';width:76;height:76;imageData:root.stickerImage}
            Text {anchors.left:stage.right;anchors.leftMargin:10;anchors.right:cancelStickerButton.left;anchors.verticalCenter:parent.verticalCenter;text:root.stickerInfo?root.stickerInfo.alt+'\n送信ボタンで送ります':'';textFormat:Text.PlainText;wrapMode:Text.Wrap;maximumLineCount:3;elide:Text.ElideRight;color:ink.foreground;font.pixelSize:13}
            ActionButton {id:cancelStickerButton;anchors.right:parent.right;text:'×';hint:'スタンプの選択を解除';enabled:!root.pending;onClicked:root.cancelSticker()}
        }
    }
    ActionButton {
        id: attachment
        anchors.left: parent.left; anchors.leftMargin: 15
        anchors.verticalCenter: field.verticalCenter
        objectName:'attachButton';text:'＋';glyphOnly:true;enabled:root.available && !root.pending && !root.stickerInfo
        hint:'ファイルを選択（まだ送信しません）';onClicked:root.attachRequested()
    }
    ActionButton {
        id:stickerButton;objectName:'stickerButton';anchors.left:attachment.right;anchors.verticalCenter:field.verticalCenter
        width:38;text:'☺';glyphOnly:true;hint:'所有スタンプを選択';enabled:root.available && !root.pending && !root.attachmentInfo;onClicked:root.stickerRequested()
    }
    Rectangle {
        id: field
        anchors.left: stickerButton.right; anchors.leftMargin: 9
        anchors.right: sendButton.left; anchors.rightMargin: 12
        anchors.top: parent.top; anchors.topMargin: 16 + root.summaryHeight
        height: parent.height - 48 - root.summaryHeight
        radius: ink.radius; color: 'transparent'; border.color: editor.activeFocus ? ink.accent : ink.line
        Flickable {
            id: flick
            anchors.fill: parent; anchors.margins: 12
            contentWidth: width; contentHeight: editor.contentHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            TextArea.flickable: TextArea {
                id: editor
                objectName: 'messageInput'
                textFormat: TextEdit.PlainText
                wrapMode: TextEdit.Wrap
                color: ink.foreground; selectionColor: ink.selected
                selectedTextColor: ink.foreground
                font.family: ink.family; font.pixelSize: ink.body + 2
                padding: 0
                placeholderText: root.available ? 'メッセージを入力' : '接続後にメッセージを入力できます'
                placeholderTextColor: ink.muted
                readOnly: !root.available
                selectByMouse: true
                background: Item {}
                onTextChanged: {
                    if (text.length > 10000) { remove(10000,text.length); return }
                    if (!root.updating) root.edited(text)
                }
                onPreeditTextChanged: {
                    if (preeditText.length > 0) root.imeGuard = true
                    else Qt.callLater(function() { root.imeGuard = false })
                }
                Keys.priority: Keys.BeforeItem
                Keys.onPressed: event => root.handleKey(event, inputMethodComposing || preeditText.length > 0)
            }
        }
    }
    ActionButton {
        id: sendButton
        objectName: 'sendButton'
        anchors.right: parent.right; anchors.rightMargin: 16
        anchors.top: field.top
        width: 54; height: field.height
        accent: true; glyphOnly: true; text: root.pending ? '…' : '➤'
        hint: 'メッセージを送信'
        enabled: root.canSend
        onClicked: root.sendRequested(editor.text)
    }
    Text {
        anchors.left: field.left; anchors.right:parent.right;anchors.rightMargin:12; anchors.top: field.bottom; anchors.topMargin: 8
        elide:Text.ElideRight
        text: root.stickerInfo ? (editor.text.length ? '本文を空にするとスタンプを送信できます' : 'スタンプのみ送信 · 静止画・音声なし') : root.attachmentInfo ? (editor.text.length ? '本文とファイルは別々に送信してください' : '画像も汎用ファイルとして送信 · 最大20 MiB') : 'Enterで送信 · Shift+Enterで改行'
        textFormat: Text.PlainText
        color: ink.muted; font.family: ink.family; font.pixelSize: 12
    }
}
