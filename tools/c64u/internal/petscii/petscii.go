package petscii

import (
	"fmt"
	"strings"
)

// Encode konvertiert einen ASCII+Escape-String zu PETSCII-Bytes.
// Unterstützte Escape-Sequenzen: \n \f1-\f8 \clr \del \stop \home
// Gibt Fehler zurück bei: leerem String, unbekannten Zeichen, unbekannten Escape-Sequenzen.
func Encode(s string) ([]byte, error) {
	if s == "" {
		return nil, fmt.Errorf("leerer String")
	}

	var result []byte
	i := 0
	for i < len(s) {
		if s[i] == '\\' {
			if i+1 >= len(s) {
				return nil, fmt.Errorf("unvollständige Escape-Sequenz am Stringende")
			}
			rest := s[i+1:]
			var b byte
			var consumed int
			switch {
			case strings.HasPrefix(rest, "n"):
				b, consumed = 0x0D, 1
			case strings.HasPrefix(rest, "f1"):
				b, consumed = 0x85, 2
			case strings.HasPrefix(rest, "f2"):
				b, consumed = 0x86, 2
			case strings.HasPrefix(rest, "f3"):
				b, consumed = 0x87, 2
			case strings.HasPrefix(rest, "f4"):
				b, consumed = 0x88, 2
			case strings.HasPrefix(rest, "f5"):
				b, consumed = 0x89, 2
			case strings.HasPrefix(rest, "f6"):
				b, consumed = 0x8A, 2
			case strings.HasPrefix(rest, "f7"):
				b, consumed = 0x8B, 2
			case strings.HasPrefix(rest, "f8"):
				b, consumed = 0x8C, 2
			case strings.HasPrefix(rest, "clr"):
				b, consumed = 0x93, 3
			case strings.HasPrefix(rest, "del"):
				b, consumed = 0x14, 3
			case strings.HasPrefix(rest, "stop"):
				b, consumed = 0x03, 4
			case strings.HasPrefix(rest, "home"):
				b, consumed = 0x13, 4
			default:
				short := rest
				if len(short) > 4 {
					short = short[:4]
				}
				return nil, fmt.Errorf("unbekannte Escape-Sequenz: \\%s", short)
			}
			result = append(result, b)
			i += 1 + consumed
			continue
		}

		c := s[i]
		b, err := asciiToPetscii(c)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
		i++
	}
	return result, nil
}

func asciiToPetscii(c byte) (byte, error) {
	switch {
	case c >= 'a' && c <= 'z':
		return c - 'a' + 0x41, nil
	case c >= 'A' && c <= 'Z':
		return c - 'A' + 0x41, nil
	case c >= '0' && c <= '9':
		return c, nil
	case c == ' ':
		return 0x20, nil
	case c == '.':
		return 0x2E, nil
	case c == ',':
		return 0x2C, nil
	case c == ';':
		return 0x3B, nil
	case c == ':':
		return 0x3A, nil
	case c == '?':
		return 0x3F, nil
	case c == '!':
		return 0x21, nil
	case c == '"':
		return 0x22, nil
	case c == '+':
		return 0x2B, nil
	case c == '-':
		return 0x2D, nil
	case c == '*':
		return 0x2A, nil
	case c == '/':
		return 0x2F, nil
	case c == '=':
		return 0x3D, nil
	case c == '(':
		return 0x28, nil
	case c == ')':
		return 0x29, nil
	}
	return 0, fmt.Errorf("unbekanntes Zeichen: %q", c)
}
