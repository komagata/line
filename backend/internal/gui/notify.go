package gui

import (
	"context"
	"io"
	"time"

	"github.com/kongesque/line-cli/internal/helper"
)

func staticNotify(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := helper.Command(ctx, "/usr/bin/notify-send", "LINE", "新しいメッセージがあります")
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return helper.Run(cmd) == nil
}
