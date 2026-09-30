import QtQuick
import QtTest
import '../../components'
import '../fixtures/fictional-qr.js' as QR

TestCase {
    id: tests
    name: 'NativeLogin'
    when: windowShown
    visible:true;width:720;height:540
    QtObject {
        id: fixture
        property int starts:0
        property int cancels:0
        property int confirms:0
        property var view: ({mode:'live',status:'unauthenticated',statusText:'ログインが必要です',busy:false,login:{stage:'idle',active:false,image:'',pin:'',statusText:''},chats:[],messages:[],selectedId:'',account:{name:'自分'}})
        function login(){starts++}
        function cancelLogin(){cancels++}
        function confirmLogin(){confirms++}
        function retryLogin(){starts++}
        function refresh(){}
        function setMode(mode){}
    }
    ChatView {id:chat;anchors.fill:parent;service:fixture}
    function stage(value, extra) {
        var next=Object.assign({},fixture.view)
        next.login=Object.assign({stage:value,active:true,image:'',pin:'',statusText:'スマートフォンのLINEで読み取ってください',canCancel:true,canConfirm:false,canRetry:false},extra||{})
        fixture.view=next
    }
    function init(){chat.settingsOpen=true;fixture.starts=0;fixture.cancels=0;fixture.confirms=0;stage('idle',{active:false});wait(20)}
    function test_primary_login_is_explicit(){var button=findChild(chat,'qrLoginButton');verify(button!==null);compare(fixture.starts,0);mouseClick(button);compare(fixture.starts,1)}
    function test_qr_and_cancel_fit_compact_window(){
        // Image contains only a deterministic fictional QR, injected by QtTest.
        stage('scan',{image:QR.image});wait(50)
        var qr=findChild(chat,'loginQr');verify(qr!==null);verify(qr.visible);verify(qr.width>=240)
        compare(qr.status,Image.Ready)
        var button=findChild(chat,'loginCancel');verify(button!==null);verify(button.visible)
        var point=button.mapToItem(chat,0,0);verify(point.y+button.height<=chat.height)
        mouseClick(button);compare(fixture.cancels,1)
        grabImage(chat).save('/tmp/omarchy-line-qr-compact.png')
        tests.width=1200;tests.height=760;wait(30)
        grabImage(chat).save('/tmp/omarchy-line-qr-wide.png')
        tests.width=720;tests.height=540
    }
    function test_replacement_requires_button_and_pin_is_plain(){
        stage('replace-confirmation',{canConfirm:true});wait(20)
        compare(fixture.confirms,0);mouseClick(findChild(chat,'loginConfirm'));compare(fixture.confirms,1)
        stage('phone',{pin:'123456'});wait(20)
        var pin=findChild(chat,'loginPin');compare(pin.text,'123456');compare(pin.textFormat,Text.PlainText)
        stage('saving');compare(pin.text,'')
    }
    function test_back_and_escape_cancel_active_login(){
        stage('scan');mouseClick(findChild(chat,'settingsBack'));compare(fixture.cancels,1)
        chat.settingsOpen=true;chat.forceActiveFocus();keyClick(Qt.Key_Escape);compare(fixture.cancels,2)
    }
    function test_success_returns_to_chats_without_cancelling_refresh(){
        stage('success',{active:false});var next=Object.assign({},fixture.view);next.status='ready';fixture.view=next;wait(20)
        verify(!chat.settingsOpen);compare(fixture.cancels,0)
    }
    function test_image_rejects_remote_and_file_sources(){
        for(var value of ['https://example.invalid/secret','file:///tmp/secret']) {
            stage('scan',{image:value});wait(10)
            compare(findChild(chat,'loginQr').source.toString(),'')
        }
    }
}
