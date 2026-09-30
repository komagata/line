pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import QtQuick.Dialogs as NativeDialogs
import 'MessageLinks.js' as MessageLinks

Rectangle {
    id: root
    property var service: null
    readonly property var view: service ? service.view : ({chats:[],messages:[],selectedId:'',draft:'',mode:'live',status:'idle',account:{name:'自分'}})
    property bool appVisible: true
    onAppVisibleChanged: if(!appVisible)closeTransient()
    onVisibleChanged: if(!visible)closeTransient()
    property bool windowFocused: false
    readonly property bool conversationFocused: appVisible && visible && windowFocused && !settingsOpen
    onConversationFocusedChanged: reportFocus()
    property bool reportedFocus: false
    function reportFocus() {
        if(!conversationFocused && !reportedFocus)return
        reportedFocus=conversationFocused
        if(service && typeof service.feature==='function')service.feature('focus',{focused:conversationFocused},capture())
    }
    property bool messageSearchOpen: false
    property string messageQuery: ''
    property int searchPosition: -1
    property real searchScroll: 0
    readonly property var searchMatches: {
        var found=[];if(!messageQuery.trim())return found
        var query=messageQuery.toLocaleLowerCase(),rows=view.messages||[]
        for(var i=0;i<rows.length;i++)if(rows[i].text.toLocaleLowerCase().indexOf(query)>=0)found.push(i)
        return found
    }
    function openMessageSearch() { if(!messageSearchOpen)searchScroll=messages.contentY;messageSearchOpen=true;Qt.callLater(function(){messageSearch.forceActiveFocus()}) }
    function closeMessageSearch() { messageSearchOpen=false;messages.contentY=searchScroll;composer.forceActiveFocus() }
    function navigateMatch(step) { if(!searchMatches.length)return;searchPosition=(searchPosition+step+searchMatches.length)%searchMatches.length;followBottom=false;messages.positionViewAtIndex(searchMatches[searchPosition],ListView.Center) }
    property var actionContext: null
    property var actionMessage: ({})
    property var attachmentContext: null
    property var saveContext: null
    property string copiedText: ''
    readonly property bool unsendVisible: unsendDialog.visible
    function capture(messageId) { var value={id:view.selectedId,session:view.session,selection:view.selection};if(messageId)value.messageId=messageId;return value }
    function current(context) {return context && context.id===view.selectedId && context.session===view.session && context.selection===view.selection}
    function perform(action,extra,context) {
        if(!current(context)||!service || typeof service.feature!=='function')return
        if(action==='preview'||action==='download'){fileContext=Object.assign({},context);previewSuppressed=false}
        service.feature(action,extra||{},context)
    }
    property var fileContext: null
    property bool previewSuppressed: false
    property var contactContext: null
    property string contactQuery: ''
    readonly property bool contactsVisible: contactPicker.visible
    readonly property var filteredContacts: (view.contacts||[]).filter(c=>c.name.toLocaleLowerCase().indexOf(contactQuery.toLocaleLowerCase())>=0)
    readonly property bool canStartChat: view.status==='ready' && view.contactsStatus==='ready' && !view.busy && view.sendStatus!=='pending' && view.fileStatus!=='pending' && !(view.login && view.login.active)
    function openContacts(){contactContext=capture();contactQuery='';contactPicker.open();Qt.callLater(function(){contactSearch.forceActiveFocus()})}
    function chooseContact(id){
        if(!canStartChat || !current(contactContext))return
        var context=Object.assign({},contactContext)
        contactPicker.close();perform('start-chat',{contactId:id},context)
    }
    function openActions(record) { actionContext=capture(record.id);actionMessage=Object.assign({},record);actions.open() }
    function chooseAction(action,value) {
        if(!current(actionContext))return
        actions.close()
        if(action==='copy'){clipboard.text=actionMessage.text||'';clipboard.selectAll();clipboard.copy();copiedText=clipboard.text;clipboard.deselect()}
        else if(action==='unsend'){if(actionMessage.own)unsendDialog.open()}
        else if(action==='download'){saveContext=Object.assign({},actionContext);saveDialog.open()}
        else perform(action,value||{},actionContext)
    }
    function confirmUnsend(){perform('unsend',{confirmed:true},actionContext);unsendDialog.close()}
    function chooseAttachment(){attachmentContext=capture();attachDialog.open()}
    function acceptAttachment(url){perform('attach',{url:String(url)},attachmentContext);attachmentContext=null}
    function acceptDownload(url){perform('download',{url:String(url)},saveContext);saveContext=null}
    function closeTransient(cancelFiles) {
        // Clear authority before closing native dialogs: dismissal cannot accept
        // an old file or message action. Keep drafts and staged files intact.
        actionContext=null;actionMessage={};attachmentContext=null;saveContext=null;contactContext=null
        previewSuppressed=true
        if(cancelFiles!==false){perform('file-cancel',{},fileContext||capture());perform('sticker-hide',{},capture())}
        fileContext=null
        actions.close();unsendDialog.close();attachDialog.close();saveDialog.close();contactPicker.close();previewDialog.close()
        messageSearchOpen=false;messageQuery='';contactQuery='';copiedText='';clipboard.text=''
    }
    function quoteFor(record){if(!record.replyTo)return '';var original=(view.messages||[]).find(m=>m.id===record.replyTo);return original?'返信: '+original.text:'返信: 元のメッセージは表示中の100件の範囲外です'}
    function links(text){return MessageLinks.urls(text).slice(0,8)}
    function openLink(url){if(MessageLinks.valid(url))Qt.openUrlExternally(url)}
    property bool settingsOpen: false
    function cancelActiveLogin() { if (service && (service.loginRequested===true || view.login && view.login.active)) service.cancelLogin() }
    onSettingsOpenChanged: { if (!settingsOpen) cancelActiveLogin();else closeTransient() }
    property string searchText: ''
    readonly property var filteredChats: (view.chats || []).filter(c => c.name.toLocaleLowerCase().indexOf(searchText.toLocaleLowerCase()) >= 0)
    readonly property var selected: (view.chats || []).find(c => c.id === view.selectedId) || ({name:'',group:false})
    property string displayedChat: ''
    property bool followBottom: true
    property bool forceBottom: false
    signal dismiss()
    onDismiss:{closeTransient();cancelActiveLogin()}
    Component.onDestruction:closeTransient()
    LinePalette { id: ink }
    color: ink.surface; radius: ink.radius
    border.color: ink.line
    focus: true
    Keys.onEscapePressed: { if(messageSearchOpen)closeMessageSearch();else root.dismiss() }
    Shortcut {sequence:'Ctrl+F';enabled:root.visible && !root.settingsOpen;onActivated:root.openMessageSearch()}
    function stickerFor(s) {return s ? ((view.stickerImages||{})[s.id+(s.hash?'-'+s.hash:'')]||'') : ''}
    function avatarFor(id) { var images=view.avatars || {}; return typeof id==='string' && Object.prototype.hasOwnProperty.call(images,id) ? images[id] : '' }
    function selectChat(id) { if (service) service.selectChat(id) }
    function syncMessages() {
        var rows = view.messages || []
        var changed = displayedChat !== view.selectedId
        if (changed) { history.clear(); displayedChat=view.selectedId||'';followBottom=true }
        var requestedBottom=forceBottom
        var shouldFollow = changed || requestedBottom || followBottom
        forceBottom=false
        var samePrefix = history.count <= rows.length
        for (var i=0; samePrefix && i<history.count; i++) if (history.get(i).record.id !== rows[i].id) samePrefix=false
        var oldY=messages.contentY
        var rowsChanged=!samePrefix || history.count!==rows.length
        if (!samePrefix) history.clear()
        for (var j=0; j<rows.length; j++) {
            if (j>=history.count) history.append({record:rows[j],showDay:j===0 || rows[j].day !== rows[j-1].day})
            else if (['text','senderId','sender','own','time','day','status','encrypted','timestamp','contentType','fileName','downloadable','replyTo'].some(field => history.get(j).record[field] !== rows[j][field]) || JSON.stringify(history.get(j).record.sticker||null) !== JSON.stringify(rows[j].sticker||null) || JSON.stringify(history.get(j).record.reactions||[]) !== JSON.stringify(rows[j].reactions||[]) || history.get(j).showDay !== (j===0 || rows[j].day !== rows[j-1].day)) {
                history.set(j,{record:rows[j],showDay:j===0 || rows[j].day !== rows[j-1].day}); rowsChanged=true
            }
        }
        if (shouldFollow && (changed || rowsChanged || requestedBottom)) Qt.callLater(function() { if(messages) messages.positionViewAtEnd() })
        else if (!samePrefix) Qt.callLater(function() { if(messages) messages.contentY=oldY })
    }
    property string contextKey: ''
    onViewChanged: {
        var key=String(view.session)+'/'+String(view.selection)+'/'+view.selectedId
        if(key!==contextKey){closeTransient(false);messageSearchOpen=false;messageQuery='';contextKey=key;reportFocus()}
        syncMessages()
        if (view.login && view.login.stage==='success' && view.status==='ready') settingsOpen=false
    }
    Component.onCompleted: syncMessages()
    ListModel { id: history; dynamicRoles:true }
    Item {
        anchors.fill: parent; anchors.margins: 1
        visible: !root.settingsOpen
        Item {
            id: sidebar
            width: Math.max(220, Math.round(parent.width*0.305)); height: parent.height
            Row {
                anchors.left:parent.left;anchors.leftMargin:22;anchors.top:parent.top;anchors.topMargin:18;spacing:12
                Text { text:'LINE';textFormat:Text.PlainText;color:ink.foreground;font.family:ink.family;font.pixelSize:29;font.weight:Font.Bold }
                Rectangle { width:9;height:9;radius:5;anchors.verticalCenter:parent.verticalCenter;color:root.view.status==='ready'?ink.accent:ink.muted;visible:root.view.mode!=='demo' }
                Rectangle {
                    visible:root.view.mode==='demo';width:44;height:22;radius:ink.radius;anchors.verticalCenter:parent.verticalCenter;color:ink.selected
                    Text { anchors.centerIn:parent;text:'デモ';textFormat:Text.PlainText;color:ink.foreground;font.family:ink.family;font.pixelSize:11 }
                }
            }
            ActionButton {
                objectName:'settingsButton';anchors.right:parent.right;anchors.rightMargin:14;anchors.top:parent.top;anchors.topMargin:17
                text:'';glyphOnly:true;hint:'設定';onClicked:root.settingsOpen=true
            }
            Rectangle {
                id: searchBox
                anchors.left:parent.left;anchors.right:parent.right;anchors.top:parent.top
                anchors.leftMargin:18;anchors.rightMargin:18;anchors.topMargin:77
                height:44;radius:ink.radius;color:'transparent';border.color:search.activeFocus?ink.accent:ink.line
                Text { x:13;anchors.verticalCenter:parent.verticalCenter;text:'⌕';textFormat:Text.PlainText;color:ink.muted;font.pixelSize:27 }
                TextField {
                    id: search;objectName:'chatSearch'
                    anchors.fill:parent;anchors.leftMargin:40;anchors.rightMargin:8
                    text:root.searchText;onTextChanged:root.searchText=text
                    maximumLength:160;placeholderText:'トークを検索';placeholderTextColor:ink.muted
                    color:ink.foreground;font.family:ink.family;font.pixelSize:ink.body
                    selectByMouse:true;background:Item {}
                    padding:0
                }
            }
            ActionButton {
                id:newChatButton;objectName:'newChatButton';anchors.left:parent.left;anchors.right:parent.right;anchors.leftMargin:18;anchors.rightMargin:18;anchors.top:searchBox.bottom;anchors.topMargin:8
                text:'＋ 新しいトーク';hint:'読み込み済みの連絡先からトークを開く';onClicked:root.openContacts()
            }
            ListView {
                id: chats;objectName:'chatList'
                anchors.left:parent.left;anchors.right:parent.right;anchors.top:newChatButton.bottom;anchors.bottom:selfRow.top
                anchors.topMargin:14;anchors.bottomMargin:12;anchors.leftMargin:3;anchors.rightMargin:3
                clip:true;model:root.filteredChats;spacing:2;boundsBehavior:Flickable.StopAtBounds
                ScrollBar.vertical: ScrollBar { policy:ScrollBar.AsNeeded }
                delegate: Rectangle {
                    id: chatRow
                    required property var modelData
                    width:ListView.view.width;height:82;radius:ink.radius
                    color:root.view.selectedId===modelData.id?ink.selected:rowMouse.containsMouse||activeFocus?ink.hover:'transparent'
                    activeFocusOnTab:true
                    Keys.onReturnPressed:root.selectChat(modelData.id)
                    Keys.onSpacePressed:root.selectChat(modelData.id)
                    Avatar { id:chatAvatar;objectName:'chatAvatar-'+chatRow.modelData.id;imageData:root.avatarFor(chatRow.modelData.id);anchors.left:parent.left;anchors.leftMargin:16;anchors.verticalCenter:parent.verticalCenter;width:sidebar.width<260?40:52;name:chatRow.modelData.name;group:chatRow.modelData.group }
                    Text {
                        id:chatTitle
                        anchors.left:chatAvatar.right;anchors.leftMargin:16;anchors.right:rowTime.left;anchors.rightMargin:8;anchors.top:parent.top;anchors.topMargin:18
                        text:chatRow.modelData.name;textFormat:Text.PlainText;color:ink.foreground;font.family:ink.family;font.pixelSize:ink.body+2;elide:Text.ElideRight
                    }
                    Text {
                        anchors.left:chatTitle.left;anchors.right:parent.right;anchors.rightMargin:chatRow.modelData.unread>0?42:16;anchors.top:chatTitle.bottom;anchors.topMargin:7
                        text:chatRow.modelData.preview;textFormat:Text.PlainText;color:ink.muted;font.family:ink.family;font.pixelSize:ink.body-1;elide:Text.ElideRight
                    }
                    Text { id:rowTime;anchors.right:parent.right;anchors.rightMargin:16;anchors.top:parent.top;anchors.topMargin:20;text:chatRow.modelData.time;textFormat:Text.PlainText;color:ink.muted;font.family:ink.family;font.pixelSize:12 }
                    Rectangle {
                        anchors.right:parent.right;anchors.rightMargin:16;anchors.bottom:parent.bottom;anchors.bottomMargin:14
                        width:24;height:24;radius:12;color:ink.accent;visible:chatRow.modelData.unread>0
                        Text { anchors.centerIn:parent;text:chatRow.modelData.unread>99?'99+':String(chatRow.modelData.unread);textFormat:Text.PlainText;color:ink.surface;font.family:ink.family;font.pixelSize:12;font.weight:Font.DemiBold }
                    }
                    MouseArea { id:rowMouse;anchors.fill:parent;hoverEnabled:true;cursorShape:Qt.PointingHandCursor;onClicked:root.selectChat(chatRow.modelData.id) }
                    Accessible.role:Accessible.ListItem;Accessible.name:modelData.name
                    Accessible.onPressAction:root.selectChat(modelData.id)
                }
                Text {
                    anchors.centerIn:parent;width:parent.width-40;horizontalAlignment:Text.AlignHCenter;wrapMode:Text.Wrap
                    visible:chats.count===0;text:root.searchText?'該当するトークはありません':root.view.status==='ready'?'トークはありません':'設定から接続できます'
                    textFormat:Text.PlainText;color:ink.muted;font.family:ink.family;font.pixelSize:14
                }
            }
            Item {
                id:selfRow;anchors.bottom:parent.bottom;width:parent.width;height:76
                Rectangle { x:12;width:parent.width-24;height:1;color:ink.line }
                Avatar { id:selfAvatar;objectName:'selfAvatar';imageData:root.avatarFor((root.view.account||{}).id);anchors.left:parent.left;anchors.leftMargin:20;anchors.verticalCenter:parent.verticalCenter;width:44;self:true }
                Text { anchors.left:selfAvatar.right;anchors.leftMargin:16;anchors.right:parent.right;anchors.rightMargin:16;anchors.verticalCenter:parent.verticalCenter;text:(root.view.account||{}).name||'自分';textFormat:Text.PlainText;color:ink.foreground;font.family:ink.family;font.pixelSize:ink.body;elide:Text.ElideRight }
            }
            Rectangle { anchors.right:parent.right;width:1;height:parent.height;color:ink.line }
        }
        Item {
            id: conversation
            anchors.left:sidebar.right;anchors.right:parent.right;height:parent.height
            Item {
                id:header;width:parent.width;height:78
                Avatar { id:headerAvatar;objectName:'headerAvatar';imageData:root.avatarFor(root.view.selectedId);anchors.left:parent.left;anchors.leftMargin:22;anchors.verticalCenter:parent.verticalCenter;width:46;name:root.selected.name;group:root.selected.group;visible:root.view.selectedId!=='' }
                Column {
                    anchors.left:headerAvatar.right;anchors.leftMargin:16;anchors.right:searchButton.left;anchors.rightMargin:12;anchors.verticalCenter:parent.verticalCenter;spacing:6
                    Text { width:parent.width;text:root.selected.name||'トーク';textFormat:Text.PlainText;elide:Text.ElideRight;color:ink.foreground;font.family:ink.family;font.pixelSize:ink.body+3 }
                    Text {
                        width:parent.width
                        text:root.view.mode==='demo'?'架空の会話 · デモ':root.view.watching?'新着を受信中':root.view.statusText||''
                        textFormat:Text.PlainText;elide:Text.ElideRight;color:ink.muted;font.family:ink.family;font.pixelSize:12
                    }
                }
                ActionButton { id:searchButton;anchors.right:closeButton.left;anchors.rightMargin:12;anchors.verticalCenter:parent.verticalCenter;text:'󰍉';glyphOnly:true;objectName:'messageSearchButton';hint:'会話内を検索（読み込み済み100件）';enabled:root.view.selectedId!=='';onClicked:root.openMessageSearch() }
                ActionButton { id:closeButton;objectName:'closeButton';anchors.right:parent.right;anchors.rightMargin:14;anchors.verticalCenter:parent.verticalCenter;text:'×';glyphOnly:true;hint:'閉じる';onClicked:root.dismiss() }
                Rectangle { anchors.bottom:parent.bottom;width:parent.width;height:1;color:ink.line }
            }
            Item {
                id:historySearch;anchors.top:header.bottom;width:parent.width;height:root.messageSearchOpen?72:0;visible:root.messageSearchOpen
                TextField {id:messageSearch;objectName:'messageSearch';x:14;y:4;width:parent.width-130;height:34;text:root.messageQuery;onTextChanged:{root.messageQuery=text;root.searchPosition=-1} maximumLength:160;placeholderText:'この会話内を検索';color:ink.foreground;placeholderTextColor:ink.muted;background:Rectangle{color:ink.surface;border.color:ink.line;radius:ink.radius} Keys.onEscapePressed:root.closeMessageSearch();Keys.onReturnPressed:root.navigateMatch(1)}
                Row {anchors.right:parent.right;anchors.rightMargin:8;y:4;spacing:0
                    ActionButton {text:'↑';hint:'前の検索結果';enabled:root.searchMatches.length>0;onClicked:root.navigateMatch(-1)}
                    ActionButton {text:'↓';hint:'次の検索結果';enabled:root.searchMatches.length>0;onClicked:root.navigateMatch(1)}
                    ActionButton {text:'×';hint:'検索を閉じる';onClicked:root.closeMessageSearch()}
                }
                Text {x:16;y:44;text:!root.messageQuery?'読み込み済みの最大100件を検索':root.searchMatches.length?String(root.searchMatches.length)+'件'+(root.searchPosition>=0?' · '+(root.searchPosition+1)+'件目':''):'該当するメッセージはありません';textFormat:Text.PlainText;color:ink.muted;font.pixelSize:12}
            }
            Item {
                id:notice
                anchors.top:historySearch.bottom;width:parent.width
                height:noticeText.text?noticeText.implicitHeight+20:0
                Text {
                    id:noticeText
                    x:22;y:10;width:parent.width-(cancelDownload.visible?110:44);wrapMode:Text.Wrap
                    text:root.view.fileStatus==='pending'?'ファイルを取得しています…':root.view.contactNote||root.view.sendNote||root.view.historyNote||(root.view.status!=='ready'?root.view.statusText||'':root.view.namesPartial?'一部のトーク名を取得できませんでした':'')
                    textFormat:Text.PlainText;color:ink.error;font.family:ink.family;font.pixelSize:13
                }
            }
            ActionButton {id:cancelDownload;anchors.right:parent.right;anchors.rightMargin:12;anchors.top:notice.top;visible:root.view.fileStatus==='pending';text:'中止';onClicked:root.perform('file-cancel',{},root.capture())}
            ListView {
                id:messages;objectName:'messageList'
                anchors.left:parent.left;anchors.right:parent.right;anchors.top:notice.bottom;anchors.bottom:composer.top
                anchors.leftMargin:22;anchors.rightMargin:22;anchors.topMargin:14;anchors.bottomMargin:8
                model:history;clip:true;spacing:4;boundsBehavior:Flickable.StopAtBounds
                onMovementEnded:root.followBottom=atYEnd
                ScrollBar.vertical: ScrollBar { onActiveChanged:if (!active) root.followBottom=messages.atYEnd }
                delegate: Column {
                    id:messageRow
                    required property var record
                    required property bool showDay
                    width:ListView.view.width;spacing:18
                    Item {
                        width:parent.width;height:messageRow.showDay?30:0;visible:messageRow.showDay
                        Rectangle {
                            anchors.centerIn:parent;width:Math.max(70,dateText.implicitWidth+28);height:28;radius:ink.radius;color:'transparent';border.color:ink.line
                            Text { id:dateText;anchors.centerIn:parent;text:messageRow.record.day;textFormat:Text.PlainText;color:ink.muted;font.family:ink.family;font.pixelSize:13 }
                        }
                    }
                    MessageBubble { width:parent.width;message:messageRow.record;replyText:root.quoteFor(messageRow.record);imageData:root.avatarFor(messageRow.record.senderId);stickerImage:root.stickerFor(messageRow.record.sticker);onActionsRequested:root.openActions(messageRow.record);onLinkActivated:url=>root.openLink(url) }
                }
            }
            Column {
                anchors.centerIn:messages;spacing:18;width:Math.min(420,parent.width-60)
                visible:root.view.selectedId==='' || (['loading','ready'].indexOf(root.view.historyStatus)>=0 && history.count===0)
                Text { width:parent.width;horizontalAlignment:Text.AlignHCenter;text:root.view.historyStatus==='loading'?'履歴を読み込んでいます…':root.view.selectedId?'まだメッセージはありません。下の入力欄から送信できます':'トークを選ぶか、「新しいトーク」から連絡先を選んでください';textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.family:ink.family;font.pixelSize:ink.body }
                ActionButton { anchors.horizontalCenter:parent.horizontalCenter;visible:root.view.status!=='ready';text:'設定を開く';accent:true;onClicked:root.settingsOpen=true }
            }
            ChatComposer {
                id:composer
                anchors.left:parent.left;anchors.right:parent.right;anchors.bottom:parent.bottom
                height:implicitHeight
                draft:root.view.draft||'';pending:root.view.sendStatus==='pending' || root.view.fileStatus==='pending'
                reply:root.view.reply||null;attachmentInfo:root.view.attachment||null
                stickerInfo:(root.view.stickers||{}).selected||null;stickerImage:root.stickerFor(stickerInfo)
                onStickerRequested:root.perform('sticker-open',{},root.capture())
                onCancelSticker:root.perform('sticker-close',{},root.capture())
                onAttachRequested:root.chooseAttachment()
                onCancelAttachment:root.perform('attach-cancel',{},root.capture())
                onCancelReply:root.perform('reply-cancel',{},root.capture())
                available:root.view.selectedId!=='' && root.view.status==='ready'
                onEdited:text=>root.service.editDraft(text)
                onSendRequested:text=>{root.forceBottom=true;var s=(root.view.stickers||{}).selected;if(s)root.perform('sticker-send',{text:text,packageId:s.packageId,stickerId:s.id,replyTo:root.view.reply?root.view.reply.id:''},root.capture());else root.service.send(text)}
            }
        }
    }
    StickerPicker {
        width:Math.min(600,root.width-32);height:Math.min(590,root.height-32);x:(root.width-width)/2;y:(root.height-height)/2
        visible:root.appVisible && root.visible && !root.settingsOpen && !!(root.view.stickers||{}).open
        catalog:root.view.stickers||{};images:root.view.stickerImages||{}
        onAction:(action,extra)=>root.perform(action,extra,root.capture())
    }
    TextEdit {id:clipboard;visible:false;textFormat:TextEdit.PlainText}
    NativeDialogs.FileDialog {id:attachDialog;objectName:'attachDialog';title:'送信するファイルを選択（20 MiBまで）';options:NativeDialogs.FileDialog.DontUseNativeDialog;fileMode:NativeDialogs.FileDialog.OpenFile;onAccepted:root.acceptAttachment(selectedFile);onRejected:root.attachmentContext=null}
    NativeDialogs.FileDialog {id:saveDialog;objectName:'saveDialog';title:'新しいファイル名で保存（既存ファイルは上書きしません）';options:NativeDialogs.FileDialog.DontUseNativeDialog;fileMode:NativeDialogs.FileDialog.SaveFile;onAccepted:root.acceptDownload(selectedFile);onRejected:root.saveContext=null}
    Popup {
        id:contactPicker;objectName:'contactPicker';modal:true;focus:true
        width:Math.min(460,root.width-40);height:Math.min(560,root.height-40);x:(root.width-width)/2;y:(root.height-height)/2;padding:18
        background:Rectangle{color:ink.surface;radius:ink.radius;border.color:ink.line}
        onClosed:{root.contactContext=null;root.contactQuery=''}
        contentItem:Item {
            Text {id:contactTitle;text:'新しいトーク';textFormat:Text.PlainText;color:ink.foreground;font.family:ink.family;font.pixelSize:20}
            TextField {
                id:contactSearch;objectName:'contactSearch';anchors.top:contactTitle.bottom;anchors.topMargin:16;width:parent.width;height:42
                text:root.contactQuery;onTextChanged:root.contactQuery=text;maximumLength:160;placeholderText:'連絡先を検索';color:ink.foreground;placeholderTextColor:ink.muted;selectByMouse:true
                background:Rectangle{color:ink.surface;border.color:contactSearch.activeFocus?ink.accent:ink.line;radius:ink.radius}
                Keys.onEscapePressed:contactPicker.close()
                Keys.onReturnPressed:if(root.filteredContacts.length===1)root.chooseContact(root.filteredContacts[0].id)
            }
            Text {
                id:contactHint;anchors.top:contactSearch.bottom;anchors.topMargin:10;width:parent.width;wrapMode:Text.Wrap;textFormat:Text.PlainText;color:ink.muted;font.pixelSize:12
                text:root.view.contactsStatus==='loading'?'連絡先を読み込んでいます…':root.view.contactsStatus==='error'?'連絡先を取得できませんでした。設定から再読み込みしてください':root.view.contactsStatus!=='ready'?'設定から接続すると連絡先を選べます':!root.canStartChat?'処理が終わってから選択してください':root.filteredContacts.length?'読み込み済みの連絡先（最大500件）。選ぶだけでは送信しません':root.contactQuery?'該当する連絡先はありません':'読み込み済みの連絡先はありません'
            }
            ListView {
                id:contactList;objectName:'contactList';anchors.top:contactHint.bottom;anchors.topMargin:10;anchors.bottom:contactCancel.top;anchors.bottomMargin:8;width:parent.width;clip:true;spacing:4;model:root.filteredContacts;boundsBehavior:Flickable.StopAtBounds
                ScrollBar.vertical:ScrollBar{policy:ScrollBar.AsNeeded}
                delegate:Rectangle {
                    id:contactRow;required property var modelData;width:ListView.view.width;height:62;radius:ink.radius;color:activeFocus||contactMouse.containsMouse?ink.hover:'transparent';activeFocusOnTab:true;enabled:root.canStartChat
                    Keys.onReturnPressed:root.chooseContact(modelData.id)
                    Keys.onSpacePressed:root.chooseContact(modelData.id)
                    Keys.onEscapePressed:contactPicker.close()
                    Avatar{id:contactAvatar;objectName:'contactAvatar-'+contactRow.modelData.id;x:8;anchors.verticalCenter:parent.verticalCenter;width:40;name:contactRow.modelData.name;imageData:root.avatarFor(contactRow.modelData.id)}
                    Text {anchors.left:contactAvatar.right;anchors.leftMargin:14;anchors.right:parent.right;anchors.rightMargin:8;anchors.verticalCenter:parent.verticalCenter;text:contactRow.modelData.name;textFormat:Text.PlainText;elide:Text.ElideRight;color:ink.foreground;font.family:ink.family;font.pixelSize:ink.body}
                    MouseArea{id:contactMouse;anchors.fill:parent;hoverEnabled:true;cursorShape:Qt.PointingHandCursor;onClicked:root.chooseContact(contactRow.modelData.id)}
                    Accessible.role:Accessible.ListItem;Accessible.name:modelData.name;Accessible.onPressAction:root.chooseContact(modelData.id)
                }
            }
            ActionButton{id:contactCancel;objectName:'contactCancel';anchors.bottom:parent.bottom;anchors.right:parent.right;text:'キャンセル';onClicked:contactPicker.close()}
        }
    }
    Popup {
        id:actions;objectName:'messageActionMenu';modal:true;focus:true;width:Math.min(390,root.width-40);height:Math.min(menuContent.implicitHeight+28,root.height-40);x:(root.width-width)/2;y:(root.height-height)/2;padding:14
        background:Rectangle{color:ink.surface;radius:ink.radius;border.color:ink.line}
        contentItem:Flickable{contentHeight:menuContent.implicitHeight;clip:true
            Column {id:menuContent;width:parent.width;spacing:4
                Text {width:parent.width;text:root.actionMessage.text||'';textFormat:Text.PlainText;color:ink.muted;elide:Text.ElideRight;font.pixelSize:13}
                Flow {width:parent.width;spacing:6
                    ActionButton{text:'返信';enabled:root.view.sendStatus!=='pending';onClicked:root.chooseAction('reply')}
                    ActionButton{text:'コピー';onClicked:root.chooseAction('copy')}
                    ActionButton{text:'保存';visible:!!root.actionMessage.downloadable;enabled:root.view.fileStatus!=='pending';onClicked:root.chooseAction('download')}
                    ActionButton{text:'画像を見る';visible:root.actionMessage.contentType===1;enabled:root.view.fileStatus!=='pending';onClicked:root.chooseAction('preview')}
                }
                Text {text:'リアクション';textFormat:Text.PlainText;color:ink.muted;font.pixelSize:12}
                Flow {width:parent.width;spacing:2
                    Repeater {model:[{name:'like',label:'👍'},{name:'love',label:'♥'},{name:'laugh',label:'😆'},{name:'surprise',label:'😮'},{name:'sad',label:'😢'},{name:'angry',label:'😡'}]
                        ActionButton {required property var modelData;text:modelData.label;enabled:root.view.sendStatus!=='pending';onClicked:root.chooseAction('react',{reaction:modelData.name})}
                    }
                }
                ActionButton {text:'自分のリアクションを削除';enabled:root.view.sendStatus!=='pending';onClicked:root.chooseAction('react',{remove:true})}
                Repeater {model:root.links(root.actionMessage.text||'')
                    ActionButton {required property string modelData;width:menuContent.width;text:modelData;hint:'選択したHTTP(S)リンクをブラウザで開く';onClicked:root.openLink(modelData)}
                }
                ActionButton {text:'送信を取り消す…';visible:root.actionMessage.own===true;enabled:root.view.sendStatus!=='pending';onClicked:root.chooseAction('unsend')}
                ActionButton {text:'閉じる';onClicked:actions.close()}
            }
        }
    }
    Popup {
        id:unsendDialog;modal:true;focus:true;width:Math.min(420,root.width-40);height:confirmContent.implicitHeight+32;x:(root.width-width)/2;y:(root.height-height)/2;padding:16
        background:Rectangle{color:ink.surface;radius:ink.radius;border.color:ink.line}
        contentItem:Column{id:confirmContent;spacing:16
            Text {width:parent.width;text:'このメッセージの送信を取り消しますか？';textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.foreground}
            Text {width:parent.width;text:root.actionMessage.text||'';textFormat:Text.PlainText;wrapMode:Text.Wrap;maximumLineCount:4;elide:Text.ElideRight;color:ink.muted}
            Row {spacing:12;ActionButton{text:'キャンセル';onClicked:unsendDialog.close()} ActionButton{objectName:'confirmUnsend';text:'送信を取り消す';onClicked:root.confirmUnsend()}}
        }
    }
    Popup {
        id:previewDialog;objectName:'previewDialog';modal:true;focus:true;visible:root.appVisible && root.visible && !root.previewSuppressed && !!root.view.preview;width:Math.min(800,root.width-36);height:Math.min(620,root.height-36);x:(root.width-width)/2;y:(root.height-height)/2;padding:14
        background:Rectangle{color:ink.surface;radius:ink.radius;border.color:ink.line}
        onClosed:if(!root.previewSuppressed && root.view.preview){root.previewSuppressed=true;root.perform('preview-close',{},root.fileContext||root.capture())}
        Image {objectName:'attachmentPreviewImage';anchors.fill:parent;anchors.bottomMargin:48;source:typeof root.view.preview==='string' && /^data:image\/(png|jpeg);base64,[A-Za-z0-9+/]+={0,2}$/.test(root.view.preview) && root.view.preview.length<=2796227 ? root.view.preview : '';sourceSize:Qt.size(1024,1024);fillMode:Image.PreserveAspectFit;cache:false}
        ActionButton{anchors.bottom:parent.bottom;anchors.horizontalCenter:parent.horizontalCenter;text:'閉じる';onClicked:{root.previewSuppressed=true;root.perform('preview-close',{},root.fileContext||root.capture())}}
    }
    SetupView {
        anchors.fill:parent;anchors.margins:1;visible:root.settingsOpen;service:root.service
        onBack:root.settingsOpen=false;onDismiss:root.dismiss()
    }
}
