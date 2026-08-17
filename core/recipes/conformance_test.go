package recipes

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/neomikhe/git-rewind/core/i18n"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/internal/scenario"
)

var kebabCase = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// worktreeDestroying lists the git operations that overwrite tracked files. A plan
// containing one of these must declare DiscardsChanges, or the dirty-tree guard in both
// front ends silently stops protecting the user's uncommitted work.
var worktreeDestroying = []struct {
	command string
	flags   []string
}{
	{"reset", []string{"--hard"}},
	{"checkout", []string{"-f", "--force"}},
	{"restore", []string{"--worktree", "-W", "--staged"}},
	{"clean", []string{"-f", "-fd", "--force"}},
	{"stash", []string{"pop", "apply"}},
}

func TestRecipeNamesAreStableIdentifiers(t *testing.T) {
	for _, r := range All() {
		if !kebabCase.MatchString(r.Name()) {
			t.Errorf("recipe name %q is not lower kebab-case; names are stable identifiers used in --json", r.Name())
		}
	}
}

func TestEveryRecipeTitleDiffersFromItsName(t *testing.T) {
	for _, r := range All() {
		for _, lang := range []i18n.Lang{i18n.EN, i18n.ES} {
			if title := r.Title(i18n.New(lang)); title == r.Name() {
				t.Errorf("recipe %q has no real title in %q; a title is prose for a user, a name is an identifier", r.Name(), lang)
			}
		}
	}
}

// TestEveryPlanHonoursTheSafetyContract runs every recipe against every fixture and checks
// the invariants each returned plan must satisfy. It is the guard that lets a contributed
// recipe be trusted: a plan that discards the working tree without saying so, or that runs a
// command with no explanation, fails here rather than in someone's repository.
func TestEveryPlanHonoursTheSafetyContract(t *testing.T) {
	checked := 0
	for _, s := range scenario.All() {
		built := buildScenario(t, s.Name)
		repo := loadRepo(t, built.Dir)

		for _, r := range All() {
			plan, err := r.Detect(context.Background(), repo)
			if err != nil {
				t.Fatalf("%s on scenario %s: Detect: %v", r.Name(), s.Name, err)
			}
			if plan == nil {
				continue
			}
			checked++
			assertPlanContract(t, s.Name, r, *plan)
		}
	}
	if checked == 0 {
		t.Fatal("no recipe applied to any fixture; the contract was never actually checked")
	}
	t.Logf("checked %d plans across %d fixtures", checked, len(scenario.All()))
}

func assertPlanContract(t *testing.T, fixture string, r Recipe, plan safety.Plan) {
	t.Helper()
	where := r.Name() + " on " + fixture

	if len(plan.Commands) == 0 {
		t.Errorf("%s: returned a plan with no commands", where)
	}
	for i, c := range plan.Commands {
		if len(c.Args) == 0 {
			t.Errorf("%s: command %d has no arguments", where, i)
		}
		if strings.TrimSpace(c.Explain) == "" {
			t.Errorf("%s: command %d has no explanation; every step must teach what it does", where, i)
		}
		for _, arg := range c.Args {
			if strings.TrimSpace(arg) == "" {
				t.Errorf("%s: command %d has an empty argument, which git would misread", where, i)
			}
		}
	}
	if len(plan.Warnings) == 0 {
		t.Errorf("%s: returned a plan with no warning; the user must know what they are agreeing to", where)
	}
	for _, w := range plan.Warnings {
		if strings.TrimSpace(w) == "" {
			t.Errorf("%s: has an empty warning", where)
		}
	}

	if touches := touchesWorkingTree(plan); touches && !plan.DiscardsChanges {
		t.Errorf("%s: overwrites the working tree but does not set DiscardsChanges, "+
			"so the dirty-tree guard would let it destroy uncommitted work: %v", where, plan.Preview())
	}
}

func touchesWorkingTree(plan safety.Plan) bool {
	for _, c := range plan.Commands {
		if len(c.Args) == 0 {
			continue
		}
		for _, danger := range worktreeDestroying {
			if c.Args[0] != danger.command {
				continue
			}
			for _, flag := range danger.flags {
				for _, arg := range c.Args[1:] {
					if arg == flag {
						return true
					}
				}
			}
		}
	}
	return false
}

func TestTouchesWorkingTreeRecognisesTheDangerousCommands(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"reset --hard", []string{"reset", "--hard", "abc"}, true},
		{"reset --soft", []string{"reset", "--soft", "abc"}, false},
		{"checkout --force", []string{"checkout", "--force", "main"}, true},
		{"clean -f", []string{"clean", "-f"}, true},
		{"stash pop", []string{"stash", "pop"}, true},
		{"stash store", []string{"stash", "store", "-m", "x", "abc"}, false},
		{"branch", []string{"branch", "recovered", "abc"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := safety.Plan{Commands: []safety.Command{{Args: c.args, Explain: "x"}}}
			if got := touchesWorkingTree(plan); got != c.want {
				t.Errorf("touchesWorkingTree(%v) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}
