package gui

// A watch echo is more complete than a local receipt. Never replace it with
// synthesized fields, and never append a receipt to a different selected chat.
func appendReceipt(rows []Message, item Message) []Message {
	for _, existing := range rows {
		if existing.ID == item.ID {
			return rows
		}
	}
	rows = append(rows, item)
	if len(rows) > 100 {
		rows = rows[len(rows)-100:]
	}
	return rows
}
func (e *Engine) sentMessage(chat string, item Message) {
	if e.state.SelectedID == chat {
		e.state.Messages = appendReceipt(e.state.Messages, item)
	}
	if e.state.Mode == "demo" {
		e.demoMessages[chat] = appendReceipt(e.demoMessages[chat], item)
	}
	for i := range e.state.Chats {
		c := &e.state.Chats[i]
		if c.ID == chat && c.UpdatedAt <= item.Timestamp {
			c.Preview, c.Time, c.UpdatedAt = label(item.Text, 100), item.Time, item.Timestamp
			copy := item
			c.PreviewMessage = &copy
			c.PreviewKey = ""
		}
	}
}
