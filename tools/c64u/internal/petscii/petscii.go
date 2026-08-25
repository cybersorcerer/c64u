package petscii

import (
	"fmt"
	"strings"
)

// Encode converts an ASCII string with escapes into PETSCII bytes.
// Supported escapes: \n \f1-\f8 \clr \del \stop \home \cup \cdn \cleft \cright
// Returns an error for an empty string, an unknown character or an unknown escape.
func Encode(s string) ([]byte, error) {
	if s == "" {
		return nil, fmt.Errorf("empty string")
	}

	var result []byte
	i := 0
	for i < len(s) {
		if s[i] == '\\' {
			if i+1 >= len(s) {
				return nil, fmt.Errorf("incomplete escape sequence at end of string")
			}
			rest := s[i+1:]
			var b byte
			var consumed int
			switch {
			case strings.HasPrefix(rest, "n"):
				b, consumed = 0x0D, 1
			// Function key codes are interleaved, not sequential: F2, F4, F6
			// and F8 are the shifted variants of F1, F3, F5 and F7.
			case strings.HasPrefix(rest, "f1"):
				b, consumed = 0x85, 2
			case strings.HasPrefix(rest, "f2"):
				b, consumed = 0x89, 2
			case strings.HasPrefix(rest, "f3"):
				b, consumed = 0x86, 2
			case strings.HasPrefix(rest, "f4"):
				b, consumed = 0x8A, 2
			case strings.HasPrefix(rest, "f5"):
				b, consumed = 0x87, 2
			case strings.HasPrefix(rest, "f6"):
				b, consumed = 0x8B, 2
			case strings.HasPrefix(rest, "f7"):
				b, consumed = 0x88, 2
			case strings.HasPrefix(rest, "f8"):
				b, consumed = 0x8C, 2
			case strings.HasPrefix(rest, "cright"):
				b, consumed = 0x1D, 6
			case strings.HasPrefix(rest, "cleft"):
				b, consumed = 0x9D, 5
			case strings.HasPrefix(rest, "clr"):
				b, consumed = 0x93, 3
			case strings.HasPrefix(rest, "cdn"):
				b, consumed = 0x11, 3
			case strings.HasPrefix(rest, "cup"):
				b, consumed = 0x91, 3
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
				return nil, fmt.Errorf("unknown escape sequence: \\%s", short)
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
	// A real line break means RETURN, like the \n escape. Callers that build a
	// line in Go rather than taking it from the command line write "\n".
	case c == '\n' || c == '\r':
		return 0x0D, nil
	case c >= 'a' && c <= 'z':
		// Lowercase ASCII maps to PETSCII 0x41-0x5A (unshifted letters).
		return c - 'a' + 0x41, nil
	case c >= 'A' && c <= 'Z':
		return c - 'A' + 0x41, nil
	// Printable ASCII 0x20-0x3F (space, digits, common punctuation) is
	// identical in PETSCII: ! " # $ % & ' ( ) * + , - . / 0-9 : ; < = > ?
	case c >= 0x20 && c <= 0x3F:
		return c, nil
	case c == '@':
		return 0x40, nil
	case c == '[':
		return 0x5B, nil
	case c == ']':
		return 0x5D, nil
	}
	return 0, fmt.Errorf("unknown character: %q", c)
}
