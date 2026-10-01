package gui

import (
	"context"
	"io"
	"time"

	"github.com/kongesque/line-cli/internal/helper"
)

func staticNotify(ctx context.Context) bool { return localizedNotify(ctx, "ja") }
func localizedNotify(ctx context.Context, locale string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := helper.Command(ctx, "/usr/bin/notify-send", "LINE", appText(locale, "新しいメッセージがあります"))
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return helper.Run(cmd) == nil
}
