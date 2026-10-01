import QtQuick
import QtTest
import QtCore
import '__ROOT__' as Plugin
import '__ROOT__/components'
import 'Fixture.js' as Fixture
TestCase {
 id:tests;name:'EnglishPreview';when:windowShown;visible:true;width:1200;height:760
 Plugin.Service {id:service;settingsLocation:StandardPaths.writableLocation(StandardPaths.ConfigLocation)+'/preview.ini'}
 ChatView {id:chat;anchors.fill:parent;service:service}
 function snapshot(path) {
  var done=false, saved=false
  verify(chat.grabToImage(function(result){saved=result.saveToFile(path);done=true}))
  tryVerify(function(){return done},5000);verify(saved)
 }
 function test_capture_actual_chat_and_settings() {
  service.setMode('demo');service.setLanguage('ja')
  service.consume(JSON.stringify({type:'state',view:Fixture.states.ja})+'\n')
  chat.settingsOpen=true
  var selector=findChild(chat,'languageSelector');verify(selector!==null)
  compare(selector.currentIndex,selector.model.indexOf('日本語'));var englishIndex=selector.model.indexOf('English');verify(englishIndex>=0);selector.currentIndex=englishIndex;selector.activated(englishIndex)
  compare(service.language,'en');compare(selector.currentIndex,englishIndex)
  service.consume(JSON.stringify({type:'state',view:Fixture.states.en})+'\n')
  compare(findChild(chat,'chatSearch').placeholderText,'Search chats')
  for(var size of [{width:1200,height:760},{width:720,height:540}]) {
   tests.width=size.width;tests.height=size.height
   chat.settingsOpen=true;waitForRendering(chat)
   snapshot('__OUTPUT__/settings-'+size.width+'x'+size.height+'.png')
   compare(selector.model[0],'System default')
   verify(selector.mapToItem(chat,selector.width,0).x<=chat.width)
   service.systemLocale='en_US';selector.currentIndex=0;selector.activated(0)
   compare(service.savedLanguage,'');compare(selector.currentIndex,0);waitForRendering(chat)
   snapshot('__OUTPUT__/settings-system-default-'+size.width+'x'+size.height+'.png')
   service.setLanguage('en');compare(selector.currentIndex,englishIndex)
   chat.settingsOpen=false;waitForRendering(chat)
   tryVerify(function(){return findChild(chat,'chatAvatar-demo-0').imageReady && findChild(chat,'selfAvatar').imageReady && findChild(chat,'headerAvatar').imageReady},5000)
   var composer=findChild(chat,'composer');verify(composer.width>300);verify(composer.y+composer.height<=chat.height)
   tryVerify(function(){return findChild(chat,'messageText-105')!==null})
   var list=findChild(chat,'messageList')
   tryVerify(function(){list.forceLayout();list.positionViewAtEnd();var last=findChild(chat,'messageText-105');return last && last.mapToItem(chat,0,last.height).y<=composer.y && last.mapToItem(chat,0,0).y>=list.mapToItem(chat,0,0).y},5000)
   list.positionViewAtBeginning()
   tryVerify(function(){var first=findChild(chat,'messageText-101');return first && first.mapToItem(chat,0,0).y>=list.mapToItem(chat,0,0).y},5000)
   waitForRendering(chat)
   snapshot('__OUTPUT__/chat-'+size.width+'x'+size.height+'.png')
  }
  compare(findChild(service,'bridgeProcess').writes.length,0)
 }
}
