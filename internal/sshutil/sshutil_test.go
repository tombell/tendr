package sshutil

import (
	"context"
	"reflect"
	"testing"
)

func TestCommandTargets(t *testing.T) {
	tests := []struct {
		target string
		want   []string
	}{
		{"workbox", []string{"ssh", "workbox", "herdr status"}},
		{"ssh://you@server.example.com:2222", []string{"ssh", "-p", "2222", "you@server.example.com", "herdr status"}},
	}
	for _, test := range tests {
		command, err := Command(context.Background(), test.target, "herdr status")
		if err != nil {
			t.Fatalf("Command(%q) error = %v", test.target, err)
		}
		if !reflect.DeepEqual(command.Args, test.want) {
			t.Errorf("Command(%q).Args = %#v, want %#v", test.target, command.Args, test.want)
		}
	}
}

func TestCommandRejectsNonSSHURL(t *testing.T) {
	if _, err := Command(context.Background(), "https://server", "herdr status"); err == nil {
		t.Fatal("Command() error = nil")
	}
}
