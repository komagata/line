# Message body links

HTTP(S) addresses in a message body are underlined in the existing accent color.
Click one to open it in the default browser. The selectable body is in the Tab
order: use arrow keys to position the cursor in a link, then Ctrl+Enter to open
it. Shift+arrow and mouse dragging still select text; Ctrl+C copies the selection.
The existing `⋯` menu keeps its first eight link actions and whole-message copy.
Opening a message, hovering, selecting, and copying do not open a URL or request
a preview. Search still searches the original message text.

## Untrusted text boundary

`components/MessageLinks.js` is the shared detector and activation validator for
the message body and menu. It keeps original spelling/case, percent escapes,
query ampersands, Unicode paths, and balanced parentheses. Sentence punctuation
and Japanese closing punctuation stay outside the anchor. Only explicit HTTP
and HTTPS URLs qualify. Credentials, backslashes, control/bidi characters,
malformed escapes, invalid ports and malformed authorities fail closed. Hosts
are ASCII DNS (including punycode), IPv4, or bracketed IPv6; Unicode hostnames
remain literal text. IPv6 zone IDs and IPv4-embedded IPv6 are not recognized.
Detection and activation reject strings longer than the existing runtime's
10,000 UTF-16 code-unit message limit. No shortening is applied to message text.

`MessageBubble.qml` supplies `TextEdit.RichText` **only** from
`MessageLinks.markup`. Every raw text segment and the href is escaped (`&`, `<`,
`>`, double and single quotes) before any application-authored markup is added.
The only generated elements are `span` (fixed pre-wrap style) and `a` (escaped
href, fixed underline and validated application palette color). Raw message
HTML, entities, image tags and styles never become document markup. The helper
is not a general HTML sanitizer and must not accept remote markup or styles.
All other external labels remain PlainText. No lint/security checker is disabled.

Native TextEdit selection, hit testing and wrapping handle mouse interaction;
there is no bubble-wide input overlay. A passive HoverHandler supplies the hand
cursor. The bubble revalidates and requires the URL to belong to its current
message before emitting `linkActivated`. ChatView validates again at its sole
`Qt.openUrlExternally` boundary. Neither rendering nor the parser fetches URLs.

Qt's rich document exposes line feeds as U+2029 paragraph separators to
`getText`/`selectedText`. Tests account for that internal representation and
separately paste the actual copied text into a PlainText editor to verify the
original spaces, tabs, blank/trailing lines, literal HTML, Japanese and emoji.

## Verification

`tests/qml/tst_MessageLinks.qml` covers URL parsing, rejected schemes and
authorities, hostile HTML/attribute injection, selection and actual clipboard
roundtrips, exact click signals, no activation on hover/drag/nonlink clicks,
Ctrl+Enter, long wrapped links and input bounds. It observes signals and never
opens a browser. `tst_Features.qml` preserves menu/search feature coverage and
captures fictional compact/wide screenshots at `/tmp/omarchy-line-links-*.png`.

`python3 -B tests/test_message_links.py` runs the actual Qt renderer with hostile
image markup pointing at an ephemeral **localhost-only** HTTP sentinel and
asserts zero requests. This requires permission to bind a loopback TCP socket;
the restricted AO worker sandbox denies it. Run it from the coordinator's
normal test environment. Failure to start the sentinel is a test error, never
a skipped or passing security check.

Qt references: [TextEdit](https://doc.qt.io/qt-6/qml-qtquick-textedit.html) and
[supported rich text subset](https://doc.qt.io/qt-6/richtext-html-subset.html).
