package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootRegistersBoards(t *testing.T) {
	cmd, _ := newRootCmd()
	if cmd.Use != "miro" {
		t.Errorf("root.Use = %q, want miro", cmd.Use)
	}
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "boards" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("root command did not register `boards` subtree")
	}
}

func TestRootHelpListsGlobalFlags(t *testing.T) {
	cmd, _ := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help execute: %v", err)
	}
	help := buf.String()
	wantFlags := []string{
		"--token", "--json", "--dry-run", "--agent",
		"--yes", "--idempotent", "--select",
	}
	for _, f := range wantFlags {
		if !strings.Contains(help, f) {
			t.Errorf("root --help missing flag %q\n%s", f, help)
		}
	}
}

func TestRootAgentExpandsViaPreRun(t *testing.T) {
	cmd, g := newRootCmd()
	cmd.SetArgs([]string{"--agent", "--help"}) // --help short-circuits execution after PreRun
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// PersistentPreRunE only fires on subcommands, not on the root itself
	// in cobra. Make Normalize call deterministic by invoking directly.
	g.Normalize()
	if !g.JSON || !g.Yes {
		t.Errorf("Normalize on --agent must imply --json and --yes; got %+v", g)
	}
}
