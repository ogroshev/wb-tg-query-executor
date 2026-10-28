package handler

import (
	"strings"
	"testing"

	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/postgres"
)

func sp(s string) *string { return &s }

func TestFormatNumber(t *testing.T) {
	cases := map[string]string{
		"1234567.890": "1\u00a0234\u00a0567,89", "-1234": "-1\u00a0234", "12": "12", "0.50": "0,5", "1000.000": "1\u00a0000", "NaN": "NaN",
	}
	for in, want := range cases {
		if got := formatNumber(in); got != want {
			t.Errorf("formatNumber(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRichTable(t *testing.T) {
	res := postgres.Result{
		Columns: []string{"Проект", "Шт", "_hidden"},
		Numeric: []bool{false, true, true},
		Rows:    [][]*string{{sp("A<b"), sp("1500"), sp("1")}, {nil, nil, nil}},
	}
	msgs := makeRichMessages(FormatTable, "Заголовок & co", true, "", "<p>низ</p>", res)
	if len(msgs) != 1 {
		t.Fatalf("want 1 message, got %d", len(msgs))
	}
	m := msgs[0]
	for _, want := range []string{"<h3>Заголовок &amp; co</h3>", `<th align="right">Шт</th>`, "<td>A&lt;b</td>",
		`<td align="right">1` + "\u00a0" + `500</td>`, "</table>\n<p>низ</p>"} {
		if !strings.Contains(m, want) {
			t.Errorf("message lacks %q:\n%s", want, m)
		}
	}
	if strings.Contains(m, "_hidden") {
		t.Errorf("hidden column rendered")
	}
	if got := makeRichMessages(FormatTable, "x", true, "<h3>свой</h3>", "", res)[0]; !strings.HasPrefix(got, "<h3>свой</h3>") {
		t.Errorf("rich_header not used: %s", got)
	}
}

func TestRichTableSplitRepeatsHeader(t *testing.T) {
	res := postgres.Result{Columns: []string{"n"}, Numeric: []bool{true}}
	for i := 0; i < 2*kRichMaxTableRows+5; i++ {
		res.Rows = append(res.Rows, []*string{sp("1")})
	}
	msgs := makeRichMessages(FormatTable, "h", true, "", "<p>f</p>", res)
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages, got %d", len(msgs))
	}
	for i, m := range msgs {
		if !strings.Contains(m, `<th align="right">n</th>`) {
			t.Errorf("message %d has no header row", i)
		}
		if (i == 0) != strings.HasPrefix(m, "<h3>") || (i == 2) != strings.HasSuffix(m, "<p>f</p>") {
			t.Errorf("message %d: header/footer misplaced", i)
		}
	}
}

func TestRichHTML(t *testing.T) {
	res := postgres.Result{Columns: []string{"frag"}, Numeric: []bool{false},
		Rows: [][]*string{{sp("<h3>a</h3>")}, {nil}, {sp("<p>b</p>")}}}
	msgs := makeRichMessages(FormatHTML, "ignored", false, "", "<p>f</p>", res)
	if len(msgs) != 1 || msgs[0] != "<h3>a</h3>\n<p>b</p>\n<p>f</p>" {
		t.Fatalf("got %q", msgs)
	}
}
