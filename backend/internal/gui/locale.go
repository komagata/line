package gui

import "strings"

// appText is only used for dedicated application-generated fields, never names,
// chat text, filenames, URLs or sticker titles supplied by LINE or the user.
func appText(locale, value string) string {
	if locale == "en" {
		if text, ok := englishLabels[value]; ok {
			return text
		}
	}
	return value
}

var englishLabels = map[string]string{
	"準備しています…":        "Preparing…",
	"LINEへログインしてください": "Please log in to LINE",
	"架空の会話 · デモ":      "Fictional conversation · Demo",
	"接続中":             "Connected",
	"LINEを確認しています…":   "Checking LINE…",
	"LINEを読み込めませんでした。再読み込みしてください":                     "Could not load LINE. Please reload",
	"LINEの応答を確認できませんでした":                              "Could not verify the LINE response",
	"受信が停止しました。設定から再読み込みしてください":                       "Receiving stopped. Reload in Settings",
	"読み込めるトークは最大500件です。作成中のトークと下書きを保つため、一覧の更新を保留しました": "Up to 500 chats can be loaded. List refresh was deferred to preserve new chats and drafts",
	"QRコードでログイン":                   "Log in with QR code",
	"QRコードを準備しています…":               "Preparing QR code…",
	"この端末に保存済みのLINEセッションを置き換えますか？": "Replace the LINE session saved on this device?",
	"スマートフォンのLINEでQRコードを読み取ってください": "Scan the QR code with LINE on your phone",
	"スマートフォンのLINEでログインを承認してください":   "Approve the login in LINE on your phone",
	"承認結果を確認し、この端末へ安全に保存しています…":    "Verifying approval and saving securely on this device…",
	"ログイン情報を保存しました。トークを読み込みます":     "Login saved. Loading chats",
	"ログインを中止しました":                  "Login cancelled",
	"QRコードの有効期限が切れました。もう一度お試しください": "QR code expired. Please try again",
	"ログインできませんでした。もう一度お試しください":     "Could not log in. Please try again",
	"ログイン結果を確認できません。スマートフォンの表示を確認し、再読み込みで保存状態を確認してください":  "Could not verify login. Check your phone, then reload to check the saved session",
	"ログインできませんでした。保存状態を確認して再試行してください":                    "Could not log in. Check the saved session and try again",
	"返信先が現在の履歴にありません。返信を解除してください":                        "The original message is outside the current history. Cancel the reply",
	"送信結果を確認できません。LINE側で確認してから、必要な場合だけ再送してください":          "Could not verify the send result. Check LINE before resending if needed",
	"履歴を取得できませんでした。再読み込みしてください":                          "Could not load history. Please reload",
	"一部のメッセージを復号できませんでした":                                "Some messages could not be decrypted",
	"所有スタンプを読み込んでいます…":                                   "Loading owned stickers…",
	"所有スタンプを取得できません（最大500パック）。再試行してください":                 "Could not load owned stickers (up to 500 packs). Please retry",
	"静止画で表示・音声なし。選択してから送信してください":                         "Still images, no audio. Select a sticker before sending",
	"名前・文字を変えるカスタムスタンプには対応していません":                        "Custom stickers with editable names or text are not supported",
	"このスタンプの所有情報を確認できません。スタンプ一覧を読み直すか、選択を解除してください":       "Could not verify sticker ownership. Reload the sticker list or clear the selection",
	"スタンプの有効期限が切れました。スタンプ一覧を開いて選び直してください":                "Sticker expired. Open the sticker list and choose again",
	"通知を表示できません。notify-send の導入を確認してください":                "Could not show notifications. Check that notify-send is installed",
	"新しいメッセージがあります":                                      "You have a new message",
	"操作結果を確認できません。LINE側で確認してから、必要な場合だけ再実行してください":         "Could not verify the operation. Check LINE before retrying if needed",
	"添付は20 MiB以下の通常ファイルを選んでください。リンクや特殊ファイルは使えません":        "Choose a regular file up to 20 MiB. Links and special files are not supported",
	"保存先を確認してください":                                       "Check the save destination",
	"ファイルの取得を中止しました":                                     "File download cancelled",
	"プレビューできません（PNG/JPEG・2 MiB・4096×4096まで）。保存を利用してください": "Preview unavailable (PNG/JPEG, up to 2 MiB and 4096×4096). Save the file instead",
	"保存結果を確認できません。既存のファイルは上書きしません。保存先を確認してください":          "Could not verify the save result. Existing files are not overwritten. Check the destination",
	"ファイルを保存しました":                                        "File saved",
	"トークを開く":                                             "Open chat",
	"まだメッセージはありません":                                      "No messages yet",
}
var demoNames = map[string]string{"demo-self": "Me", "demo-0": "Alex Morgan", "demo-1": "Weekend plans", "demo-2": "Jamie Lee", "demo-3": "Family", "demo-4": "Sam Taylor", "demo-new": "Robin Green", "demo-new-2": "Casey Parker"}
var demoTexts = map[string]string{"101": "What time shall we meet tomorrow?", "102": "How about around 7 pm?", "103": "Sounds good!", "104": "A photo of the cafe", "105": "Let's go there!", "202": "Looking forward to Saturday!", "203": "Thanks for the photo!", "204": "Got it", "205": "See you next week!"}
var demoPreviews = map[string]string{"demo-0": "How about the cafe near the station?", "demo-1": "Looking forward to Saturday!", "demo-2": "Thanks for the photo!", "demo-3": "Got it", "demo-4": "See you next week!"}

