import QtQuick
import QtQuick.Window
import QtTest
import QtQuick.Dialogs as NativeDialogs
import '../../components'
import '../fixtures/fictional-avatars.js' as Photos
TestCase {
 id:tests;name:'EverydayFeatures';when:windowShown;visible:true;width:1200;height:760
 property var calls:[]
 QtObject {
  id:fixture
  property var view:tests.initial()
  function editDraft(text){var n=Object.assign({},view);n.draft=text;view=n}
  function feature(action,extra,context){tests.calls.push({action:action,extra:extra,context:context})}
  function send(text){tests.calls.push({action:'send',text:text})}
  function selectChat(id){var n=Object.assign({},view);n.selectedId=id;n.selection++;view=n}
  function cancelLogin(){}
 }
 function initial(){return {session:'fixture',selection:1,mode:'demo',status:'ready',chats:[{id:'demo-0',name:'山田 太郎',preview:'カフェで会いましょう',unread:0,time:'19:40',group:false}],selectedId:'demo-0',account:{id:'demo-self',name:'自分'},avatars:{},draft:'入力途中',sendStatus:'idle',historyStatus:'ready',reply:null,attachment:null,messages:[{id:'101',text:'カフェの写真 https://example.org/cafe',own:false,senderId:'demo-0',sender:'山田 太郎',time:'19:40',day:'今日',contentType:1,downloadable:true,fileName:'cafe.png',reactions:[{name:'love',count:2,own:true}]},{id:'102',text:'19時に行きます',own:true,senderId:'demo-self',time:'19:41',day:'今日',replyTo:'101',reactions:[]}]}}
 ChatView{id:chat;anchors.fill:parent;service:fixture}
 SignalSpy{id:dismissed;target:chat;signalName:'dismiss'}
 function init(){chat.appVisible=true;chat.visible=true;chat.windowFocused=false;fixture.view=initial();chat.settingsOpen=false;chat.closeTransient(false);tests.calls=[];dismissed.clear();tests.Window.window.requestActivate();chat.forceActiveFocus();wait(30)}
 function test_local_search_navigation_preserves_draft(){
  chat.openMessageSearch();var field=findChild(chat,'messageSearch');verify(field!==null);field.text='カフェ';compare(chat.searchMatches.length,1);compare(chat.searchMatches[0],0);compare(fixture.view.draft,'入力途中');
  field.text='該当なし';compare(chat.searchMatches.length,0);keyClick(Qt.Key_Escape);verify(!chat.messageSearchOpen);compare(dismissed.count,0);
 }
 function test_context_menu_copy_reply_and_stale_file_dialog(){
  chat.openActions(fixture.view.messages[0]);compare(chat.actionContext.messageId,'101');chat.chooseAction('copy');compare(chat.copiedText,fixture.view.messages[0].text);
  chat.openActions(fixture.view.messages[0]);chat.chooseAction('reply');compare(tests.calls[0].action,'reply');compare(tests.calls[0].context.selection,1);
  chat.attachmentContext=chat.capture();fixture.selectChat('demo-other');chat.acceptAttachment('file:///tmp/fictional.png');compare(tests.calls.filter(c=>c.action==='attach').length,0);
 }
 function test_unsend_requires_confirmation_and_immutable_target(){
  chat.openActions(fixture.view.messages[1]);chat.chooseAction('unsend');compare(tests.calls.length,0);verify(chat.unsendVisible);chat.confirmUnsend();compare(tests.calls[0].action,'unsend');compare(tests.calls[0].extra.confirmed,true);compare(tests.calls[0].context.messageId,'102');
 }
 function test_focus_requires_actual_active_window_and_search_escape_first(){
  chat.windowFocused=false;tests.calls=[];chat.windowFocused=true;compare(tests.calls[tests.calls.length-1].action,'focus');compare(tests.calls[tests.calls.length-1].extra.focused,true);
  chat.settingsOpen=true;compare(tests.calls[tests.calls.length-1].extra.focused,false);chat.settingsOpen=false;chat.windowFocused=false;
 }
 function test_only_explicit_http_links_and_literal_selectable_copy(){
  compare(chat.links('javascript:alert(1) file:///tmp/a https://example.org/a').length,1);
  var text=findChild(chat,'messageText-101');verify(text!==null);compare(text.getText(0,text.length),fixture.view.messages[0].text);text.selectAll();compare(text.selectedText,fixture.view.messages[0].text);verify(text.selectByMouse);
  compare(chat.links('HTTPS://example.org/道?a=1&b=2。 https://user:pw@example.org'),['HTTPS://example.org/道?a=1&b=2']);
 }
 function test_inline_links_compact_wide_screenshots(){
  var n=initial();n.messages=[
   {id:'101',text:'明日の待ち合わせです。\nhttps://example.org/cafe?day=1&time=19\n「HTTPS://example.org/道案内」も見てください。',own:false,senderId:'demo-0',sender:'山田 太郎',time:'19:40',day:'今日'},
   {id:'102',text:'ありがとう！ https://example.org/a_(b)\nhttps://example.org/very-long-fictional-path-to-the-cafe-and-station?day=1&time=19',own:true,senderId:'demo-self',time:'19:41',day:'今日'}
  ];fixture.view=n;
  for(var size of [[720,540,'compact'],[1200,760,'wide']]){
   tests.width=size[0];tests.height=size[1];wait(80);
   var body=findChild(chat,'messageText-102');verify(body!==null);verify(body.contentWidth<=body.width+1);
   grabImage(chat).save('/tmp/omarchy-line-links-'+size[2]+'.png');
  }
 }
 function test_native_save_acceptance_keeps_original_target_and_cancel_has_no_action(){
  chat.saveContext=chat.capture('101');chat.acceptDownload('file:///tmp/fixture-output');compare(tests.calls[0].action,'download');compare(tests.calls[0].context.messageId,'101');
  chat.saveContext=chat.capture('102');fixture.selectChat('demo-other');chat.acceptDownload('file:///tmp/should-not-save');compare(tests.calls.filter(c=>c.action==='download').length,1);
 }
 function dialogCases(){return [
  {tag:'attach',name:'attachDialog',context:'attachmentContext',action:'attach',mode:NativeDialogs.FileDialog.OpenFile,url:Qt.resolvedUrl('../fixtures/fictional-avatars.js')},
  {tag:'save',name:'saveDialog',context:'saveContext',action:'download',mode:NativeDialogs.FileDialog.SaveFile,url:Qt.resolvedUrl('../fixtures/fictional-dialog-output.png')}
 ]}
 function openFileDialog(data){
  var dialog=findChild(chat,data.name);verify(dialog!==null);
  // Confine the picker to fictional test files, never the user's home folder.
  dialog.currentFolder=Qt.resolvedUrl('../fixtures/');
  if(data.action==='attach')chat.chooseAttachment();
  else {chat.openActions(fixture.view.messages[0]);chat.chooseAction('download')}
  tryCompare(dialog,'visible',true);return dialog;
 }
 function test_file_dialog_uses_quick_before_open_data(){return dialogCases()}
 function test_file_dialog_uses_quick_before_open(data){
  var dialog=findChild(chat,data.name);verify(dialog!==null);verify(!dialog.visible);
  verify((dialog.options & NativeDialogs.FileDialog.DontUseNativeDialog)!==0,'File picker must bypass the host native GTK implementation before opening');
  compare(dialog.fileMode,data.mode);
 }
 function test_file_dialog_open_cancel_and_selected_file_data(){return dialogCases()}
 function test_file_dialog_open_cancel_and_selected_file(data){
  var dialog=openFileDialog(data);compare(tests.calls.length,0);
  dialog.selectedFile=data.url;dialog.reject();tryCompare(dialog,'visible',false);
  compare(chat[data.context],null);compare(tests.calls.length,0);compare(fixture.view.draft,'入力途中');
  dialog=openFileDialog(data);dialog.selectedFile=data.url;dialog.accept();tryCompare(dialog,'visible',false);
  compare(chat[data.context],null);compare(tests.calls.length,1);compare(tests.calls[0].action,data.action);
  compare(tests.calls[0].extra.url,String(data.url));compare(tests.calls[0].context.id,'demo-0');
  compare(tests.calls[0].context.session,'fixture');compare(tests.calls[0].context.selection,1);
  if(data.action==='download')compare(tests.calls[0].context.messageId,'101');
  compare(tests.calls.filter(c=>c.action==='send').length,0);compare(fixture.view.draft,'入力途中');
 }
 function test_file_dialog_late_accept_after_context_change_data(){
  var rows=[];for(var data of dialogCases())for(var boundary of ['session','selection'])rows.push(Object.assign({},data,{tag:data.tag+'-'+boundary,boundary:boundary}));return rows;
 }
 function test_file_dialog_late_accept_after_context_change(data){
  var dialog=openFileDialog(data);dialog.selectedFile=data.url;
  var stale=chat[data.context],n=Object.assign({},fixture.view);
  if(data.boundary==='session')n.session='next-session';else n.selection++;
  fixture.view=n;tryCompare(dialog,'visible',false);compare(chat[data.context],null);
  tests.calls=[];
  // Simulate an already-queued acceptance with old authority after dismissal.
  chat[data.context]=stale;dialog.accepted();
  compare(chat[data.context],null);compare(tests.calls.length,0);compare(fixture.view.draft,'入力途中');
 }
 function test_preview_bounded_source_and_explicit_close(){
  var n=initial();n.preview='https://example.org/untrusted';fixture.view=n;var image=findChild(chat,'attachmentPreviewImage');verify(image!==null);compare(String(image.source),'');
  n=initial();fixture.view=n;
 }
 function test_attachment_and_reply_bars_and_compact_wide_screenshots(){
  var n=initial();n.reply={id:'101',text:'カフェの写真',sender:'山田 太郎'};n.attachment={id:'fixture-file',name:'待ち合わせ.png',size:12048};n.draft='';fixture.view=n;
  for(var size of [[720,540,'compact'],[1200,760,'wide']]){tests.width=size[0];tests.height=size[1];wait(80);verify(findChild(chat,'attachmentSummary').visible);verify(findChild(chat,'replySummary').visible);verify(findChild(chat,'sendButton').enabled);var shot=grabImage(chat);compare(shot.width,size[0]);shot.save('/tmp/omarchy-line-features-'+size[2]+'.png');}
 }
 function test_contacts_entry_search_cancel_and_empty_selection(){
  var n=initial();n.contacts=[{id:'demo-new',name:'高橋 葵'},{id:'demo-0',name:'山田 太郎'}];n.contactsStatus='ready';n.selectedId='';fixture.view=n;
  var entry=findChild(chat,'newChatButton');verify(entry!==null);mouseClick(entry);verify(chat.contactsVisible);
  var field=findChild(chat,'contactSearch');field.text='高橋';compare(chat.filteredContacts.length,1);keyClick(Qt.Key_Escape);verify(!chat.contactsVisible);compare(tests.calls.filter(c=>c.action==='start-chat').length,0);
  mouseClick(entry);wait(20);field.text='';field.text='高橋';verify(chat.canStartChat);verify(chat.current(chat.contactContext));compare(chat.filteredContacts.length,1);field.forceActiveFocus();keyClick(Qt.Key_Return);compare(tests.calls.filter(c=>c.action==='start-chat').length,1);compare(tests.calls[tests.calls.length-1].context.id,'');compare(tests.calls[tests.calls.length-1].extra.contactId,'demo-new');
 }
 function test_dismiss_closes_transients_and_cancels_download_preserving_composition(){
  var n=initial();n.fileStatus='pending';n.preview='https://example.invalid/untrusted';n.attachment={id:'kept',name:'keep.txt',size:2};fixture.view=n;
  chat.openActions(n.messages[1]);chat.chooseAction('unsend');chat.openMessageSearch();chat.attachmentContext=chat.capture();chat.saveContext=chat.capture('101');tests.calls=[];
  chat.dismiss();verify(!chat.unsendVisible);verify(!chat.messageSearchOpen);compare(chat.attachmentContext,null);compare(chat.saveContext,null);compare(tests.calls.filter(c=>c.action==='file-cancel').length,1);compare(tests.calls.filter(c=>c.action==='unsend').length,0);compare(fixture.view.draft,'入力途中');compare(fixture.view.attachment.id,'kept');verify(!findChild(chat,'previewDialog').visible);
 }

 function test_contact_picker_stale_context_and_keyboard_tab(){
  var n=initial();n.contacts=[{id:'demo-new',name:'高橋 葵'}];n.contactsStatus='ready';fixture.view=n;
  chat.openContacts();wait(10);var field=findChild(chat,'contactSearch');field.forceActiveFocus();keyClick(Qt.Key_Tab);verify(!field.activeFocus);keyClick(Qt.Key_Return);compare(tests.calls.filter(c=>c.action==='start-chat').length,1);
  chat.openContacts();var stale=chat.contactContext;fixture.selectChat('demo-other');verify(!chat.contactsVisible);chat.contactContext=stale;chat.chooseContact('demo-new');compare(tests.calls.filter(c=>c.action==='start-chat').length,1);
 }
 function test_visibility_loss_suppresses_late_preview_until_explicit_new_request(){
  var n=initial();n.preview=Photos.sage;n.fileStatus='pending';fixture.view=n;chat.previewSuppressed=false;wait(10);verify(findChild(chat,'previewDialog').visible);
  chat.appVisible=false;verify(!findChild(chat,'previewDialog').visible);compare(tests.calls.filter(c=>c.action==='file-cancel').length,1);
  fixture.view=Object.assign({},n);chat.appVisible=true;wait(10);verify(!findChild(chat,'previewDialog').visible);compare(fixture.view.draft,'入力途中');
  chat.perform('preview',{},chat.capture('101'));verify(findChild(chat,'previewDialog').visible);
  tests.calls=[];chat.visible=false;verify(!findChild(chat,'previewDialog').visible);compare(tests.calls.filter(c=>c.action==='file-cancel').length,1);chat.visible=true;
 }
 Component {id:temporaryChat;ChatView{width:720;height:540}}
 function test_component_destruction_cancels_captured_read_without_writing(){
  var item=createTemporaryObject(temporaryChat,tests,{service:fixture});verify(item!==null);item.perform('preview',{},item.capture('101'));tests.calls=[];item.destroy();wait(10);
  compare(tests.calls.filter(c=>c.action==='file-cancel').length,1);compare(tests.calls[0].context.messageId,'101');compare(tests.calls.filter(c=>c.action==='send'||c.action==='unsend'||c.action==='react').length,0);
 }
 function test_contact_status_and_compact_wide_screenshots(){
  var n=initial();n.contacts=[{id:'demo-0',name:'山田 太郎'},{id:'demo-new',name:'高橋 葵'},{id:'demo-new-2',name:'田中 直人'}];n.contactsStatus='ready';n.avatars={'demo-0':Photos.sage,'demo-new':Photos.blue,'demo-self':Photos.sand};fixture.view=n;
  for(var size of [[720,540,'compact'],[1200,760,'wide']]){
   tests.width=size[0];tests.height=size[1];chat.openContacts();wait(80);verify(chat.contactsVisible);verify(findChild(chat,'contactCancel').visible);var avatar=findChild(findChild(chat,'contactList').itemAtIndex(1),'contactAvatar-demo-new');verify(avatar!==null);tryCompare(avatar,'imageReady',true);var point=avatar.mapToItem(tests,20,20);tryVerify(function(){return !Qt.colorEqual(grabImage(tests).pixel(Math.round(point.x),Math.round(point.y)),avatar.color)},1000,'contact photo must paint on first popup open');grabImage(tests).save('/tmp/omarchy-line-contacts-'+size[2]+'.png');
   chat.chooseContact('demo-new');var conversation=Object.assign({},n);conversation.chats=n.chats.concat([{id:'demo-new',name:'高橋 葵',preview:'まだメッセージはありません',unread:0,time:'',group:false}]);conversation.selectedId='demo-new';conversation.messages=[];conversation.draft='';conversation.selection=2;fixture.view=conversation;wait(60);grabImage(chat).save('/tmp/omarchy-line-contact-conversation-'+size[2]+'.png');fixture.view=n;
  }
  for(var status of ['loading','error','idle']){n=Object.assign({},n,{contactsStatus:status});fixture.view=n;chat.openContacts();verify(!chat.canStartChat);chat.chooseContact('demo-new');verify(chat.contactsVisible);chat.closeTransient(false)}
 }

}
