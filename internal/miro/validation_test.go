package miro

import (
	"strings"
	"testing"
)

func TestValidateIDAcceptsTypicalMiroIDs(t *testing.T) {
	tests := []string{
		"uXjVO_abc",        // board ID shape
		"3458764612345678", // decimal item ID
		"abc-123_xyz.AB",   // mixed alphanumeric + safe punctuation
	}
	for _, v := range tests {
		if err := ValidateID("board_id", v); err != nil {
			t.Errorf("ValidateID(%q) rejected valid input: %v", v, err)
		}
	}
}

func TestValidateIDRejectsEmpty(t *testing.T) {
	err := ValidateID("board_id", "")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
	if !strings.Contains(err.Error(), "board_id") {
		t.Errorf("error should name the field: %q", err)
	}
}

func TestValidateIDRejectsPathTraversal(t *testing.T) {
	tests := []string{
		"..",
		"../etc/passwd",
		"foo/../bar",
		"foo..bar",
	}
	for _, v := range tests {
		if err := ValidateID("board_id", v); err == nil {
			t.Errorf("ValidateID(%q) should have rejected traversal", v)
		}
	}
}

func TestValidateIDRejectsSlash(t *testing.T) {
	if err := ValidateID("board_id", "abc/def"); err == nil {
		t.Error("expected rejection of '/'")
	}
}

func TestValidateIDRejectsWhitespace(t *testing.T) {
	tests := []string{
		"abc def",
		"abc\tdef",
		"abc\ndef",
		" leading",
		"trailing ",
	}
	for _, v := range tests {
		if err := ValidateID("board_id", v); err == nil {
			t.Errorf("ValidateID(%q) should have rejected whitespace", v)
		}
	}
}

func TestValidateIDRejectsControlChars(t *testing.T) {
	if err := ValidateID("board_id", "abc\x01def"); err == nil {
		t.Error("expected rejection of control char")
	}
	if err := ValidateID("board_id", "abc\x00def"); err == nil {
		t.Error("expected rejection of null byte")
	}
}

func TestValidateIDRejectsOverlyLongInput(t *testing.T) {
	tooLong := strings.Repeat("a", MaxIDLength+1)
	if err := ValidateID("board_id", tooLong); err == nil {
		t.Error("expected rejection of overlong id")
	}
}

func TestValidatePathBackstop(t *testing.T) {
	good := []string{
		"/v2/boards/abc",
		"/v2/boards/abc/items/123",
	}
	for _, p := range good {
		if err := validatePath(p); err != nil {
			t.Errorf("validatePath(%q) rejected valid path: %v", p, err)
		}
	}
	bad := []string{
		"/v2/boards/../secret",
		"/v2/../boards/abc",
		"/v2/boards/abc\x00",
	}
	for _, p := range bad {
		if err := validatePath(p); err == nil {
			t.Errorf("validatePath(%q) should have rejected", p)
		}
	}
}