func (e *Engine) localizedMessage(m Message) Message {
	if e.locale != "en" {
		return m
	}
	if m.DemoFixture {
		m.Text = demoTexts[m.ID]
		m.Sender = demoNames[m.SenderID]
		m.Day = "Today"
		if m.Time == "昨日" {
			m.Time = "Yesterday"
		}
		if m.Sticker != nil {
			s := *m.Sticker
			s.Alt = "Fictional color stickers"
			m.Sticker = &s
		}
	}
	if m.SenderUnavailable {
		if m.Own {
			m.Sender = "Me"
		} else {
			m.Sender = "Unnamed contact"
		}
	}
	if m.Timestamp == 0 && m.Day == "日時不明" {
		m.Day = "Unknown date"
	}
	if m.GeneratedText {
		switch {
		case m.Status == "decryption_failed":
			m.Text = "This message could not be decrypted"
		case m.ContentType == 1:
			m.Text = "[Image]"
		case m.ContentType == 2:
			m.Text = "[Video]"
		case m.ContentType == 3:
			m.Text = "[Audio]"
		case m.ContentType == 7:
			withTitle := strings.HasPrefix(m.Text, "［スタンプ］ ")
			m.Text = "[Sticker]"
			if m.Sticker != nil && withTitle {
				m.Text += " " + m.Sticker.Alt
			}
		case m.ContentType == 14:
			m.Text = "[File: " + m.FileName + "]"
		default:
			m.Text = "[Unsupported message]"
		}
	}
	return m
}

// Caller holds mu. Projection copies changed slices and pointers so switching
// locale cannot rewrite canonical content, drafts, or in-flight state.
func (e *Engine) localizedView() View {
	v := e.state
	if e.locale != "en" {
		return v
	}
	v.StatusText = appText(e.locale, v.StatusText)
	v.ContactNote = appText(e.locale, v.ContactNote)
	v.HistoryNote = appText(e.locale, v.HistoryNote)
	v.SendNote = appText(e.locale, v.SendNote)
	v.NotificationNote = appText(e.locale, v.NotificationNote)
	v.Login.Label = appText(e.locale, v.Login.Label)
	v.Login.StatusText = appText(e.locale, v.Login.StatusText)
	v.Stickers.Note = appText(e.locale, v.Stickers.Note)
	if v.Account.NameUnavailable || v.Account.DemoFixture {
		v.Account.Name = "Me"
	}
	v.Chats = append([]Chat{}, v.Chats...)
	for i := range v.Chats {
		c := &v.Chats[i]
		if c.DemoFixture {
			if name, ok := demoNames[c.ID]; ok {
				c.Name = name
			}
			if c.PreviewKey == "" && c.PreviewMessage == nil {
				if preview, ok := demoPreviews[c.ID]; ok {
					c.Preview = preview
				}
			}
			if c.Time == "昨日" {
				c.Time = "Yesterday"
			}
		}
		if c.NameUnavailable {
			c.Name = "Name unavailable"
		}
		if c.PreviewKey != "" {
			c.Preview = appText(e.locale, c.PreviewKey)
		}
		if c.PreviewMessage != nil {
			c.Preview = label(e.localizedMessage(*c.PreviewMessage).Text, 100)
		}
	}
	v.Contacts = append([]Contact{}, v.Contacts...)
	for i := range v.Contacts {
		c := &v.Contacts[i]
		if c.DemoFixture {
			if name, ok := demoNames[c.ID]; ok {
				c.Name = name
			}
		} else if c.NameUnavailable {
			c.Name = "Unnamed contact"
		}
	}
	v.Messages = append([]Message{}, v.Messages...)
	for i, m := range v.Messages {
		v.Messages[i] = e.localizedMessage(m)
	}
	if v.Reply != nil && v.Reply.Source != nil {
		r := *v.Reply
		m := e.localizedMessage(*r.Source)
		r.Text = label(m.Text, 160)
		r.Sender = m.Sender
		v.Reply = &r
	}
	if v.Mode == "demo" {
		v.Stickers.Products = append([]StickerProduct{}, v.Stickers.Products...)
		for i := range v.Stickers.Products {
			if v.Stickers.Products[i].ID == "1" {
				v.Stickers.Products[i].Name = "Fictional color stickers"
			}
		}
		v.Stickers.Items = append([]Sticker{}, v.Stickers.Items...)
		for i := range v.Stickers.Items {
			v.Stickers.Items[i].Alt = "Fictional color stickers"
		}
		if v.Stickers.Selected != nil {
			s := *v.Stickers.Selected
			s.Alt = "Fictional color stickers"
			v.Stickers.Selected = &s
		}
	}
	return v
}
