package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kongesque/line-cli/internal/gui"
)

func main() { run(gui.NewDirect().Operations()) }

func run(operations gui.Operations) {
	parent := os.Getppid()
	// Upstream library diagnostics can contain remote detail; GUI stdout is JSONL only.
	log.SetOutput(io.Discard)
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	input, err := pollable(os.Stdin)
	if err != nil {
		return
	}
	defer input.Close()
	output, err := pollable(os.Stdout)
	if err != nil {
		return
	}
	defer output.Close()
	writer := gui.NewWriter(output)
	engine := gui.NewEngine(operations, func(v gui.View) {
		if writer.Publish(v) != nil {
			cancel()
		}
	})
	writer.Publish(engine.View())
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		if err := writer.Run(ctx); err != nil && ctx.Err() == nil {
			cancel()
		}
	}()
	go func() {
		timer := time.NewTicker(500 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if os.Getppid() != parent {
					cancel()
					return
				}
			}
		}
	}()
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		reader := gui.NewReader(input, func(raw json.RawMessage) {
			var cmd gui.Command
			if json.Unmarshal(raw, &cmd) == nil {
				engine.Handle(cmd)
			}
		})
		_ = reader.Run(ctx)
		cancel()
	}()
	<-ctx.Done()
	input.Close()
	output.Close()
	engine.Close()
	writer.Close()
	output.Close()
	<-readDone
	<-writerDone
}
