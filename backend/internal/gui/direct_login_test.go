package gui

import (
	"errors"
	"testing"
)

func TestPINEventOnlyPublishesBoundedDigits(t *testing.T) {
	for _, pin := range []string{"123", "1234567890123", "12ab56", "１２３４", "1234\n"} {
		called := false
		if err := emitPIN(pin, func(LoginEvent) error { called = true; return nil }); err == nil || called {
			t.Fatalf("accepted invalid PIN %q: err=%v called=%v", pin, err, called)
		}
	}
	var got LoginEvent
	if err := emitPIN("123456", func(ev LoginEvent) error { got = ev; return nil }); err != nil || got.Stage != "phone" || got.PIN != "123456" {
		t.Fatalf("valid PIN event: %+v %v", got, err)
	}
	marker := errors.New("sink rejected")
	if err := emitPIN("123456", func(LoginEvent) error { return marker }); !errors.Is(err, marker) {
		t.Fatalf("sink error lost: %v", err)
	}
}
