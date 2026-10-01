import QtQuick
import 'Locale.js' as Locale
import QtQuick.Controls
import QtQuick.Layouts

Rectangle {
    id:root
    readonly property string language: service && service.language === 'en' ? 'en' : 'ja'
    function tr(value) { return Locale.text(language,value) }
    property var service:null
    readonly property var login:service && service.view.login ? service.view.login : ({stage:'idle',active:false})
    readonly property bool loginVisible:!!service && service.view.mode==='live' && login.stage!=='idle' && (login.stage!=='success' || service.view.status!=='ready')
    signal back()
    signal dismiss()
    LinePalette {id:ink}
    color:ink.surface
    ColumnLayout {
        anchors.fill:parent;anchors.margins:28;spacing:16
        RowLayout {
            Layout.fillWidth:true
            ActionButton {objectName:'settingsBack';text:'‹';hint:root.tr('トークへ戻る');onClicked:root.back()}
            Text {text:root.tr('設定');textFormat:Text.PlainText;color:ink.foreground;font.family:ink.family;font.pixelSize:24;Layout.fillWidth:true}
            ActionButton {text:'×';hint:root.tr('閉じる');onClicked:root.dismiss()}
        }
        RowLayout {
            Layout.fillWidth:true
            Text { text:root.language==='en'?'Language':'言語';textFormat:Text.PlainText;color:ink.foreground;font.family:ink.family;font.pixelSize:ink.body }
            ComboBox {
                objectName:'languageSelector';model:[root.language==='en'?'System default':'システム設定','日本語','English']
                readonly property int preferenceIndex:root.service && root.service.savedLanguage==='ja'?1:root.service && root.service.savedLanguage==='en'?2:0
                currentIndex:preferenceIndex
                onPreferenceIndexChanged:currentIndex=preferenceIndex
                onModelChanged:currentIndex=preferenceIndex
                onActivated:index=>{if(root.service)root.service.setLanguage(index===1?'ja':index===2?'en':'')}
            }
        }
        LoginView {Layout.fillWidth:true;Layout.fillHeight:true;visible:root.loginVisible;service:root.service}
        ColumnLayout {
            Layout.fillWidth:true;Layout.fillHeight:true;spacing:18;visible:!root.loginVisible
            Item {Layout.fillHeight:true}
            Text {
                Layout.fillWidth:true;text:root.service ? root.service.view.statusText || '' : root.tr('サービスを準備しています…')
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.foreground;font.family:ink.family;font.pixelSize:ink.body
            }
            Text {
                Layout.fillWidth:true;text:root.tr('スマートフォンのLINEで、この画面に表示するQRコードを読み取ります。LINEのChrome版など、同じ方式のクライアントがログアウトする場合があります。')
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.family:ink.family;font.pixelSize:ink.body
            }
            RowLayout {
                spacing:12
                ActionButton {objectName:'qrLoginButton';text:root.tr('QRコードでログイン');accent:true;enabled:!!root.service && root.service.started!==false && root.service.view.mode==='live' && !root.service.view.busy;onClicked:root.service.login()}
                ActionButton {text:root.tr('再読み込み');enabled:!!root.service && !root.service.view.busy;onClicked:root.service.refresh()}
            }
            Text {
                Layout.fillWidth:true;text:root.tr('非公式クライアントです。導入手順と対応版はREADMEをご覧ください。パスワードの入力は不要です。')
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.family:ink.family;font.pixelSize:13
            }
            ActionButton {objectName:'notificationToggle';text:root.service && root.service.view.notifications===false?root.tr('新着通知: オフ'):root.tr('新着通知: オン');onClicked:if(root.service && typeof root.service.notifications==='function')root.service.notifications(root.service.view.notifications===false)}
            Text {Layout.fillWidth:true;text:root.service && root.service.view.notificationNote || root.tr('通知に名前・本文は表示しません。設定はこの起動中だけ保持します。');textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.pixelSize:12}
            Item {Layout.fillHeight:true}
        }
    }
}
