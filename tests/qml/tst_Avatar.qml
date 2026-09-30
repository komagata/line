import QtQuick
import QtTest
import '../../components'
import '../fixtures/fictional-avatars.js' as Photos

TestCase {
    id: tests
    name: 'Avatars'
    when: windowShown
    visible: true; width: 1200; height: 760
    Component { id: avatarComponent; Avatar {} }
    Component { id: chatComponent; ChatView { anchors.fill:parent } }
    Rectangle { id: background; width:100; height:100; color:'#202020' }
    QtObject {
        id: fixture
        property var view: ({})
        function selectChat(id) { var next=Object.assign({},view);next.selectedId=id;view=next }
        function editDraft(text) { var next=Object.assign({},view);next.draft=text;view=next }
        function send(text) {}
        function cancelLogin() {}
    }
    function initial() {
        return {mode:'demo',status:'ready',statusText:'架空の会話 · デモ',chats:[
            {id:'demo-a',name:'山田 太郎',preview:'駅前のカフェでどう？',time:'19:40',unread:0,group:false},
            {id:'demo-group',name:'週末の集まり',preview:'土曜日、楽しみにしてます！',time:'19:32',unread:2,group:true},
            {id:'demo-b',name:'山田 太郎',preview:'また来週！',time:'18:15',unread:0,group:false},
            {id:'demo-missing',name:'鈴木 健',preview:'了解です',time:'17:08',unread:0,group:false}],
            selectedId:'demo-a',draft:'じゃあ、明日19時に！',account:{id:'demo-self',name:'自分'},historyStatus:'ready',sendStatus:'idle',messages:[
            {id:'m1',senderId:'demo-a',sender:'山田 太郎',text:'明日の待ち合わせ、何時にする？',own:false,time:'19:35',day:'今日'},
            {id:'m2',senderId:'demo-self',sender:'自分',text:'19時くらいでどうかな？',own:true,time:'19:36',day:'今日'},
            {id:'m3',senderId:'demo-b',sender:'山田 太郎',text:'いいね！',own:false,time:'19:38',day:'今日'},
            {id:'m4',senderId:'demo-missing',sender:'鈴木 健',text:'駅前のカフェでどう？',own:false,time:'19:40',day:'今日'}],avatars:{}}
    }
    function avatar() {
        var item=createTemporaryObject(avatarComponent,background,{width:60,name:'山田'})
        verify(item!==null);verify('imageData' in item,'Avatar must provide a safe imageData boundary')
        return item
    }
    function test_formats_circular_crop_and_source_bounds() {
        var item=avatar()
        for(var size of [38,40,44,46,52]) for(var data of [Photos.sage,Photos.sand]) {
            item.width=size
            item.imageData=data;tryCompare(item,'imageReady',true)
            var image=findChild(item,'avatarImage');verify(image!==null)
            verify(image.sourceSize.width<=512 && image.sourceSize.height<=512)
            wait(30)
            var pixels=grabImage(background)
            compare(pixels.pixel(1,1),Qt.rgba(32/255,32/255,32/255,1),'circular corner reveals background')
            verify(!Qt.colorEqual(pixels.pixel(size/2,size/2),Qt.rgba(32/255,32/255,32/255,1)),'photo center is rendered')
            verify(!Qt.colorEqual(pixels.pixel(size/2,2),item.color),'photo replaces initial background')
        }
    }
    function test_cover_crop_preserves_non_square_aspect_ratio() {
        var item=avatar();item.width=40
        // 400x100 RGB stripes: cover must crop both outer stripes entirely.
        item.imageData='data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAZAAAABkCAIAAAAnqfEgAAADV0lEQVR4nO3OsQ0DQRDEsOu/abuIBZ6BBmAuvd978z1/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRP0jS/Sh/MEf+IEn3o/zBHPmDJN2P8gdz5A+SdD/KH8yRPyj6A2C2rNY+Io7vAAAAAElFTkSuQmCC'
        tryCompare(item,'imageReady',true)
        tryVerify(function(){return Qt.colorEqual(grabImage(background).pixel(8,20),'#00ff00')})
        var pixels=grabImage(background)
        compare(pixels.pixel(32,20),'#00ff00')
        compare(pixels.pixel(1,1),'#202020')
    }
    function test_same_identity_new_source_and_rapid_valid_invalid_valid() {
        var item=avatar();item.width=40
        item.imageData=Photos.sage;tryCompare(item,'imageReady',true)
        tryVerify(function(){return !Qt.colorEqual(grabImage(background).pixel(20,20),item.color)})
        var first=grabImage(background).pixel(20,20)
        item.imageData=Photos.sand;tryCompare(item,'imageReady',true)
        tryVerify(function(){var p=grabImage(background).pixel(20,20);return !Qt.colorEqual(p,first)&&!Qt.colorEqual(p,item.color)})
        item.imageData=Photos.sage
        item.imageData='https://example.invalid/never-fetch'
        compare(item.imageReady,false);compare(item.safeSource,'')
        item.imageData=Photos.blue;tryCompare(item,'imageReady',true)
        tryVerify(function(){return Qt.colorEqual(grabImage(background).pixel(20,20),'#cbddec')})
        compare(findChild(item,'avatarInitial').visible,false)
    }
    function test_ready_photo_renders_after_parent_becomes_visible() {
        for(var i=0;i<8;i++) {
            background.visible=false
            var item=avatar();item.self=true;item.width=44;item.imageData=Photos.blue
            tryCompare(item,'imageReady',true)
            background.visible=true;wait(30)
            verify(!Qt.colorEqual(grabImage(background).pixel(22,22),item.color),'ready photo becomes visible with its parent')
            item.destroy();wait(1)
        }
    }
    function test_ready_photo_attached_to_window_after_loading() {
        var item=createTemporaryObject(avatarComponent,null,{width:44,self:true,imageData:Photos.blue})
        verify(item!==null);tryCompare(item,'imageReady',true)
        item.parent=background
        tryVerify(function(){return !Qt.colorEqual(grabImage(background).pixel(22,22),item.color)},1000,'photo loaded before Canvas has a window must paint when attached')
    }
    function test_rejects_arbitrary_sources_and_fallbacks() {
        var item=avatar()
        for(var data of ['https://example.invalid/photo','file:///tmp/no-read','data:image/svg+xml;base64,PHN2Zy8+','data:image/gif;base64,R0lGODlh','data:image/png;base64,'+'A'.repeat(88000)]) {
            item.imageData=data;compare(item.safeSource,'');compare(item.imageReady,false)
        }
        item.imageData=Photos.sage;tryCompare(item,'imageReady',true)
        item.group=true;compare(item.safeSource,'');compare(item.imageReady,false)
        item.group=false;item.imageData='data:image/png;base64,iVBORw0KGgoAAAAA';tryCompare(item,'imageReady',false)
        verify(findChild(item,'avatarInitial').visible)
        item.imageData='';item.self=true;compare(findChild(item,'avatarInitial').text,'自')
    }
    function test_late_map_updates_same_name_identity_and_scroll_without_resetting_draft() {
        fixture.view=initial()
        var chat=createTemporaryObject(chatComponent,tests,{service:fixture})
        verify(chat!==null);var header=findChild(chat,'headerAvatar');verify(header!==null)
        verify('imageReady' in header,'header uses native photo avatar')
        var editor=findChild(chat,'messageInput');editor.text='入力途中'
        chat.followBottom=false
        var messages=findChild(chat,'messageList');verify(messages!==null);messages.contentY=-8
        var oldY=messages.contentY
        var next=Object.assign({},fixture.view);next.avatars={'demo-a':Photos.sage,'demo-b':Photos.sand,'demo-self':Photos.blue};fixture.view=next
        tryCompare(header,'imageReady',true);compare(editor.text,'入力途中');compare(fixture.view.selectedId,'demo-a');wait(30);compare(messages.contentY,oldY)
        compare(findChild(chat,'messageAvatar-m1').imageData,Photos.sage)
        compare(findChild(chat,'messageAvatar-m3').imageData,Photos.sand)
        verify(!findChild(chat,'messageAvatar-m4').imageReady)
        // No text change: sender identity/name updates must still reach ListModel.
        next=Object.assign({},fixture.view);next.messages=next.messages.map(m=>Object.assign({},m));next.messages[0].senderId='demo-b';next.messages[0].sender='別の人';fixture.view=next
        tryCompare(findChild(chat,'messageAvatar-m1'),'imageData',Photos.sand)
        compare(findChild(chat,'messageAvatar-m1').name,'別の人')
        fixture.selectChat('demo-group');wait(30)
        verify(!findChild(chat,'headerAvatar').imageReady)
        compare(findChild(chat,'messageAvatar-m1').imageData,Photos.sand)
    }
    function test_image_completion_preserves_chat_list_scroll_and_delegates() {
        fixture.view=initial();var next=Object.assign({},fixture.view)
        next.chats=next.chats.slice();for(var i=0;i<35;i++)next.chats.push({id:'demo-extra-'+i,name:'架空の人 '+i,preview:'会話',time:'12:00',group:false,unread:0})
        fixture.view=next
        var chat=createTemporaryObject(chatComponent,tests,{service:fixture});wait(30)
        var list=findChild(chat,'chatList');verify(list!==null,'chat list must be inspectable for scroll regression')
        list.contentY=700;wait(30);var oldY=list.contentY
        var row=list.itemAtIndex(10);verify(row!==null)
        next=JSON.parse(JSON.stringify(fixture.view));next.avatars={'demo-a':Photos.sage};fixture.view=next;wait(30)
        compare(list.contentY,oldY);compare(list.itemAtIndex(10),row,'unchanged list delegates retained')
    }
    function test_avatar_only_frame_does_not_pull_history_to_bottom() {
        fixture.view=initial();var next=Object.assign({},fixture.view)
        next.messages=[];for(var i=0;i<45;i++)next.messages.push({id:'long-'+i,senderId:'demo-a',sender:'山田',text:'架空の会話 '+i,own:false,time:'12:00',day:'今日'})
        fixture.view=next
        var chat=createTemporaryObject(chatComponent,tests,{service:fixture});wait(40)
        var list=findChild(chat,'messageList');verify(list!==null)
        // A movement may be in progress before onMovementEnded updates followBottom.
        list.contentY=100;chat.followBottom=true;var oldY=list.contentY
        next=JSON.parse(JSON.stringify(fixture.view));next.avatars={'demo-a':Photos.sage};fixture.view=next;wait(40)
        compare(list.contentY,oldY)
        chat.forceBottom=true;next=Object.assign({},fixture.view);next.sendStatus='pending';fixture.view=next;wait(30)
        verify(list.atYEnd,'explicit send still follows the bottom')
    }
    function test_self_photo_eventually_paints_in_fresh_chat_views() {
        for(var i=0;i<12;i++) {
            fixture.view=initial();var next=Object.assign({},fixture.view);next.avatars={'demo-a':Photos.sage,'demo-b':Photos.sand,'demo-self':Photos.blue};fixture.view=next
            var chat=createTemporaryObject(chatComponent,tests,{service:fixture})
            var self=findChild(chat,'selfAvatar');tryCompare(self,'imageReady',true)
            var point=self.mapToItem(chat,self.width/2,self.height/2)
            tryVerify(function(){return !Qt.colorEqual(grabImage(chat).pixel(Math.round(point.x),Math.round(point.y)),self.color)},1000,'self photo must paint after async readiness')
            chat.destroy();wait(1)
        }
    }
    function test_compact_wide_screenshots_with_ready_png_and_jpeg() {
        fixture.view=initial();var next=Object.assign({},fixture.view);next.avatars={'demo-a':Photos.sage,'demo-b':Photos.sand,'demo-self':Photos.blue};fixture.view=next
        var chat=createTemporaryObject(chatComponent,tests,{service:fixture})
        verify(chat!==null);var header=findChild(chat,'headerAvatar');verify(header!==null)
        verify('imageReady' in header,'header photo binding present')
        for(var size of [[720,540,'compact'],[1200,760,'wide']]) {
            tests.width=size[0];tests.height=size[1];wait(60)
            tryCompare(header,'imageReady',true)
            tryCompare(findChild(chat,'selfAvatar'),'imageReady',true)
            tryCompare(findChild(chat,'messageAvatar-m1'),'imageReady',true)
            tryCompare(findChild(chat,'messageAvatar-m3'),'imageReady',true)
            var self=findChild(chat,'selfAvatar'), photo=findChild(self,'avatarImage')
            var center=self.mapToItem(chat,self.width/2,self.height/2)
            // Image.Ready precedes the Canvas paint event. Wait for the actual
            // photo pixel, not only the image decoder, before saving evidence.
            tryVerify(function(){return !Qt.colorEqual(grabImage(chat).pixel(Math.round(center.x),Math.round(center.y)),self.color)},1000,'self photo pixels must render after Image.Ready')
            grabImage(chat).save('/tmp/omarchy-line-avatars-'+size[2]+'.png')
        }
    }
}
