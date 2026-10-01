import QtQuick
import QtCore
import 'components/Locale.js' as Locale
import Quickshell.Io

Item {
    id: root
    property url settingsLocation: StandardPaths.writableLocation(StandardPaths.ConfigLocation) + "/omarchy-line/preferences.ini"
    property string systemLocale: Qt.locale().name
    readonly property string savedLanguage: preferences.language
    readonly property string language: Locale.resolveLanguage(savedLanguage, systemLocale)
    Settings {
        id: preferences
        location: root.settingsLocation
        property string language: ''
    }
    property string localStatusKey: '準備しています…'
    function setLocalStatus(value) {localStatusKey=value;var next=Object.assign({},view);next.statusText=tr(value);view=next}
    function setLanguage(value) {
        if(value !== '' && value !== 'ja' && value !== 'en') return
        preferences.language=value;preferences.sync()
    }
    onLanguageChanged: {
        if (!view) return
        if(localStatusKey) setLocalStatus(localStatusKey)
        if(view.account && view.account.nameUnavailable) {
            var next=Object.assign({},view)
            next.account=Object.assign({},view.account,{name:tr('自分')})
            view=next
        }
        command({action:'locale',text:language})
    }
    function tr(value) { return Locale.text(language,value) }
    property var shell: null
    property var manifest: null
    property bool alive: true
    property bool started: false
    property bool startupPending: true
    property string intendedMode: 'live'
    property bool modePending: false
    property bool refreshPending: false
    property bool loginRequested: false
    property bool loginHidden: false
    property int loginSequence: 0
    property string loginRequest: ''
    property string buffer: ''
    property int draftSequence: 0
    property string editedChat: ''
    property string editedSession: ''
    property int editedSelection: -1
    property string editedText: ''
    property var view: ({mode:'live',session:'',selection:0,status:'idle',statusText:root.tr('準備しています…'),avatars:{},chats:[],messages:[],selectedId:'',draft:'',account:{name:root.tr('自分'),nameUnavailable:true},sendStatus:'idle',historyStatus:'idle',busy:false})
    readonly property int unread: (view.chats || []).reduce((n,c)=>n+(c.unread||0),0)
    function safeAvatars(value, mode) {
        if (!value || typeof value!=='object' || Array.isArray(value)) return {}
        var ids=Object.keys(value), images={}, bytes=2
        if (ids.length>500) return images
        for(var id of ids) {
            if (!/^(?:[uUcCrR][A-Za-z0-9_-]{43}|[ucr][0-9a-f]{32})$/.test(id) && !(mode==='demo' && /^demo-[a-z0-9-]{1,64}$/.test(id))) continue
            var data=value[id]
            if (typeof data!=='string' || data.length>87407 || !/^data:image\/(png|jpeg);base64,[A-Za-z0-9+/]+={0,2}$/.test(data)) continue
            if (data.indexOf('data:image/png;base64,iVBORw0KGgo')!==0 && data.indexOf('data:image/jpeg;base64,/9j/')!==0) continue
            bytes+=id.length+data.length+6
            if(bytes>2097152) return {}
            images[id]=data
        }
        return images
    }
    function safeStickers(value) {
        if(!value || typeof value!=='object' || Array.isArray(value))return {}
        var images={},bytes=2,keys=Object.keys(value)
        if(keys.length>140)return {}
        for(var key of keys){
            if(!/^[1-9][0-9]{0,19}(-[a-fA-F0-9]{1,128})?$/.test(key))continue
            var data=value[key]
            if(typeof data!=='string'||data.length>87407||!/^data:image\/(png|jpeg);base64,[A-Za-z0-9+/]+={0,2}$/.test(data))continue
            if(data.indexOf('data:image/png;base64,iVBORw0KGgo')!==0&&data.indexOf('data:image/jpeg;base64,/9j/')!==0)continue
            bytes+=key.length+data.length+6;if(bytes>1048576)return {}
            images[key]=data
        }
        return images
    }
    function safePhotos(value, messages) {
        if(!value || typeof value!=='object' || Array.isArray(value))return {}
        var keys=Object.keys(value), images={}, bytes=2
        if(keys.length>100)return {}
        for(var key of keys) {
            if(!/^[1-9][0-9]{0,19}$/.test(key) || !messages.some(m=>m.id===key && m.contentType===1))continue
            var entry=value[key]
            if(!entry || typeof entry!=='object' || ['loading','ready','error'].indexOf(entry.status)<0)continue
            var data=entry.data, status=entry.status
            if(status==='ready') {
                if(typeof data!=='string'||data.length>87407||!/^data:image\/(png|jpeg);base64,[A-Za-z0-9+/]+={0,2}$/.test(data) || data.indexOf('data:image/png;base64,iVBORw0KGgo')!==0&&data.indexOf('data:image/jpeg;base64,/9j/')!==0){data='';status='error'}
            } else data=''
            bytes+=key.length+data.length+40
            if(bytes>524288)return {}
            images[key]={status:status,data:data}
        }
        return images
    }
    function clearAvatars() { var next=Object.assign({},view);next.avatars={};next.stickerImages={};next.stickers={open:false,selected:null,products:[],items:[]};next.preview='';next.photoThumbnails={};view=next }
    function command(value) {
        if (!alive) return
        if (!started) return
        var data=JSON.stringify(value)
        if(data.length<=60000) bridge.write(data+'\n')
    }
    function selectChat(id) { command({action:'select',id:id}) }
    function editDraft(text) {
        if(text.length>10000) return
        editedChat=view.selectedId;editedSession=view.session;editedSelection=view.selection;editedText=text;draftSequence++
        var next=Object.assign({},view);next.draft=text;view=next
        command({action:'draft',id:editedChat,session:editedSession,selection:editedSelection,text:text,sequence:draftSequence})
    }
    function send(text) { command({action:'send',id:view.selectedId,session:view.session,selection:view.selection,text:text,replyTo:view.reply ? view.reply.id : '',attachmentId:view.attachment ? view.attachment.id : ''}) }
    function feature(action,extra,context) {
        var target=context || {id:view.selectedId,session:view.session,selection:view.selection}
        command(Object.assign({},extra||{},target,{action:action}))
    }
    function notifications(enabled) { command({action:'notifications',enabled:enabled}) }
    function refresh() {
        refreshPending=true
        if (!bridge.running) { started=false;buffer='';startupPending=true;bridge.running=true;return }
        if (started && !modePending) { refreshPending=false;command({action:'refresh'}) }
    }
    function setMode(mode) {
        if(mode!=='live' && mode!=='demo') return
        cancelLogin()
        if(mode!==view.mode) clearAvatars()
        intendedMode=mode;modePending=true;editedChat='';editedText=''
        command({action:'mode',mode:mode,locale:language})
    }
    function login() {
        if(!started || loginRequested || (view.login && view.login.active)) return
        clearAvatars()
        loginHidden=false;loginRequested=true;loginRequest='ui-'+(++loginSequence)
        command({action:'login',request:loginRequest})
    }
    function confirmLogin() { command({action:'login-confirm',attempt:view.login ? view.login.attempt : ''}) }
    function retryLogin() {
        if(!started || loginRequested || (view.login && view.login.active)) return
        clearAvatars()
        loginHidden=false;loginRequested=true;loginRequest='ui-'+(++loginSequence)
        command({action:'login-retry',request:loginRequest})
    }
    function clearLogin() {
        var next=Object.assign({},view)
        next.login={stage:'idle',active:false,image:'',pin:'',statusText:'',canCancel:false,canConfirm:false,canRetry:false}
        view=next
    }
    function cancelLogin() {
        if(loginRequested || (view.login && view.login.active)) command({action:'login-cancel'})
        loginHidden=true;loginRequested=false;clearLogin()
    }
    function consume(chunk) {
        if(!alive) return
        // Producer is capped in bytes; this second guard caps accumulated UTF-16.
        if(buffer.length+chunk.length>5242880) { protocolError();return }
        buffer+=chunk
        var index
        while((index=buffer.indexOf('\n'))>=0) {
            var line=buffer.slice(0,index);buffer=buffer.slice(index+1)
            try {
                var frame=JSON.parse(line)
                if(frame.type!=='state' || !frame.view || !Array.isArray(frame.view.chats) || !Array.isArray(frame.view.messages) || frame.view.chats.length>500 || frame.view.messages.length>100) {protocolError();return}
                var next=frame.view
                // A fresh process initially reports live. Wait for the requested
                // mode before issuing the explicit reload, especially in demo.
                if(next.mode!==intendedMode) continue
                if(next.login) {
                    var currentLogin=(next.login.request||'')===loginRequest
                    if(loginHidden && next.login.active || !currentLogin) next.login={stage:'idle',active:false,image:'',pin:'',statusText:''}
                    if(currentLogin && !next.login.active) loginRequested=false
                }
                if(next.selectedId===editedChat && next.session===editedSession && next.selection===editedSelection && (next.draftAck||0)<draftSequence) next.draft=editedText
                else { editedChat='';editedText='';editedSession='';editedSelection=-1 }
                next.preview=typeof next.preview==='string' && next.preview.length<=2796227 && /^data:image\/(png|jpeg);base64,[A-Za-z0-9+/]+={0,2}$/.test(next.preview) && (next.preview.indexOf('data:image/png;base64,iVBORw0KGgo')===0 || next.preview.indexOf('data:image/jpeg;base64,/9j/')===0) ? next.preview : ''
                next.avatars=loginRequested || (next.login && next.login.active) ? {} : safeAvatars(next.avatars,next.mode)
                next.stickerImages=loginRequested || (next.login && next.login.active) ? {} : safeStickers(next.stickerImages)
                next.photoThumbnails=loginRequested || (next.login && next.login.active) ? {} : safePhotos(next.photoThumbnails,next.messages)
                localStatusKey='';view=next
                modePending=false
                if(refreshPending && started) { refreshPending=false;command({action:'refresh'}) }
            } catch(error) {protocolError();return}
        }
    }
    function protocolError() { clearAvatars();loginHidden=true;loginRequested=false;clearLogin();bridge.running=false;started=false;buffer='';var next=Object.assign({},view);next.session='';next.status='error';localStatusKey='応答を読み取れませんでした。再読み込みしてください';next.statusText=root.tr(localStatusKey);next.sendStatus='idle';view=next }
    Timer {
        objectName:'startupTimeout'
        interval:4000;running:root.startupPending
        onTriggered:if(!root.started){root.startupPending=false;root.protocolError();var next=Object.assign({},root.view);root.localStatusKey='LINE バックエンドを起動できません。プラグインの bin/line-gui を確認して再読み込みしてください';next.statusText=root.tr(root.localStatusKey);root.view=next}
    }
    Process {
        id:bridge
        command:[Qt.resolvedUrl('bin/line-gui').toString().replace(/^file:\/\//,'')]
        stdinEnabled:true
        running:true
        stdout:SplitParser { splitMarker:'';onRead:data=>root.consume(data) }
        stderr:SplitParser { splitMarker:'';onRead:data=>{} }
        onStarted:{root.started=true;root.startupPending=false;root.modePending=true;root.command({action:'mode',mode:root.intendedMode,locale:root.language})}
        onExited:{root.started=false;if(root.alive){root.clearAvatars();root.loginHidden=true;root.loginRequested=false;root.clearLogin();var next=Object.assign({},root.view);next.session='';next.status='error';root.localStatusKey='LINEサービスが停止しました。プラグインの bin/line-gui を確認して再読み込みしてください';next.statusText=root.tr(root.localStatusKey);next.sendStatus='idle';next.watching=false;root.view=next}}
    }
    Component.onDestruction:{alive=false;bridge.running=false;buffer='';clearLogin();clearAvatars()}
    IpcHandler {
        target:'io.github.komagata.line'
        function status():string { return JSON.stringify({mode:root.view.mode,state:root.view.status,chatCount:root.view.chats.length,watching:root.view.watching===true}) }
        function demo():void { root.setMode('demo');if(root.shell) root.shell.summon('io.github.komagata.line','{"demo":true}') }
    }
}
