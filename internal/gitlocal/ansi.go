package gitlocal

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var ansiRe = regexp.MustCompile(`\x1b\[([0-9;]*)m|\x1b\[[0-9]*[A-HJK]`)

// AnsiToHTML converts ANSI escape sequences to inline-styled HTML spans.
// Handles 24-bit RGB, 256-color, and standard color codes.
func AnsiToHTML(input string) string {
	// Escape HTML entities first
	input = strings.ReplaceAll(input, "&", "&amp;")
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")

	var b strings.Builder
	b.Grow(len(input) * 2)

	open := false
	last := 0

	for _, loc := range ansiRe.FindAllStringIndex(input, -1) {
		b.WriteString(input[last:loc[0]])
		last = loc[1]

		code := input[loc[0]:loc[1]]

		// Skip non-SGR sequences (cursor movement, erase line, etc.)
		if !strings.HasSuffix(code, "m") {
			continue
		}

		params := ansiRe.FindStringSubmatch(code)
		if len(params) < 2 {
			continue
		}

		if open {
			b.WriteString("</span>")
			open = false
		}

		style := ansiStyle(params[1])
		if style != "" {
			b.WriteString(`<span style="`)
			b.WriteString(style)
			b.WriteString(`">`)
			open = true
		}
	}

	b.WriteString(input[last:])
	if open {
		b.WriteString("</span>")
	}

	return b.String()
}

func ansiStyle(params string) string {
	if params == "" || params == "0" {
		return "" // reset
	}

	parts := strings.Split(params, ";")
	var styles []string

	for i := 0; i < len(parts); i++ {
		p := parts[i]
		switch p {
		case "0":
			return "" // reset
		case "1":
			styles = append(styles, "font-weight:bold")
		case "2":
			styles = append(styles, "opacity:0.5")
		case "3":
			styles = append(styles, "font-style:italic")
		case "4":
			styles = append(styles, "text-decoration:underline")
		case "31":
			styles = append(styles, "color:#f87171")
		case "32":
			styles = append(styles, "color:#4ade80")
		case "33":
			styles = append(styles, "color:#facc15")
		case "34":
			styles = append(styles, "color:#60a5fa")
		case "35":
			styles = append(styles, "color:#c084fc")
		case "36":
			styles = append(styles, "color:#22d3ee")
		case "37":
			styles = append(styles, "color:#d4d4d8")
		case "91":
			styles = append(styles, "color:#f87171")
		case "92":
			styles = append(styles, "color:#4ade80")
		case "93":
			styles = append(styles, "color:#facc15")
		case "94":
			styles = append(styles, "color:#60a5fa")
		case "95":
			styles = append(styles, "color:#c084fc")
		case "96":
			styles = append(styles, "color:#22d3ee")
		case "97":
			styles = append(styles, "color:#f0ebe4")
		case "38": // foreground: extended color
			if i+1 < len(parts) {
				if parts[i+1] == "5" && i+2 < len(parts) {
					// 256-color: 38;5;N
					if c := color256(parts[i+2]); c != "" {
						styles = append(styles, "color:"+c)
					}
					i += 2
				} else if parts[i+1] == "2" && i+4 < len(parts) {
					// 24-bit RGB: 38;2;R;G;B
					styles = append(styles, fmt.Sprintf("color:rgb(%s,%s,%s)", parts[i+2], parts[i+3], parts[i+4]))
					i += 4
				}
			}
		case "48": // background: extended color
			if i+1 < len(parts) {
				if parts[i+1] == "5" && i+2 < len(parts) {
					if c := color256(parts[i+2]); c != "" {
						styles = append(styles, "background:"+c)
					}
					i += 2
				} else if parts[i+1] == "2" && i+4 < len(parts) {
					styles = append(styles, fmt.Sprintf("background:rgb(%s,%s,%s)", parts[i+2], parts[i+3], parts[i+4]))
					i += 4
				}
			}
		case "41":
			styles = append(styles, "background:#7f1d1d")
		case "42":
			styles = append(styles, "background:#14532d")
		case "43":
			styles = append(styles, "background:#713f12")
		case "44":
			styles = append(styles, "background:#1e3a5f")
		case "45":
			styles = append(styles, "background:#581c87")
		case "46":
			styles = append(styles, "background:#164e63")
		}
	}

	if len(styles) == 0 {
		return ""
	}
	return strings.Join(styles, ";")
}

// color256 converts a 256-color index to a hex color.
func color256(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 255 {
		return ""
	}

	// Standard 16 colors
	std16 := []string{
		"#000000", "#aa0000", "#00aa00", "#aa5500", "#0000aa", "#aa00aa", "#00aaaa", "#aaaaaa",
		"#555555", "#ff5555", "#55ff55", "#ffff55", "#5555ff", "#ff55ff", "#55ffff", "#ffffff",
	}
	if n < 16 {
		return std16[n]
	}

	// 216-color cube (16-231)
	if n < 232 {
		n -= 16
		r := n / 36
		g := (n % 36) / 6
		b := n % 6
		vals := []int{0, 95, 135, 175, 215, 255}
		return fmt.Sprintf("#%02x%02x%02x", vals[r], vals[g], vals[b])
	}

	// Grayscale (232-255)
	v := 8 + (n-232)*10
	return fmt.Sprintf("#%02x%02x%02x", v, v, v)
}
