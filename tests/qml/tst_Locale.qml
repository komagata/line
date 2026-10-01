import QtQuick
import QtCore
import QtTest
import '../..' as Plugin
import '../../components'
import '../../components/Locale.js' as Locale
TestCase {
 id: tests; name:'Locale'; when:windowShown; visible:true; width:1200;height:760
 Component {id:serviceComponent; Plugin.Service { settingsLocation:StandardPaths.writableLocation(StandardPaths.ConfigLocation)+'/locale-test.ini' }}
 Component {id:settingsComponent; Settings {property string language:''} }
 Component {id:chatComponent; ChatView {width:tests.width;height:tests.height} }
 function test_reactive_selector_persistence_and_unchanged_draft() {
  var service=createTemporaryObject(serviceComponent,tests);verify(service!==null)
  service.setLanguage('ja')
  service.view={mode:'demo',status:'ready',session:'fixture',selection:1,selectedId:'demo-0',chats:[{id:'demo-0',name:'設定',preview:'',time:'',unread:0,group:false}],messages:[],draft:'設定',account:{name:'自分'},historyStatus:'ready'}
  var chat=createTemporaryObject(chatComponent,tests,{service:service});verify(chat!==null)
  compare(findChild(chat,'chatSearch').placeholderText,'トークを検索')
  chat.settingsOpen=true
  var selector=findChild(chat,'languageSelector');verify(selector!==null)
  selector.currentIndex=2;selector.activated(2)
  compare(service.language,'en');compare(selector.currentIndex,2);compare(findChild(chat,'chatSearch').placeholderText,'Search chats')
  compare(findChild(chat,'messageInput').text,'設定');compare(service.view.chats[0].name,'設定')
  service.setLanguage('xx');compare(service.language,'en')
  chat.destroy();service.destroy();wait(20)
  var recreated=createTemporaryObject(serviceComponent,tests);compare(recreated.language,'en')
  recreated.setLanguage('ja');compare(recreated.language,'ja')
 }
 function test_resolution_data() {
  return [
   {tag:'japanese',locale:'ja_JP',saved:'',expected:'ja'},
   {tag:'japanese-hyphen',locale:'ja-JP',saved:'',expected:'ja'},
   {tag:'english',locale:'en_US',saved:'',expected:'en'},
   {tag:'unsupported',locale:'de_DE',saved:'',expected:'en'},
   {tag:'empty',locale:'',saved:'',expected:'en'},
   {tag:'C',locale:'C',saved:'',expected:'en'},
   {tag:'POSIX',locale:'POSIX',saved:'',expected:'en'},
   {tag:'explicit-ja',locale:'en_US',saved:'ja',expected:'ja'},
   {tag:'explicit-en',locale:'ja_JP',saved:'en',expected:'en'},
   {tag:'invalid-ja',locale:'ja_JP',saved:'xx',expected:'ja'},
   {tag:'invalid-en',locale:'de_DE',saved:'xx',expected:'en'}
  ]
 }
 function test_resolution(data) {
  compare(Locale.resolveLanguage(data.saved,data.locale),data.expected)
  var location=StandardPaths.writableLocation(StandardPaths.ConfigLocation)+'/resolve-'+data.tag+'.ini'
  var settings=createTemporaryObject(settingsComponent,tests,{location:location})
  settings.language=data.saved;settings.sync();settings.destroy();wait(1)
  var service=createTemporaryObject(serviceComponent,tests,{settingsLocation:location,systemLocale:data.locale})
  compare(service.language,data.expected);compare(service.savedLanguage,data.saved)
  compare(service.view.account.name,data.expected==='ja'?'自分':'Me')
  var bridge=findChild(service,'bridgeProcess');bridge.started()
  compare(JSON.parse(bridge.writes[0]).locale,data.expected)
  service.destroy();wait(1)
  var recreated=createTemporaryObject(serviceComponent,tests,{settingsLocation:location,systemLocale:'de_DE'})
  compare(recreated.savedLanguage,data.saved)
  compare(recreated.language,data.saved==='ja'?'ja':'en')
 }
 function test_system_default_after_english_preserves_state_and_retranslates_errors() {
  var service=createTemporaryObject(serviceComponent,tests,{settingsLocation:StandardPaths.writableLocation(StandardPaths.ConfigLocation)+'/system-test.ini',systemLocale:'ja_JP'})
  service.setLanguage('en')
  service.view={mode:'live',status:'ready',session:'fixture',selection:1,selectedId:'demo-0',chats:[{id:'demo-0',name:'設定',preview:'',time:'',unread:0,group:false}],messages:[],draft:'設定',account:{name:'本当の名前'},login:{stage:'pin',active:true,pin:'1234'}}
  service.localStatusKey=''
  var bridge=findChild(service,'bridgeProcess');bridge.started();bridge.writes=[]
  var chat=createTemporaryObject(chatComponent,tests,{service:service});chat.settingsOpen=true
  var selector=findChild(chat,'languageSelector');compare(selector.currentIndex,2)
  compare(selector.model[0],'System default')
  bridge.writes=[]
  selector.currentIndex=0;selector.activated(0)
  compare(service.savedLanguage,'');compare(service.language,'ja');compare(selector.model[0],'システム設定')
  compare(service.view.session,'fixture');compare(service.view.draft,'設定');compare(service.view.chats[0].name,'設定');compare(service.view.account.name,'本当の名前');compare(service.view.login.pin,'1234')
  compare(bridge.writes.length,1);compare(JSON.parse(bridge.writes[0]).text,'ja')
  service.systemLocale='de_DE';compare(service.language,'en');compare(selector.currentIndex,0)
  compare(JSON.parse(bridge.writes[1]).text,'en')
  bridge.exited(1,0)
  compare(service.view.statusText,"LINE service stopped. Check the plugin's bin/line-gui and reload")
  service.systemLocale='ja_JP'
  compare(service.view.statusText,'LINEサービスが停止しました。プラグインの bin/line-gui を確認して再読み込みしてください')
  chat.destroy();service.destroy();wait(1)
  var recreated=createTemporaryObject(serviceComponent,tests,{settingsLocation:StandardPaths.writableLocation(StandardPaths.ConfigLocation)+'/system-test.ini',systemLocale:'en_US'})
  compare(recreated.savedLanguage,'');compare(recreated.language,'en')
 }
 function test_language_keeps_completed_login_settings_and_error_text_reactive() {
  var service=createTemporaryObject(serviceComponent,tests,{settingsLocation:StandardPaths.writableLocation(StandardPaths.ConfigLocation)+'/locale-lifecycle-test.ini'})
  service.setLanguage('ja');service.setMode('demo')
  var state={mode:'demo',session:'same-session',selection:2,status:'ready',selectedId:'demo-0',chats:[{id:'demo-0',name:'名前',preview:'',time:'',unread:0,group:false}],messages:[],draft:'input',account:{name:'自分'},historyStatus:'ready',login:{stage:'success',active:false}}
  service.consume(JSON.stringify({type:'state',view:state})+'\n')
  var chat=createTemporaryObject(chatComponent,tests,{service:service})
  chat.settingsOpen=true;service.setLanguage('en')
  service.consume(JSON.stringify({type:'state',view:state})+'\n')
  verify(chat.settingsOpen);compare(service.view.session,'same-session');compare(service.view.draft,'input')
  service.protocolError();service.setLanguage('ja')
  compare(service.view.statusText,'応答を読み取れませんでした。再読み込みしてください')
  service.setLanguage('en');compare(service.view.statusText,'Could not read the response. Please reload')
  service.setLanguage('ja')
 }
}
