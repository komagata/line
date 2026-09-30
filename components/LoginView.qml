import QtQuick
import QtQuick.Layouts

Item {
    id: root
    property var service: null
    readonly property var login: service && service.view.login ? service.view.login : ({stage:'idle',image:'',pin:'',statusText:'',active:false})
    LinePalette {id:ink}
    RowLayout {
        anchors.fill:parent;spacing:24
        Rectangle {
            visible:root.login.stage==='scan'
            Layout.preferredWidth:264;Layout.preferredHeight:264
            color:'#ffffff';radius:ink.radius
            Image {
                id:qr;objectName:'loginQr'
                anchors.fill:parent
                // Locally generated PNG only; never a file/remote URL.
                source:typeof root.login.image==='string' && root.login.image.length<=87406 && /^data:image\/png;base64,[A-Za-z0-9+/]+={0,2}$/.test(root.login.image) ? root.login.image : ''
                cache:false;smooth:false;fillMode:Image.PreserveAspectFit
                Accessible.name:'スマートフォンのLINEで読み取るログイン用QRコード'
            }
        }
        ColumnLayout {
            Layout.fillWidth:true;Layout.alignment:Qt.AlignVCenter;spacing:18
            Text {
                Layout.fillWidth:true;text:root.login.stage==='replace-confirmation'?'保存済みセッションの確認':'QRコードでログイン'
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.foreground;font.family:ink.family;font.pixelSize:22
            }
            Text {
                Layout.fillWidth:true;text:root.login.statusText||''
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.foreground;font.family:ink.family;font.pixelSize:ink.body
            }
            Text {
                objectName:'loginPin';Layout.fillWidth:true
                visible:root.login.stage==='phone' && text!==''
                text:root.login.stage==='phone' ? root.login.pin||'' : ''
                textFormat:Text.PlainText;color:ink.accent;font.family:ink.family;font.pixelSize:38;font.letterSpacing:5
            }
            Text {
                Layout.fillWidth:true;visible:root.login.stage==='phone' && !!root.login.pin
                text:'この番号をスマートフォンのLINEに入力してください'
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.family:ink.family;font.pixelSize:13
            }
            Text {
                Layout.fillWidth:true;visible:root.login.stage==='replace-confirmation'
                text:'続けると、LINEのChrome版や同じ方式のクライアントがログアウトする場合があります。'
                textFormat:Text.PlainText;wrapMode:Text.Wrap;color:ink.muted;font.family:ink.family;font.pixelSize:13
            }
            Flow {
                Layout.fillWidth:true;spacing:8
                ActionButton {objectName:'loginCancel';visible:root.login.canCancel===true;text:'キャンセル';onClicked:root.service.cancelLogin()}
                ActionButton {objectName:'loginConfirm';visible:root.login.canConfirm===true;text:'置き換えて続ける';accent:true;onClicked:root.service.confirmLogin()}
                ActionButton {objectName:'loginRetry';visible:root.login.canRetry===true;text:'QRコードを再作成';accent:true;onClicked:root.service.retryLogin()}
                ActionButton {visible:root.login.stage==='uncertain' || root.login.stage==='success';enabled:!!root.service && !root.service.view.busy;text:'再読み込み';onClicked:root.service.refresh()}
            }
        }
    }
}
