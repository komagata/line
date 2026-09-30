"""Observe the real Qt renderer against a local-only hostile image sentinel."""
import http.server
import json
import os
from pathlib import Path
import subprocess
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[1]


class MessageLinkResources(unittest.TestCase):
    def test_raw_markup_never_requests_an_image(self):
        requests = []

        class Sentinel(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                requests.append(self.path)
                self.send_response(204)
                self.end_headers()

            def log_message(self, *_):
                pass

        with http.server.ThreadingHTTPServer(('127.0.0.1', 0), Sentinel) as server:
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            try:
                url = f'http://127.0.0.1:{server.server_port}/must-not-fetch.png'
                raw = f'<img src="{url}"> &lt;img src="{url}"&gt;\n<a href="file:///tmp/nope">literal</a> {url}'
                qml = '''import QtQuick
import QtTest
import %s
TestCase {
    name: 'MessageResourceBoundary'; when: windowShown
    width:720; height:540; visible:true
    MessageBubble { id:bubble; width:680; message:({id:'hostile',text:%s}) }
    function test_literal_no_images() {
        var body=findChild(bubble,'messageText-hostile')
        compare(body.getText(0,body.length).replace(/\\u2029/g,'\\n'),%s)
        verify(!/<img\\b/i.test(body.text))
        wait(250)
    }
}''' % (json.dumps((ROOT / 'components').as_uri()), json.dumps(raw), json.dumps(raw))
                with tempfile.TemporaryDirectory(prefix='omarchy-links-') as directory:
                    path = Path(directory) / 'tst_Resource.qml'
                    path.write_text(qml)
                    env = dict(os.environ, QT_QPA_PLATFORM='offscreen', QT_QUICK_BACKEND='software',
                               QT_QUICK_CONTROLS_STYLE='Basic', QT_QPA_PLATFORMTHEME='generic')
                    result = subprocess.run(['qmltestrunner', '-input', str(path), '-import',
                                             str(ROOT / 'tests/qml/imports'), '-o', '-,txt'],
                                            env=env, text=True, capture_output=True, timeout=30)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(requests, [], 'Rendering raw message markup fetched an image')
            finally:
                server.shutdown()
                thread.join(timeout=2)


if __name__ == '__main__':
    unittest.main()
