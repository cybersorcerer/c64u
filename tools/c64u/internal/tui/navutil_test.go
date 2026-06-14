package tui

import "testing"

func TestApplyListNav(t *testing.T) {
	tests := []struct {
		key        string
		cursor     int
		count      int
		page       int
		wantCursor int
	}{
		{"j", 0, 10, 4, 1},
		{"k", 5, 10, 4, 4},
		{"k", 0, 10, 4, 0},
		{"j", 9, 10, 4, 9},
		{"g", 7, 10, 4, 0},
		{"G", 2, 10, 4, 9},
		{"ctrl+d", 0, 10, 4, 2},
		{"ctrl+u", 9, 10, 4, 7},
	}
	for _, tt := range tests {
		got := applyListNav(tt.key, tt.cursor, tt.count, tt.page)
		if got != tt.wantCursor {
			t.Errorf("applyListNav(%q, cursor=%d, count=%d, page=%d) = %d, want %d",
				tt.key, tt.cursor, tt.count, tt.page, got, tt.wantCursor)
		}
	}
}

func TestApplyListNav_UnknownKeyReturnsSame(t *testing.T) {
	if got := applyListNav("x", 3, 10, 4); got != 3 {
		t.Errorf("unknown key should return same cursor, got %d", got)
	}
}

func TestApplyListNav_EmptyList(t *testing.T) {
	if got := applyListNav("j", 0, 0, 4); got != 0 {
		t.Errorf("empty list should keep cursor 0, got %d", got)
	}
}
