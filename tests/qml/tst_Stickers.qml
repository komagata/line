import QtQuick
import QtTest
import '../../components'
import '../fixtures/fictional-avatars.js' as Photos
TestCase {
 id:tests;name:'Stickers';when:windowShown;visible:true;width:1200;height:760
 property var calls:[]
 QtObject {
  id:fixture;property var view:tests.initial()
  function feature(action,extra,context){tests.calls.push({action:action,extra:extra,context:context})}
  function editDraft(text){var n=Object.assign({},view);n.draft=text;view=n}
  function send(text){tests.calls.push({action:'send',text:text})}
  function cancelLogin(){}
 }
 function initial(){return {session:'fixture',selection:1,mode:'demo',status:'ready',chats:[{id:'demo-0',name:'山田 太郎',preview:'スタンプ',unread:0,time:'19:40',group:false}],selectedId:'demo-0',account:{id:'demo-self',name:'自分'},avatars:{},stickerImages:{'1001':Photos.sage},draft:'',sendStatus:'idle',historyStatus:'ready',stickers:{open:false,status:'ready',products:[{id:'1',name:'架空のカラースタンプ',supported:true,count:48,poster:{id:'1001',hash:''}}],packageId:'1',page:0,pages:2,items:[{id:'1001',packageId:'1',alt:'架空のスタンプ'}],selected:null,note:'静止画・音声なし'},messages:[{id:'101',text:'［スタンプ］',contentType:7,sticker:{id:'1001',hash:'',alt:'架空のスタンプ'},own:false,senderId:'demo-0',sender:'山田 太郎',time:'19:40',day:'今日'}]}}
 ChatView{id:chat;anchors.fill:parent;service:fixture}
 function init(){tests.width=1200;tests.height=760;chat.appVisible=true;fixture.view=initial();chat.settingsOpen=false;chat.closeTransient(false);tests.calls=[];wait(40)}
 function update(values){fixture.view=Object.assign({},fixture.view,values);wait(20)}
 function test_receive_and_metadata_refresh(){var image=findChild(chat,'receivedSticker-101');verify(image!==null);compare(image.source.toString(),Photos.sage);compare(image.fillMode,Image.PreserveAspectFit);verify(image.width<=200);var n=initial();n.messages[0].sticker.id='1002';n.stickerImages={'1002':Photos.sand};fixture.view=n;wait(30);compare(findChild(chat,'receivedSticker-101').source.toString(),Photos.sand)}
 function test_open_choose_stage_explicit_send_preserves_draft(){
  mouseClick(findChild(chat,'stickerButton'));compare(tests.calls[0].action,'sticker-open');compare(tests.calls.filter(c=>c.action==='send'||c.action==='sticker-send').length,0);
  var v=initial();v.stickers.open=true;fixture.view=v;wait(30);verify(findChild(chat,'stickerPicker').visible);mouseClick(findChild(chat,'stickerGrid').itemAtIndex(0));compare(tests.calls[tests.calls.length-1].action,'sticker-choose');
  v=initial();v.stickers.selected={id:'1001',packageId:'1',alt:'架空のスタンプ'};v.draft='下書きを保存';fixture.view=v;wait(30);verify(!findChild(chat,'sendButton').enabled);compare(findChild(chat,'messageInput').text,'下書きを保存');
  update({draft:''});verify(findChild(chat,'sendButton').enabled);mouseClick(findChild(chat,'sendButton'));compare(tests.calls[tests.calls.length-1].action,'sticker-send');
 }
 function test_reject_remote_image_and_show_fallback(){var n=initial();n.stickerImages={'1001':'https://example.org/evil.png'};fixture.view=n;wait(20);compare(findChild(chat,'receivedSticker-101').source.toString(),'');verify(findChild(chat,'stickerFallback-101').visible)}
 function test_picker_close_and_window_hide(){var n=initial();n.stickers.open=true;fixture.view=n;wait(30);mouseClick(findChild(chat,'stickerPickerClose'));compare(tests.calls[tests.calls.length-1].action,'sticker-close');tests.calls=[];chat.appVisible=false;compare(tests.calls.filter(c=>c.action==='sticker-hide').length,1)}
 function test_escape_closes_picker_without_sending(){var n=initial();n.stickers.open=true;fixture.view=n;wait(40);keyClick(Qt.Key_Escape);compare(tests.calls[tests.calls.length-1].action,'sticker-close');compare(tests.calls.filter(c=>c.action==='sticker-send').length,0)}
 function test_sticker_compact_wide_screenshots(){
  var n=initial();n.stickers.open=true;n.stickers.items=[];
  for(var i=0;i<40;i++){var id=String(1001+i);n.stickers.items.push({id:id,packageId:'1',alt:'架空のカラースタンプ'});n.stickerImages[id]=i%2?Photos.sand:Photos.sage}
  fixture.view=n;
  for(var size of [[720,540,'compact'],[1200,760,'wide']]){tests.width=size[0];tests.height=size[1];wait(80);grabImage(chat).save('/tmp/omarchy-line-stickers-picker-'+size[2]+'.png')}
  n=initial();n.stickers.selected={id:'1001',packageId:'1',alt:'架空のスタンプ'};fixture.view=n;
  for(var size of [[720,540,'compact'],[1200,760,'wide']]){tests.width=size[0];tests.height=size[1];wait(80);grabImage(chat).save('/tmp/omarchy-line-stickers-stage-'+size[2]+'.png')}
 }
}
