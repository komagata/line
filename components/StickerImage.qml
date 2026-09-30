import QtQuick
Image {
    property string imageData: ''
    source: imageData.length<=87407 && /^data:image\/(png|jpeg);base64,[A-Za-z0-9+/]+={0,2}$/.test(imageData) && (imageData.indexOf('data:image/png;base64,iVBORw0KGgo')===0 || imageData.indexOf('data:image/jpeg;base64,/9j/')===0) ? imageData : ''
    sourceSize: Qt.size(512,512)
    fillMode: Image.PreserveAspectFit
    cache: false
}
