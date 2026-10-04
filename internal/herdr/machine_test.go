package herdr

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestResolveMachineMatchesHerdrRules(t *testing.T) {
	client := New("", nil)
	client.commandRunner = func(_ context.Context, session string, args []string) ([]byte, error) {
		if session != "" || !reflect.DeepEqual(args, []string{"machine", "list", "--json"}) {
			t.Fatalf("unexpected catalog request: session = %q, args = %q", session, args)
		}
		return []byte(`[
  {"id":"ssh_build","label":"Build machine","target":"workbox","session":"saved","enabled":true,"selected":false},
  {"id":"ssh_disabled","label":"Disabled","target":"offline","enabled":false},
  {"id":"ssh_one","label":"Duplicate","target":"one","enabled":true},
  {"id":"ssh_two","label":"Duplicate","target":"two","enabled":false},
  {"id":"ssh_collision","label":"ssh_build","target":"other","enabled":true},
  {"id":"ssh_empty","label":"Empty","enabled":true}
]`), nil
	}
	for _, test := range []struct {
		selector string
		target   string
		wantErr  string
	}{
		{"Build machine", "workbox", ""}, {"ssh_build", "workbox", ""},
		{"build machine", "", "unknown machine"}, {"missing", "", "herdr machine list"},
		{"Disabled", "", "is disabled"}, {"ssh_disabled", "", "is disabled"},
		{"Duplicate", "", "ambiguous"}, {"Empty", "", "has no SSH target"},
	} {
		t.Run(test.selector, func(t *testing.T) {
			machine, err := client.ResolveMachine(context.Background(), test.selector)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("ResolveMachine() error = %v, want %q", err, test.wantErr)
				}
			} else if err != nil || machine.Target != test.target {
				t.Fatalf("ResolveMachine() = %+v, %v", machine, err)
			}
		})
	}
}

func TestListMachinesReportsCatalogErrors(t *testing.T) {
	for _, response := range []string{"not json", `{"machines":[]}`} {
		client := New("", nil)
		client.commandRunner = func(context.Context, string, []string) ([]byte, error) { return []byte(response), nil }
		if _, err := client.ListMachines(context.Background()); err == nil || !strings.Contains(err.Error(), "parse Herdr machines") {
			t.Fatalf("ListMachines() error = %v", err)
		}
	}
	want := errors.New("catalog unreadable")
	client := New("", nil)
	client.commandRunner = func(context.Context, string, []string) ([]byte, error) { return nil, want }
	if _, err := client.ResolveMachine(context.Background(), "Build machine"); !errors.Is(err, want) {
		t.Fatalf("ResolveMachine() error = %v", err)
	}
}
