import QtQuick
import QtTest
import '../../components'
import '../../components/MessageLinks.js' as Links

TestCase {
    id: tests
    name: 'MessageLinks'
    when: windowShown
    visible: true
    width: 720; height: 540
    MessageBubble {
        id: bubble
        width: parent.width - 40
        message: ({id:'links',text:'本文 https://example.org/道?a=1&b=2。',own:false})
    }
    SignalSpy { id: opened; target: bubble; signalName: 'linkActivated' }
    TextEdit { id: pasteTarget; visible: false; textFormat: TextEdit.PlainText }
    function setBody(text) {
        bubble.message={id:'links',text:text,own:false}
        wait(10)
        return findChild(bubble,'messageText-links')
    }
    function init() { opened.clear(); bubble.width=680 }
    function test_inline_url_has_hit_target() {
        var body=setBody('本文 https://example.org/道?a=1&b=2。')
        verify(body!==null)
        var point=body.positionToRectangle(8)
        compare(body.linkAt(point.x+2,point.y+point.height/2),'https://example.org/道?a=1&b=2')
    }
    function test_parser_data() { return [
        {tag:'query-unicode',text:'本文 HTTPS://Example.org/道?q=%E6%97%A5&b=2。続き',urls:['HTTPS://Example.org/道?q=%E6%97%A5&b=2']},
        {tag:'balanced',text:'(https://example.org/a_(b)). https://example.org/x?x=(y)&a=2!',urls:['https://example.org/a_(b)','https://example.org/x?x=(y)&a=2']},
        {tag:'japanese',text:'「https://example.org/道」『http://example.org/次』 https://example.org/末尾、次',urls:['https://example.org/道','http://example.org/次','https://example.org/末尾']},
        {tag:'valid-endings',text:'https://example.org/a%29 https://example.org/a/?x=1& https://example.org/#hash',urls:['https://example.org/a%29','https://example.org/a/?x=1&','https://example.org/#hash']},
        {tag:'schemes',text:'javascript:alert(1) data:text/html,a file:///tmp/a javascript:https://example.org',urls:[]},
        {tag:'bad-authorities',text:'https:// https:///oops https://user:pw@example.org https://.evil.org https://bad..org https://-bad.org https://example.org:99999 https://example.org:abc https://999.1.1.1',urls:[]},
        {tag:'controls',text:'https://example.org/a\u0001b https://example.org/a\u007fb https://example.org/a\u0085b https://example.org/a%0Ab https://example.org/a%zz https://example.org\\evil',urls:[]},
        {tag:'host-types',text:'http://localhost:8080/x https://127.0.0.1/x https://[::1]/x https://xn--wgv71a119e.jp/道',urls:['http://localhost:8080/x','https://127.0.0.1/x','https://[::1]/x','https://xn--wgv71a119e.jp/道']}
    ] }
    function test_malformed_ipv6_and_numeric_authorities() {
        for(var url of ['http://[:1::]/','http://[1::2:]/','http://[1:2:3:4:5:6:7:8::]/','http://0x7f000001/'])
            verify(!Links.valid(url),url)
        verify(Links.valid('https://[2001:db8::1]:443/path'))
    }
    function test_parser(data) {
        var spans=Links.spans(data.text)
        compare(spans.map(s=>s.url),data.urls)
        for(var span of spans) {
            compare(data.text.slice(span.start,span.end),span.url)
            verify(Links.valid(span.url))
        }
    }
    function test_literal_roundtrip_data() { return [
        {tag:'html',text:'<img src="https://example.invalid/beacon"> &amp; &#60; <b>太字ではない</b>\nhttps://example.org/?a=1&b=%22'},
        {tag:'attribute',text:'https://example.org/\" onclick=\"bad <img src=file:///tmp/no-image>\n</a><style>body{background:url(https://example.invalid/a)}</style>'},
        {tag:'whitespace',text:' 先頭  二つ\tタブ\n\n日本語 😀 & < > " \'\n末尾  \n'},
        {tag:'nonhttp',text:'javascript:alert(1) data:image/png;base64,AAAA file:///etc/passwd'},
        {tag:'empty',text:''}
    ] }
    function test_literal_roundtrip(data) {
        var body=setBody(data.text)
        // Rich QTextDocument represents LF as a paragraph separator internally.
        // Verify both document text and the *actual plain clipboard* roundtrip.
        compare(body.getText(0,body.length).replace(/\u2029/g,'\n'),data.text)
        body.selectAll(); compare(body.selectedText.replace(/\u2029/g,'\n'),data.text)
        if(data.text.length) {body.copy(); pasteTarget.text='';pasteTarget.paste();compare(pasteTarget.text,data.text)}
        compare(opened.count,0)
        verify(!/<(?:img|style|script|iframe)\b/i.test(Links.markup(data.text,Links.spans(data.text),'#83b487')))
        verify(!/<(?:img|script|iframe)\b/i.test(body.text))
        verify(body.selectByMouse);verify(body.readOnly)
    }
    function test_explicit_click_once_and_nonlink_selection() {
        var body=setBody('本文 https://example.org/道?a=1&b=2。 後ろ')
        var point=body.positionToRectangle(8)
        mouseMove(body,point.x+2,point.y+point.height/2)
        compare(body.hoveredLink,'https://example.org/道?a=1&b=2')
        compare(opened.count,0)
        mouseClick(body,point.x+2,point.y+point.height/2)
        compare(opened.count,1);compare(opened.signalArguments[0][0],'https://example.org/道?a=1&b=2')
        opened.clear();mouseClick(body,2,5);compare(opened.count,0)
        mouseDrag(body,point.x+2,point.y+point.height/2,60,0)
        verify(body.selectedText.length>0);compare(opened.count,0)
        body.linkActivated('javascript:alert(1)');body.linkActivated('https://other.example/not-in-message');compare(opened.count,0)
    }
    function test_keyboard_and_multiple_links() {
        var first='https://example.org/one',second='HTTP://example.org/two?x=1&y=2'
        var body=setBody('前 '+first+'\n次 '+second)
        body.forceActiveFocus();keyClick(Qt.Key_Home,Qt.ControlModifier)
        keyClick(Qt.Key_Right);keyClick(Qt.Key_Right);keyClick(Qt.Key_Right)
        keyClick(Qt.Key_Return,Qt.ControlModifier)
        compare(opened.count,1);compare(opened.signalArguments[0][0],first)
        body.cursorPosition=body.length-1;keyClick(Qt.Key_Return,Qt.ControlModifier)
        compare(opened.count,2);compare(opened.signalArguments[1][0],second)
        body.cursorPosition=0;keyClick(Qt.Key_Return,Qt.ControlModifier);compare(opened.count,2)
    }
    function test_no_link_stays_literal_and_input_bound() {
        var body=setBody('<img src="file:///tmp/no-image"> &amp;\n普通の本文')
        compare(body.getText(0,body.length).replace(/\u2029/g,'\n'),bubble.message.text)
        var prefix='https://example.org/'
        var maximum=prefix+new Array(10000-prefix.length+1).join('a')
        compare(maximum.length,10000);compare(Links.spans(maximum).length,1)
        compare(Links.spans(maximum+'a').length,0)
        verify(!Links.valid(maximum+'a'))
    }
    function test_wrapped_long_url_stays_selectable_and_clickable() {
        bubble.width=300
        var url='https://example.org/'+new Array(10).join('long-path-')+'?a=1&b=2'
        var body=setBody('前\n'+url+'。\n後')
        verify(body.contentHeight>body.font.pixelSize*3)
        verify(body.contentWidth<=body.width+1)
        var point=body.positionToRectangle(70)
        compare(body.linkAt(point.x+2,point.y+point.height/2),url)
        mouseClick(body,point.x+2,point.y+point.height/2);compare(opened.count,1);compare(opened.signalArguments[0][0],url)
        body.selectAll();compare(body.selectedText.replace(/\u2029/g,'\n'),'前\n'+url+'。\n後')
    }
}
