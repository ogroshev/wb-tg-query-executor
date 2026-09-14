package handler

import (
	"strings"
	"testing"
)

func TestMakeMessagesTrimsTrailingSpaces(t *testing.T) {
	res := "   Свод финки   \n----------\nИтого       \nМаржа: 1   \n"
	msgs := makeMessages("Сводка", res, true, true)
	if len(msgs) != 1 {
		t.Fatalf("want 1 message, got %d", len(msgs))
	}
	want := "Сводка\n<pre>   Свод финки\n----------\nИтого\nМаржа: 1\n</pre>"
	if msgs[0] != want {
		t.Fatalf("got %q\nwant %q", msgs[0], want)
	}
}

func TestMakeMessagesSplitsAtBlankLines(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("hdr\n")
	for g := 0; g < 40; g++ {
		for l := 0; l < 9; l++ {
			sb.WriteString(strings.Repeat("x", 30) + "     \n")
		}
		sb.WriteString("\n")
	}
	res := sb.String()
	msgs := makeMessages("Сводка", res, true, true)
	if len(msgs) < 2 {
		t.Fatalf("want >=2 messages, got %d", len(msgs))
	}
	var joined []string
	for i, m := range msgs {
		if len([]rune(m)) > kTelegramMaxMessageLen {
			t.Fatalf("message %d too long: %d", i, len([]rune(m)))
		}
		if !strings.HasPrefix(m, "<pre>") && i > 0 {
			t.Fatalf("message %d must start with <pre>", i)
		}
		if !strings.HasSuffix(m, "</pre>") {
			t.Fatalf("message %d must end with </pre>", i)
		}
		body := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(m, "Сводка\n"), "<pre>"), "</pre>")
		if i < len(msgs)-1 && !strings.HasSuffix(body, "\n"+strings.Repeat("x", 30)) {
			// cut must be at a blank line: the chunk ends with a data line, next chunk starts a new group
			t.Fatalf("message %d does not end at group boundary: %q", i, body[len(body)-40:])
		}
		joined = append(joined, body)
	}
	// nothing lost: all data lines are present in order
	all := strings.Join(joined, "\n\n")
	wantAll := strings.Join(func() []string {
		var out []string
		for _, l := range strings.Split(res, "\n") {
			out = append(out, strings.TrimRight(l, " "))
		}
		return out
	}(), "\n")
	if all != wantAll {
		t.Fatalf("content changed after split\n got len %d\nwant len %d", len(all), len(wantAll))
	}
}

func TestMakeMessagesHardCutsLongLine(t *testing.T) {
	res := strings.Repeat("y", 9000)
	msgs := makeMessages("d", res, false, false)
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages, got %d", len(msgs))
	}
	if strings.Join(msgs, "") != res {
		t.Fatal("content lost on hard cut")
	}
}
