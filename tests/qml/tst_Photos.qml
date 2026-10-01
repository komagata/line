import QtQuick
import QtTest
import '../../components'
import '../fixtures/fictional-avatars.js' as Photos
TestCase {
 id:tests;name:'Photos';when:windowShown;visible:true;width:1200;height:760
 property var calls:[]
 QtObject { id:fixture;property string language:'en';property var view:tests.initial();function feature(action,extra,context){tests.calls.push({action:action,extra:extra,context:context})}function cancelLogin(){} }
 function initial(){return {session:'fixture',selection:1,mode:'demo',status:'ready',chats:[{id:'demo-0',name:'Fictional chat',preview:'Fictional preview',time:'12:00',unread:0,group:false}],selectedId:'demo-0',account:{name:'Me'},draft:'',messages:[{id:'101',text:'Fictional caption https://example.org',contentType:1,downloadable:true,own:false,sender:'Fictional sender',day:'Today'}],photoThumbnails:{'101':{status:'ready',data:Photos.sage}},historyStatus:'ready',sendStatus:'idle'}}
 ChatView {id:chat;anchors.fill:parent;service:fixture}
 function init(){tests.width=1200;tests.height=760;fixture.language='en';chat.settingsOpen=false;chat.appVisible=true;fixture.view=initial();tests.calls=[];wait(50)}
 function test_inline_photo_ready_and_preserves_caption(){var img=findChild(chat,'receivedPhoto-101');verify(img!==null);tryCompare(img,'status',Image.Ready);compare(img.fillMode,Image.PreserveAspectFit);compare(findChild(chat,'messageText-101').getText(0,1000),'Fictional caption https://example.org')}
 function test_visible_requests_and_hide(){chat.appVisible=false;wait(50);chat.appVisible=true;tryVerify(function(){return calls.some(c=>c.action==='photos-visible' && c.extra.messageIds.indexOf('101')>=0)});chat.appVisible=false;verify(calls.some(c=>c.action==='photos-hide'))}
 function test_status_transition_reissues_visible_photo_request(){var v=initial();v.status='checking';v.photoThumbnails={};fixture.view=v;wait(100);var before=calls.filter(c=>c.action==='photos-visible').length;v=Object.assign({},v,{status:'ready'});fixture.view=v;tryVerify(function(){return calls.filter(c=>c.action==='photos-visible').length>before})}
 function test_failure_and_retry_localized(){var v=initial();v.photoThumbnails={'101':{status:'error',data:''}};fixture.view=v;wait(30);var fallback=findChild(chat,'photoFallback-101');verify(fallback.visible);compare(fallback.text,'Photo unavailable. Retry or save from the message menu');mouseClick(findChild(chat,'photoRetry-101'));compare(calls[calls.length-1].action,'photo-retry');fixture.language='ja';wait(20);compare(fallback.text,'写真を表示できません。再試行するか、メニューから保存してください')}
 function test_remote_and_file_urls_rejected(){for(var value of ['https://example.org/private.png','file:///tmp/private.png','data:image/svg+xml;base64,AAAA']){var v=initial();v.photoThumbnails['101'].data=value;fixture.view=v;wait(20);compare(findChild(chat,'receivedPhoto-101').source.toString(),'')}}
 function test_only_visible_history_rows_are_requested() {
  var v=initial();v.messages=[];v.photoThumbnails={}
  for(var i=100;i<200;i++)v.messages.push({id:String(i),text:'Fictional caption',contentType:1,downloadable:true,own:false,sender:'Fictional sender',day:'Today'})
  fixture.view=v;wait(150)
  var list=findChild(chat,'messageList');chat.followBottom=false;list.positionViewAtBeginning();wait(100)
  tests.calls=[];chat.photoRequestKey='';chat.requestVisiblePhotos()
  var requested=calls.filter(c=>c.action==='photos-visible');compare(requested.length,1)
  verify(requested[0].extra.messageIds.length>0 && requested[0].extra.messageIds.length<=8)
  verify(requested[0].extra.messageIds.length<100)
  for(var id of requested[0].extra.messageIds){var img=findChild(chat,'receivedPhoto-'+id);verify(img!==null);var point=img.mapToItem(list,0,0);verify(point.y<list.height && point.y+img.height>0)}
  chat.requestVisiblePhotos();compare(calls.filter(c=>c.action==='photos-visible').length,1)
  list.positionViewAtEnd();tryVerify(function(){return calls.some(c=>c.action==='photos-visible' && c.extra.messageIds.indexOf('199')>=0)},1000)
 }
 function test_missing_sticker_metadata_has_readable_fallback(){var v=initial();v.messages=[{id:'101',contentType:7,text:'[Sticker]',sticker:null,own:false,sender:'Fictional sender',day:'Today'}];fixture.view=v;wait(30);var text=findChild(chat,'messageText-101');verify(text!==null);verify(text.parent.children.some(c=>c.text==='Sticker image unavailable (loading or unsupported)' && c.visible))}
 function test_layout_and_click_preview(){for(var size of [[720,540],[1200,760]]){tests.width=size[0];tests.height=size[1];wait(30);var img=findChild(chat,'receivedPhoto-101');verify(img.width<=240);verify(img.height<=220);verify(img.width>0);mouseClick(img);compare(calls[calls.length-1].action,'preview')}}
}
