package handler

import (
	"fmt"
	"html"
	"strings"

	"gitlab.com/wb-dynamics/wb-tg-query-executor/internal/postgres"
)

const (
	// Rich message limits (Bot API 10.1+): 32768 chars, 500 blocks (every table row is a block).
	kRichMaxLen       = 30000
	kRichMaxTableRows = 300

	FormatPre   = "pre"   // legacy: xsql.Pretty in <pre> via sendMessage
	FormatTable = "table" // result set -> native table via sendRichMessage
	FormatHTML  = "html"  // query returns rich-HTML fragments (first column), joined by "\n"
)

// makeRichMessages renders a query result into one or more rich-HTML messages.
// table: <h3>chat_describe</h3> (or rich_header as is) + native table (numeric columns right-aligned, 1 234 567,89) + footer.
// html:  values of the first column joined by newlines, header and footer around them.
// Columns whose name starts with "_" are not shown in the table.
func makeRichMessages(format, desc string, addHeader bool, header, footer string, res postgres.Result) []string {
	head := ""
	if strings.TrimSpace(header) != "" {
		head = header + "\n"
	} else if addHeader && desc != "" {
		head = "<h3>" + html.EscapeString(desc) + "</h3>\n"
	}
	tail := ""
	if strings.TrimSpace(footer) != "" {
		tail = "\n" + footer
	}

	if format == FormatHTML {
		var parts []string
		for _, r := range res.Rows {
			if len(r) > 0 && r[0] != nil {
				parts = append(parts, *r[0])
			}
		}
		return splitHTML(head, parts, tail)
	}

	var cols []int
	for i, c := range res.Columns {
		if !strings.HasPrefix(c, "_") {
			cols = append(cols, i)
		}
	}
	var hdr strings.Builder
	hdr.WriteString("<tr>")
	for _, i := range cols {
		hdr.WriteString(cellTag("th", res.Numeric[i]) + html.EscapeString(res.Columns[i]) + "</th>")
	}
	hdr.WriteString("</tr>")

	var rows []string
	for _, r := range res.Rows {
		var sb strings.Builder
		sb.WriteString("<tr>")
		for _, i := range cols {
			v := ""
			if r[i] != nil {
				v = *r[i]
				if res.Numeric[i] {
					v = formatNumber(v)
				}
			}
			sb.WriteString(cellTag("td", res.Numeric[i]) + html.EscapeString(v) + "</td>")
		}
		sb.WriteString("</tr>")
		rows = append(rows, sb.String())
	}

	// one table per message; long results are split, the header row is repeated
	var msgs []string
	for start := 0; start < len(rows); {
		size := len(head) + len(tail) + len(hdr.String()) + 64
		end := start
		for end < len(rows) && end-start < kRichMaxTableRows && size+len(rows[end]) < kRichMaxLen {
			size += len(rows[end])
			end++
		}
		if end == start { // a single huge row: send it anyway, Telegram will reject and we fall back
			end++
		}
		var sb strings.Builder
		if start == 0 {
			sb.WriteString(head)
		}
		sb.WriteString("<table bordered striped compact>")
		sb.WriteString(hdr.String())
		sb.WriteString(strings.Join(rows[start:end], ""))
		sb.WriteString("</table>")
		if end == len(rows) {
			sb.WriteString(tail)
		}
		msgs = append(msgs, sb.String())
		start = end
	}
	return msgs
}

func cellTag(tag string, numeric bool) string {
	if numeric {
		return "<" + tag + ` align="right">`
	}
	return "<" + tag + ">"
}

// splitHTML packs fragments into messages under the rich length limit, never cutting a fragment.
func splitHTML(head string, parts []string, tail string) []string {
	var msgs []string
	cur := head
	for _, p := range parts {
		if len(cur)+len(p)+1 > kRichMaxLen && cur != "" && cur != head {
			msgs = append(msgs, cur)
			cur = ""
		}
		if cur != "" && cur != head {
			cur += "\n"
		}
		cur += p
	}
	if len(cur)+len(tail) > kRichMaxLen && cur != "" {
		msgs = append(msgs, cur)
		cur = ""
	}
	cur += tail
	if strings.TrimSpace(cur) != "" {
		msgs = append(msgs, cur)
	}
	return msgs
}

// formatNumber: "-1234567.890" -> "−1 234 567,89" style used in noco (space thousands, comma decimals).
// Non-breaking spaces keep the number on one line in a narrow column.
func formatNumber(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	sign := ""
	if s[0] == '-' || s[0] == '+' {
		if s[0] == '-' {
			sign = "-"
		}
		s = s[1:]
	}
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i+1:]
	}
	for _, c := range intPart {
		if c < '0' || c > '9' {
			return sign + s // NaN, Infinity, exponent — leave as is
		}
	}
	frac = strings.TrimRight(frac, "0")
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteString(" ")
		}
		b.WriteRune(c)
	}
	out := sign + b.String()
	if frac != "" {
		out += "," + frac
	}
	return out
}

func describeResult(res postgres.Result) string {
	return fmt.Sprintf("%d cols, %d rows", len(res.Columns), len(res.Rows))
}
