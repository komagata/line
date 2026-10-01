import QtQuick
import QtTest
import '../..' as Plugin
import '../../components'
import '../fixtures/fictional-avatars.js' as Photos
TestCase {
    id:tests
    name:'ServiceBoundary'
    when:windowShown
    visible:true
    width:1200; height:760
    property var service
    property var process
    Component { id:serviceComponent; Plugin.Service {} }
    Component { id:chatComponent; ChatView { width:1200; height:760 } }
    function state(draft, ack, overrides) {
        return Object.assign({mode:'demo',session:'demo-session',selection:1,status:'ready',statusText:'架空の会話 · デモ',chats:[{id:'demo-0',name:'山田 太郎',preview:'',time:'',unread:0,group:false}],messages:[],selectedId:'demo-0',draft:draft,draftAck:ack,account:{name:'自分'},sendStatus:'idle',historyStatus:'ready'},overrides||{})
    }
    function frame(draft, ack, overrides) { return JSON.stringify({type:'state',view:state(draft,ack,overrides)})+'\n' }
    function commands() { return process.writes.map(line=>JSON.parse(line)) }
    function init() {
        service=createTemporaryObject(serviceComponent,tests)
        verify(service!==null)
        process=findChild(service,'bridgeProcess');verify(process!==null)
        process.started()
        service.setMode('demo');service.consume(frame('',0));process.writes=[]
    }
    function test_sticker_images_are_bounded_and_clear_with_session() {
        var images={'1':Photos.sage,'2-abcd':Photos.sand,'../3':Photos.sage,'4':'https://example.org/sticker.png'}
        service.consume(frame('',0,{stickerImages:images,stickers:{selected:{id:'1'},open:true,products:[]}}))
        compare(Object.keys(service.view.stickerImages).sort().join(','),'1,2-abcd')
        service.setMode('live');compare(Object.keys(service.view.stickerImages).length,0);compare(service.view.stickers.selected,null)
        service.consume(frame('',0,{mode:'live',stickerImages:{'1':Photos.sage}}));process.exited(1,0);compare(Object.keys(service.view.stickerImages).length,0)
        images={};for(var i=1;i<142;i++)images[String(i)]=Photos.sage
        compare(Object.keys(service.safeStickers(images)).length,0)
    }
    function test_photo_current_message_allowlist_and_total_budget() {
        var entries={'101':{status:'ready',data:Photos.sage},'102':{status:'ready',data:'https://example.org/private.png'},'103':{status:'ready',data:'file:///tmp/private.png'},'104':{status:'error',data:Photos.sage},'999':{status:'ready',data:Photos.sage}}
        var messages=[];for(var i=101;i<=104;i++)messages.push({id:String(i),contentType:1})
        service.consume(frame('',0,{photoThumbnails:entries,messages:messages}))
        compare(Object.keys(service.view.photoThumbnails).sort().join(','),'101,102,103,104')
        compare(service.view.photoThumbnails['102'].status,'error');compare(service.view.photoThumbnails['102'].data,'')
        compare(service.view.photoThumbnails['104'].data,'')
        entries={};messages=[]
        for(i=100;i<108;i++){var id=String(i);messages.push({id:id,contentType:1});entries[id]={status:'ready',data:'data:image/png;base64,iVBORw0KGgo'+'A'.repeat(86000)}}
        compare(Object.keys(service.safePhotos(entries,messages)).length,0)
        service.consume(frame('',0,{photoThumbnails:{'101':{status:'ready',data:Photos.sage}},messages:[{id:'101',contentType:0}]}))
        compare(Object.keys(service.view.photoThumbnails).length,0)
    }
    function test_photos_clear_on_mode_login_and_exit() {
        var entries={'101':{status:'ready',data:Photos.sage}},messages=[{id:'101',contentType:1}]
        service.consume(frame('',0,{photoThumbnails:entries,messages:messages}));compare(Object.keys(service.view.photoThumbnails).length,1)
        service.setMode('live');compare(Object.keys(service.view.photoThumbnails).length,0)
        service.consume(frame('',0,{mode:'live',photoThumbnails:entries,messages:messages}));service.login();compare(Object.keys(service.view.photoThumbnails).length,0)
        service.cancelLogin();service.consume(frame('',0,{mode:'live',photoThumbnails:entries,messages:messages}));process.exited(1,0);compare(Object.keys(service.view.photoThumbnails).length,0)
    }
    function test_preview_guard_and_lifecycle_clear() {
        for(var source of ['https://example.org/a','file:///tmp/a','data:image/svg+xml;base64,AAAA']) {service.consume(frame('',0,{preview:source}));compare(service.view.preview,'')}
        service.consume(frame('',0,{preview:Photos.sage}));compare(service.view.preview,Photos.sage)
        process.exited(1,0);compare(service.view.preview,'')
    }
    function test_send_carries_matching_reply_and_attachment_identity() {
        service.consume(frame('',0,{reply:{id:'101',text:'quote'},attachment:{id:'file-uuid',name:'fictional'}}));service.send('')
        var sent=commands()[commands().length-1];compare(sent.replyTo,'101');compare(sent.attachmentId,'file-uuid');compare(sent.id,'demo-0')
    }
    function test_avatar_map_allowlist_and_total_budget() {
        var images={'demo-0':Photos.sage,'demo-self':Photos.sand,'demo-remote':'https://example.invalid/photo','demo-file':'file:///tmp/secret','demo-svg':'data:image/svg+xml;base64,PHN2Zy8+','bad.id':Photos.sage}
        service.consume(frame('',0,{avatars:images}))
        compare(Object.keys(service.view.avatars).sort().join(','),'demo-0,demo-self')
        images={};for(var i=0;i<501;i++) images['demo-'+i]=Photos.sage
        service.consume(frame('',0,{avatars:images}));compare(Object.keys(service.view.avatars).length,0)
        images={};var large='data:image/png;base64,iVBORw0KGgo'+'A'.repeat(86000)
        for(i=0;i<30;i++) images['demo-'+i]=large
        service.consume(frame('',0,{avatars:images}));compare(Object.keys(service.view.avatars).length,0)
        compare(service.view.status,'ready')
    }
    function test_avatar_maps_clear_immediately_on_mode_login_and_process_exit() {
        service.consume(frame('',0,{avatars:{'demo-0':Photos.sage}}));service.setMode('live')
        compare(Object.keys(service.view.avatars).length,0)
        var id='u'+'1'.repeat(32),images={};images[id]=Photos.sage
        service.consume(frame('',0,{mode:'live',avatars:images}));compare(Object.keys(service.view.avatars).length,1)
        service.login();compare(Object.keys(service.view.avatars).length,0)
        service.cancelLogin();service.consume(frame('',0,{mode:'live',avatars:images}))
        process.exited(1,0);compare(Object.keys(service.view.avatars).length,0)
    }
    function test_stream_chunks_and_stale_draft_ack() {
        var input=frame('元の下書き',0)
        service.consume(input.slice(0,17));compare(service.buffer,input.slice(0,17))
        service.consume(input.slice(17));compare(service.view.draft,'元の下書き')
        service.editDraft('入力中の日本語');service.consume(frame('元の下書き',0));compare(service.view.draft,'入力中の日本語')
        service.consume(frame('',service.draftSequence));compare(service.view.draft,'')
    }
    function test_visible_identity_is_sent_even_while_selection_response_is_delayed() {
        service.consume(frame('A draft',0));service.selectChat('demo-1')
        service.editDraft('A visible final');service.send('A visible final')
        var sent=commands()
        compare(sent[0].action,'select');compare(sent[0].id,'demo-1')
        for(var i=1;i<3;i++) {
            compare(sent[i].id,'demo-0');compare(sent[i].session,'demo-session')
            compare(sent[i].selection,1);compare(sent[i].text,'A visible final')
        }
    }
    function test_actual_composer_immediate_enter_uses_latest_editor_text() {
        var chat=createTemporaryObject(chatComponent,tests,{service:service})
        var editor=findChild(chat,'messageInput')
        editor.text='入力したばかりの日本語'
        compare(service.view.draft,editor.text)
        service.consume(frame('古い応答',0))
        compare(editor.text,'入力したばかりの日本語')
        editor.forceActiveFocus();keyClick(Qt.Key_Return)
        var sent=commands();compare(sent.length,2)
        compare(sent[0].action,'draft');compare(sent[1].action,'send')
        compare(sent[1].text,'入力したばかりの日本語')
        compare(sent[1].id,'demo-0');compare(sent[1].session,'demo-session')
    }
    function test_draft_mask_never_crosses_selection_or_session() {
        service.editDraft('古い入力')
        service.consume(frame('new selection',0,{selection:3}))
        compare(service.view.draft,'new selection')
        service.editDraft('old session input')
        service.consume(frame('new session',0,{selection:3,session:'restarted-demo'}))
        compare(service.view.draft,'new session')
    }
    function test_stopped_demo_refresh_restores_mode_before_refreshing() {
        process.running=false;process.exited(1,0);process.writes=[]
        service.refresh();verify(process.running);compare(commands().length,0)
        process.started()
        compare(commands().length,1);compare(commands()[0].action,'mode');compare(commands()[0].mode,'demo')
        // Fresh process starts in live; this frame must not trigger a live refresh.
        service.consume(frame('',0,{mode:'live',session:'restarted-live',selectedId:''}))
        compare(commands().length,1)
        service.consume(frame('',0,{session:'restarted-demo'}))
        compare(commands().length,2);compare(commands()[1].action,'refresh')
        service.consume(frame('',0,{session:'restarted-demo'}));compare(commands().length,2)
    }
    function test_stopped_live_refresh_needs_only_one_explicit_click() {
        service.setMode('live');service.consume(frame('',0,{mode:'live',session:'live-session',selectedId:''}))
        process.running=false;process.exited(1,0);process.writes=[]
        service.refresh();process.started()
        compare(commands().length,1);compare(commands()[0].action,'mode');compare(commands()[0].mode,'live')
        service.consume(frame('',0,{mode:'live',session:'restarted-live',selectedId:''}))
        compare(commands().length,2);compare(commands()[1].action,'refresh')
        service.consume(frame('',0,{mode:'live',session:'restarted-live',selectedId:''}));compare(commands().length,2)
    }
    function test_stopped_mode_choice_survives_restart_without_login_or_send() {
        process.running=false;process.exited(1,0);process.writes=[]
        service.setMode('live');service.setMode('demo');service.refresh();process.started()
        compare(commands().length,1);compare(commands()[0].mode,'demo')
        service.consume(frame('',0,{session:'new-demo'}))
        compare(commands().map(c=>c.action).join(','),'mode,refresh')
    }
    function test_initial_start_does_not_refresh_or_login() {
        var fresh=createTemporaryObject(serviceComponent,tests)
        var freshProcess=findChild(fresh,'bridgeProcess');freshProcess.started()
        fresh.consume(frame('',0,{mode:'live',session:'fresh-live',selectedId:''}))
        verify(freshProcess.writes.every(line=>JSON.parse(line).action==='mode'))
    }
    function test_refresh_while_starting_is_retained() {
        var fresh=createTemporaryObject(serviceComponent,tests)
        var freshProcess=findChild(fresh,'bridgeProcess')
        fresh.refresh();freshProcess.started()
        fresh.consume(frame('',0,{mode:'live',session:'fresh-live',selectedId:''}))
        compare(freshProcess.writes.map(line=>JSON.parse(line).action).join(','),'mode,refresh')
    }
    function test_overflow_fails_closed() {
        service.buffer='';service.consume('x'.repeat(5242881));compare(service.view.status,'error');compare(service.buffer,'')
    }
    function test_fixed_native_backend_path() {
        verify(process.command.length===1)
        verify(process.command[0].endsWith("/bin/line-gui"))
    }
    function test_missing_runtime_failure_visible() {
        service.started=false;service.startupPending=true
        findChild(service,'startupTimeout').triggered()
        compare(service.view.status,'error')
    }
    function test_invalid_shape_fails_closed() {
        service.consume('{"type":"state","view":{"chats":{}}}\n');compare(service.view.status,'error')
    }
    function test_close_before_login_response_cancels_and_suppresses_late_secrets() {
        service.setMode('live');service.consume(frame('',0,{mode:'live'}));process.writes=[]
        service.login();service.cancelLogin()
        compare(commands().map(c=>c.action).join(','),'login,login-cancel')
        service.consume(frame('',0,{mode:'live',login:{request:service.loginRequest,stage:'scan',active:true,image:'private-image',pin:'123456'}}))
        compare(service.view.login.image,'');compare(service.view.login.pin,'');verify(!service.view.login.active)
    }
    function test_mode_change_and_runtime_exit_clear_login_immediately() {
        service.setMode('live');service.consume(frame('',0,{mode:'live'}));service.login()
        service.consume(frame('',0,{mode:'live',login:{request:service.loginRequest,stage:'scan',active:true,image:'private-image',pin:'123456'}}))
        compare(service.view.login.image,'private-image')
        service.setMode('demo');compare(service.view.login.image,'');compare(service.view.login.pin,'')
        service.consume(frame('',0));service.setMode('live');service.consume(frame('',0,{mode:'live'}))
        service.login()
        service.consume(frame('',0,{mode:'live',login:{request:service.loginRequest,stage:'scan',active:true,image:'private-image',pin:'123456'}}))
        compare(service.view.login.image,'private-image')
        process.exited(1,0);compare(service.view.login.image,'');compare(service.view.login.pin,'')
    }
    function test_new_attempt_does_not_accept_old_queued_login_frame() {
        service.setMode('live');service.consume(frame('',0,{mode:'live'}));service.login()
        var first=commands().filter(c=>c.action==='login').pop()
        service.cancelLogin();service.login()
        var second=commands().filter(c=>c.action==='login').pop()
        verify(typeof first.request==='string' && first.request!==second.request)
        service.consume(frame('',0,{mode:'live',login:{request:first.request,stage:'scan',active:true,image:'old-image',pin:'123456'}}))
        compare(service.view.login.image,'');compare(service.view.login.pin,'')
    }
    function test_status_ipc_omits_login_secrets() {
        service.setMode('live');service.consume(frame('',0,{mode:'live'}));service.login()
        service.consume(frame('',0,{mode:'live',login:{request:service.loginRequest,stage:'scan',active:true,image:'private-image',pin:'123456'}}))
        compare(service.view.login.image,'private-image')
        var ipc=null
        for(var i=0;i<service.data.length;i++) if(service.data[i].target==='io.github.komagata.line') ipc=service.data[i]
        verify(ipc!==null)
        var status=JSON.parse(ipc.status())
        compare(Object.keys(status).sort().join(','),'chatCount,mode,state,watching')
        verify(ipc.status().indexOf('private-image')<0);verify(ipc.status().indexOf('123456')<0)
    }
    function test_same_mode_navigation_cancels_pending_login() {
        service.setMode('live');service.consume(frame('',0,{mode:'live'}));process.writes=[]
        service.login();service.setMode('live')
        compare(commands().map(c=>c.action).join(','),'login,login-cancel,mode')
    }
    function test_cancellation_can_show_uncertain_result_without_secrets() {
        service.setMode('live');service.consume(frame('',0,{mode:'live'}));service.login()
        var request=commands().filter(c=>c.action==='login').pop().request
        service.cancelLogin()
        service.consume(frame('',0,{mode:'live',login:{request:request,stage:'uncertain',active:false,image:'',pin:'',statusText:'結果を確認してください'}}))
        compare(service.view.login.stage,'uncertain');compare(service.view.login.image,'')
    }
    function test_runtime_exit_allows_new_explicit_login_after_restart() {
        service.setMode('live');service.consume(frame('',0,{mode:'live'}));service.login()
        process.exited(1,0);process.started();process.writes=[]
        service.login();compare(commands().filter(c=>c.action==='login').length,1)
    }
}
