import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Rectangle {
    id:root
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
            ActionButton {objectName:'settingsBack';text:'‹';hint:'トークへ戻る';onClicked:root.back()}
            Text {text:'設定';textFormat:Text.PlainText;color:ink.foreground;font.family:ink.family;font.pixelSize:24;Layout.fillWidth:true}
            ActionButton {text:'×';hint:'閉じる';onClicked:root.dismiss()}
        }
        LoginView {Layout.fillWidth:true;Layout.fillHeight:true;visible:root.loginVisible;service:root.service}
        ColumnLayout {
            Layout.fillWidth:true;Layout.fillHeight:true;spacing:18;visible:!root.loginVisible
            Item {Layout.fillHeight:true}
            Text {
                Layout.fillWidth:true;text:root.service ? root.service.view.statusText || '' : 'サービスを準備しています…'
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.foreground;font.family:ink.family;font.pixelSize:ink.body
            }
            Text {
                Layout.fillWidth:true;text:'スマートフォンのLINEで、この画面に表示するQRコードを読み取ります。LINEのChrome版など、同じ方式のクライアントがログアウトする場合があります。'
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.family:ink.family;font.pixelSize:ink.body
            }
            RowLayout {
                spacing:12
                ActionButton {objectName:'qrLoginButton';text:'QRコードでログイン';accent:true;enabled:!!root.service && root.service.started!==false && root.service.view.mode==='live' && !root.service.view.busy;onClicked:root.service.login()}
                ActionButton {text:'再読み込み';enabled:!!root.service && !root.service.view.busy;onClicked:root.service.refresh()}
            }
            Text {
                Layout.fillWidth:true;text:'非公式クライアントです。導入手順と対応版はREADMEをご覧ください。パスワードの入力は不要です。'
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.family:ink.family;font.pixelSize:13
            }
            ActionButton {objectName:'notificationToggle';text:root.service && root.service.view.notifications===false?'新着通知: オフ':'新着通知: オン';onClicked:if(root.service && typeof root.service.notifications==='function')root.service.notifications(root.service.view.notifications===false)}
            Text {Layout.fillWidth:true;text:root.service && root.service.view.notificationNote || '通知に名前・本文は表示しません。設定はこの起動中だけ保持します。';textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.pixelSize:12}
            Item {Layout.fillHeight:true}
        }
    }
}
