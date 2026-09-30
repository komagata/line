import QtQuick
import QtTest
import '../../components'

TestCase {
    id: testCase
    name: 'ChatView'
    when: windowShown
    visible: true
    width: 1200; height: 760
    QtObject {
        id: fixture
        property var drafts: ({})
        property int sends: 0
        property string sentText: ''
        property var view: ({mode:'demo', status:'ready', statusText:'架空の会話 · デモ', chats:[{id:'demo-0', name:'山田 太郎',preview:'駅前のカフェでどう？',time:'19:40',unread:0,group:false},{id:'demo-1', name:'週末の集まり',preview:'土曜日、楽しみにしてます！',time:'19:32',unread:2,group:true}],selectedId:'demo-0',messages:[],draft:'',account:{name:'自分'},historyStatus:'ready',historyNote:'',sendStatus:'idle',sendNote:'',busy:false})
        function selectChat(id) { var n=Object.assign({},view); n.selectedId=id; n.draft=drafts[id]||''; view=n }
        function editDraft(text) { drafts[view.selectedId]=text; var n=Object.assign({},view);n.draft=text;view=n }
        function send(text) { if (!text || !text.trim()) return; sentText=text;sends++;editDraft('') }
        function refresh() {}
        function setMode(mode) {}
        function login() {}
    }
    ChatView { id: chat; anchors.fill: parent; service: fixture }
    SignalSpy { id: closed; target: chat; signalName:'dismiss' }
    function init() { chat.settingsOpen=false;chat.searchText='';fixture.drafts={};fixture.sends=0;fixture.selectChat('demo-0');fixture.editDraft('');closed.clear();wait(30) }
    function test_search_and_selection() {
        var search=findChild(chat,'chatSearch');verify(search!==null);
        search.text='週末';compare(chat.filteredChats.length,1);
        chat.selectChat('demo-1');compare(fixture.view.selectedId,'demo-1');
        search.text='存在しない';compare(chat.filteredChats.length,0);
    }
    function test_japanese_draft_and_send_keys() {
        var composer=findChild(chat,'composer');var editor=findChild(chat,'messageInput');verify(editor!==null);
        editor.text='じゃあ、明日19時に！';compare(fixture.view.draft,editor.text);
        chat.selectChat('demo-1');chat.selectChat('demo-0');compare(editor.text,'じゃあ、明日19時に！');
        editor.forceActiveFocus();keyClick(Qt.Key_Return,Qt.ShiftModifier);compare(fixture.sends,0);verify(editor.text.indexOf('\n')>=0);
        var intended=editor.text;keyClick(Qt.Key_Return);compare(fixture.sends,1);compare(fixture.sentText,intended);compare(editor.text,'');
        keyClick(Qt.Key_Return);compare(fixture.sends,1);
        var event={key:Qt.Key_Return,modifiers:Qt.NoModifier,accepted:false};
        editor.text='変換中';composer.handleKey(event,true);compare(fixture.sends,1);compare(event.accepted,false);
    }
    function test_settings_close_escape_and_responsive() {
        mouseClick(findChild(chat,'settingsButton'));verify(chat.settingsOpen);
        mouseClick(findChild(chat,'settingsBack'));verify(!chat.settingsOpen);
        mouseClick(findChild(chat,'closeButton'));compare(closed.count,1);
        chat.forceActiveFocus();keyClick(Qt.Key_Escape);compare(closed.count,2);
        testCase.width=720;testCase.height=540;wait(20);
        var composer=findChild(chat,'composer');verify(composer.width>300);verify(composer.height>70);
        testCase.width=1200;testCase.height=760;
    }
}
