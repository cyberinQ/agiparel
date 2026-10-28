// Author: Sunil Jain, 2026-10-28
// Prevent accidental regression to short, chapter-specific Aggie panels or in-memory-only chat.
package web

import (
	"strings"
	"testing"
)

func TestAggieSharedConversationAndFullHeight(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		raw, err := staticFS.ReadFile("static/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	chat := read("portal.js")
	for _, expected := range []string{
		"sessionStorage.getItem(STORAGE_KEY)", "sessionStorage.setItem(STORAGE_KEY",
		"history.slice(-12)", "saved.pending", "Reply interrupted by navigation",
		"input.addEventListener('input', persist)", "window.addEventListener('pagehide', persist)",
		"close.setAttribute('aria-label', 'Collapse Aggie')", "answerNode.textContent = data.reply",
	} {
		if !strings.Contains(chat, expected) {
			t.Errorf("shared Aggie script missing %q", expected)
		}
	}
	for _, name := range []string{"portal.css", "workspace_nav.css"} {
		css := read(name)
		for _, expected := range []string{"height:calc(100dvh - 24px)", "height:calc(100dvh - 10px)", "max-height:none", "min-height:0", "overflow-y:auto"} {
			if !strings.Contains(css, expected) {
				t.Errorf("%s must give Aggie a full-height scrolling transcript; missing %q", name, expected)
			}
		}
	}
}
