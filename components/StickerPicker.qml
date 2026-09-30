pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
Popup {
    id: root
    objectName: 'stickerPicker'
    property var catalog: ({})
    property var images: ({})
    signal action(string action, var extra)
    modal: true; focus: true; padding: 16
    closePolicy: Popup.NoAutoClose
    onOpened: contentItem.forceActiveFocus()
    LinePalette { id: ink }
    background: Rectangle { color:ink.surface;radius:ink.radius;border.color:ink.line }
    function imageFor(s) {var key=s.id+(s.hash?'-'+s.hash:'');return images[key]||''}
    contentItem: Item {
        focus:true
        Keys.onEscapePressed: root.action('sticker-close',{})
        Text {id:title;width:parent.width;text:'所有スタンプ';textFormat:Text.PlainText;color:ink.foreground;font.pixelSize:20}
        ListView {
            id:packs;objectName:'stickerPackages';anchors.top:title.bottom;anchors.topMargin:8;width:parent.width;height:60;orientation:ListView.Horizontal;clip:true;spacing:6
            model:root.catalog.products||[];boundsBehavior:Flickable.StopAtBounds
            ScrollBar.horizontal:ScrollBar{}
            delegate: ActionButton {
                required property var modelData
                width:48;height:48;text:'';accent:modelData.id===root.catalog.packageId
                Accessible.name:modelData.name
                StickerImage {id:packPhoto;anchors.centerIn:parent;width:32;height:32;imageData:parent.modelData.poster?root.imageFor(parent.modelData.poster):''}
                onClicked:root.action('sticker-page',{packageId:modelData.id,page:0})
            }
        }
        Text {
            id:note;anchors.top:packs.bottom;anchors.topMargin:8;width:parent.width;wrapMode:Text.Wrap;text:root.catalog.note||''
            textFormat:Text.PlainText;color:ink.muted;font.pixelSize:12
        }
        ActionButton {id:retry;anchors.top:note.bottom;anchors.topMargin:4;text:'再試行';visible:root.catalog.status==='error';onClicked:root.action('sticker-open',{})}
        GridView {
            id:grid;objectName:'stickerGrid';anchors.top:root.catalog.status==='error'?retry.bottom:note.bottom;anchors.topMargin:8;anchors.bottom:footer.top;anchors.bottomMargin:8;width:parent.width;clip:true
            cellWidth:Math.floor(width/Math.max(1,Math.floor(width/100)));cellHeight:110;model:root.catalog.items||[];boundsBehavior:Flickable.StopAtBounds
            ScrollBar.vertical:ScrollBar{}
            delegate: AbstractButton {
                id:choice;required property var modelData
                objectName:'stickerChoice-'+modelData.id;width:grid.cellWidth;height:grid.cellHeight;focusPolicy:Qt.StrongFocus;hoverEnabled:true
                onClicked:root.action('sticker-choose',{packageId:modelData.packageId,stickerId:modelData.id})
                background:Rectangle{radius:ink.radius;color:choice.hovered||choice.activeFocus?ink.hover:'transparent';border.color:choice.activeFocus?ink.accent:'transparent'}
                contentItem:Item{
                    StickerImage {id:poster;anchors.fill:parent;anchors.margins:8;imageData:root.imageFor(choice.modelData)}
                    Text {anchors.centerIn:parent;width:parent.width-12;horizontalAlignment:Text.AlignHCenter;wrapMode:Text.Wrap;text:'スタンプ\n画像を表示できません';visible:poster.status!==Image.Ready;textFormat:Text.PlainText;color:ink.muted;font.pixelSize:11}
                }
                Accessible.name:choice.modelData.alt+' '+choice.modelData.id
            }
        }
        Item {
            id:footer;anchors.bottom:parent.bottom;width:parent.width;height:40
            Row {spacing:4
                ActionButton {text:'‹';hint:'前の40個';enabled:(root.catalog.page||0)>0;onClicked:root.action('sticker-page',{packageId:root.catalog.packageId,page:root.catalog.page-1})}
                Text {height:38;verticalAlignment:Text.AlignVCenter;text:root.catalog.pages?(root.catalog.page+1)+' / '+root.catalog.pages:'';textFormat:Text.PlainText;color:ink.muted}
                ActionButton {text:'›';hint:'次の40個';enabled:(root.catalog.page||0)+1<(root.catalog.pages||0);onClicked:root.action('sticker-page',{packageId:root.catalog.packageId,page:root.catalog.page+1})}
            }
            ActionButton {objectName:'stickerPickerClose';anchors.right:parent.right;text:'閉じる';onClicked:root.action('sticker-close',{})}
        }
    }
}
