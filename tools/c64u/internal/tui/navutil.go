package tui

// applyListNav applies a vim-style navigation key to a cursor position and
// returns the new cursor, clamped to [0, count-1]. page is the half-page step
// size for ctrl+d / ctrl+u. Unknown keys return the cursor unchanged.
func applyListNav(key string, cursor, count, page int) int {
	if count <= 0 {
		return 0
	}
	half := page / 2
	if half < 1 {
		half = 1
	}
	switch key {
	case "j", "down":
		cursor++
	case "k", "up":
		cursor--
	case "g":
		cursor = 0
	case "G":
		cursor = count - 1
	case "ctrl+d":
		cursor += half
	case "ctrl+u":
		cursor -= half
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > count-1 {
		cursor = count - 1
	}
	return cursor
}

// isListNavKey reports whether key is one handled by applyListNav.
func isListNavKey(key string) bool {
	switch key {
	case "j", "k", "down", "up", "g", "G", "ctrl+d", "ctrl+u":
		return true
	}
	return false
}
