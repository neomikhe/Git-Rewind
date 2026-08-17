package tui

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/neomikhe/git-rewind/core/gitexec"
	"github.com/neomikhe/git-rewind/core/recipes"
	"github.com/neomikhe/git-rewind/core/safety"
	"github.com/neomikhe/git-rewind/core/timeline"
	"github.com/neomikhe/git-rewind/internal/scenario"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func discardingRescue() rescue {
	return rescue{
		recipe: recipes.RecoverAfterResetHard{},
		plan: &safety.Plan{
			Commands: []safety.Command{{
				Args:    []string{"reset", "--hard", orphanHash},
				Explain: "move the branch back onto the recovered commit",
			}},
			Warnings:        []string{"Any uncommitted changes are discarded."},
			DiscardsChanges: true,
		},
	}
}

func safeRescue() rescue {
	return rescue{
		recipe: recipes.UndoLastCommit{},
		plan: &safety.Plan{
			Commands: []safety.Command{{
				Args:    []string{"reset", "--soft", "HEAD~1"},
				Explain: "move HEAD back one commit; the changes stay staged",
			}},
		},
	}
}

func atRescues(t *testing.T, rescues []rescue, dirty bool) model {
	t.Helper()
	m := update(newModel(sampleSession()), key("enter"))
	if m.screen != screenDetail {
		t.Fatalf("screen = %v after enter, want detail", m.screen)
	}
	return update(m, rescuesMsg{rescues: rescues, dirty: dirty})
}

func TestEnterOpensEventDetail(t *testing.T) {
	m := update(newModel(sampleSession()), key("enter"))
	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want detail", m.screen)
	}

	out := m.View()
	for _, want := range []string{
		"Event HEAD@{0}",
		"reset",
		"red",
		"Reset the branch to HEAD~1",
		"reset: moving to HEAD~1",
		"Commits left unreachable but still recoverable",
		shortHash(orphanHash),
		"enter: rescues",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("detail view is missing %q\n---\n%s", want, out)
		}
	}
}

func TestEscReturnsFromDetailToTimeline(t *testing.T) {
	m := update(newModel(sampleSession()), key("enter"))
	m = update(m, key("esc"))
	if m.screen != screenTimeline {
		t.Fatalf("screen = %v after esc, want timeline", m.screen)
	}
}

func TestRescuesScreenListsApplicableRescues(t *testing.T) {
	m := atRescues(t, []rescue{discardingRescue(), safeRescue()}, false)
	if m.screen != screenRescues {
		t.Fatalf("screen = %v, want rescues", m.screen)
	}

	out := m.View()
	for _, want := range []string{
		"Available rescues (2)",
		"Recover commits discarded by reset --hard",
		"Undo the last commit (keep the changes)",
		"enter: review",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rescues view is missing %q\n---\n%s", want, out)
		}
	}
}

func TestRescuesScreenWhenNothingApplies(t *testing.T) {
	m := atRescues(t, nil, false)
	if !strings.Contains(m.View(), "No rescue applies") {
		t.Errorf("rescues view should say nothing applies\n---\n%s", m.View())
	}
	if _, cmd := m.Update(key("enter")); cmd != nil {
		t.Error("enter with no rescues should not start anything")
	}
}

func TestConfirmShowsExactCommandsAndExplanations(t *testing.T) {
	m := atRescues(t, []rescue{discardingRescue()}, false)
	m = update(m, key("enter"))
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want confirm", m.screen)
	}

	out := m.View()
	for _, want := range []string{
		"Rescue: Recover commits discarded by reset --hard",
		"git reset --hard " + orphanHash,
		"move the branch back onto the recovered commit",
		"saved to a backup branch before anything runs",
		"Any uncommitted changes are discarded.",
		"y: apply",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("confirm view is missing %q\n---\n%s", want, out)
		}
	}
}

func TestConfirmAppliesOnYesWhenTreeIsClean(t *testing.T) {
	m := atRescues(t, []rescue{safeRescue()}, false)
	m = update(m, key("enter"))

	next, cmd := m.Update(key("y"))
	if cmd == nil {
		t.Fatal("y on a clean tree produced no command, want apply")
	}
	if !next.(model).loading {
		t.Error("model should be loading while the rescue runs")
	}
}

func TestConfirmRequiresForceWhenTreeIsDirty(t *testing.T) {
	m := atRescues(t, []rescue{discardingRescue()}, true)
	m = update(m, key("enter"))

	if !strings.Contains(m.View(), "f: apply, discarding changes") {
		t.Errorf("confirm view should require force on a dirty tree\n---\n%s", m.View())
	}
	if strings.Contains(m.View(), "y: apply") {
		t.Errorf("confirm view must not offer y on a dirty tree\n---\n%s", m.View())
	}

	refused, cmd := m.Update(key("y"))
	if cmd != nil {
		t.Fatal("y must not apply a plan that would discard uncommitted changes")
	}
	if !errors.Is(refused.(model).err, errDirtyTree) {
		t.Errorf("err = %v, want errDirtyTree", refused.(model).err)
	}

	forced, cmd := m.Update(key("f"))
	if cmd == nil {
		t.Fatal("f must apply the plan on a dirty tree")
	}
	if !forced.(model).loading {
		t.Error("model should be loading after f")
	}
}

func TestForceIsIgnoredWhenNothingWouldBeDiscarded(t *testing.T) {
	m := atRescues(t, []rescue{safeRescue()}, false)
	m = update(m, key("enter"))

	if _, cmd := m.Update(key("f")); cmd != nil {
		t.Error("f should do nothing when the plan discards nothing; y is the confirm key")
	}
}

func TestResultShowsBackupBranchAndCommands(t *testing.T) {
	m := atRescues(t, []rescue{safeRescue()}, false)
	m = update(m, key("enter"))
	m = update(m, appliedMsg{result: safety.Result{
		BackupBranch: "backup/rewind-20260817-120000",
		Commands:     []string{"git reset --soft HEAD~1"},
	}})

	if m.screen != screenResult {
		t.Fatalf("screen = %v, want result", m.screen)
	}
	out := m.View()
	for _, want := range []string{"Done", "backup/rewind-20260817-120000", "git reset --soft HEAD~1"} {
		if !strings.Contains(out, want) {
			t.Errorf("result view is missing %q\n---\n%s", want, out)
		}
	}
}

func TestDetectFailureKeepsUserOnDetail(t *testing.T) {
	m := update(newModel(sampleSession()), key("enter"))
	m = update(m, rescuesMsg{err: errors.New("git exploded")})

	if m.screen != screenDetail {
		t.Fatalf("screen = %v after a failed detection, want detail", m.screen)
	}
	if !strings.Contains(m.View(), "git exploded") {
		t.Errorf("the error should be visible\n---\n%s", m.View())
	}
}

func TestKeysAreIgnoredWhileLoading(t *testing.T) {
	m := update(newModel(sampleSession()), key("enter"))
	m.loading = true

	if next := update(m, key("esc")); next.screen != screenDetail {
		t.Error("navigation must be ignored while a git operation is running")
	}
}

func TestQuitKeysWorkOnEveryScreen(t *testing.T) {
	screens := []model{
		newModel(sampleSession()),
		update(newModel(sampleSession()), key("enter")),
		atRescues(t, []rescue{safeRescue()}, false),
		update(atRescues(t, []rescue{safeRescue()}, false), key("enter")),
	}
	for _, m := range screens {
		for _, k := range []tea.KeyMsg{key("q"), {Type: tea.KeyCtrlC}} {
			_, cmd := m.Update(k)
			if cmd == nil {
				t.Fatalf("screen %v: key %q produced no command, want quit", m.screen, k.String())
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("screen %v: key %q produced %T, want tea.QuitMsg", m.screen, k.String(), cmd())
			}
		}
	}
}

func TestEscQuitsFromTheTimeline(t *testing.T) {
	_, cmd := newModel(sampleSession()).Update(key("esc"))
	if cmd == nil {
		t.Fatal("esc on the timeline produced no command, want quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("esc produced %T, want tea.QuitMsg", cmd())
	}
}

func TestHelpOpensClosesAndIsContextual(t *testing.T) {
	m := update(newModel(sampleSession()), key("?"))
	if !m.help {
		t.Fatal("? did not open the help")
	}
	out := m.View()
	for _, want := range []string{"Keys", "timeline", "open the event detail", "backup/rewind-"} {
		if !strings.Contains(out, want) {
			t.Errorf("timeline help is missing %q\n---\n%s", want, out)
		}
	}

	if update(m, key("?")).help {
		t.Error("? did not close the help")
	}
	if update(m, key("esc")).help {
		t.Error("esc did not close the help")
	}
}

func TestHelpShowsTheKeysOfTheCurrentScreen(t *testing.T) {
	detail := update(update(newModel(sampleSession()), key("enter")), key("?"))
	if !strings.Contains(detail.View(), "event detail") {
		t.Errorf("help does not name the current screen\n---\n%s", detail.View())
	}
	if !strings.Contains(detail.View(), "list the rescues that apply") {
		t.Errorf("detail help is missing its enter binding\n---\n%s", detail.View())
	}

	confirm := update(update(atRescues(t, []rescue{discardingRescue()}, true), key("enter")), key("?"))
	if !strings.Contains(confirm.View(), "apply and discard your uncommitted changes") {
		t.Errorf("confirm help must offer force on a dirty tree\n---\n%s", confirm.View())
	}
	if strings.Contains(confirm.View(), "apply this rescue") {
		t.Errorf("confirm help must not offer y on a dirty tree\n---\n%s", confirm.View())
	}
}

func TestHelpSwallowsNavigationButNotQuit(t *testing.T) {
	m := update(newModel(sampleSession()), key("?"))

	if moved := update(m, tea.KeyMsg{Type: tea.KeyDown}); moved.cursor != 0 {
		t.Error("the timeline must not scroll behind an open help screen")
	}
	if opened := update(m, key("enter")); opened.screen != screenTimeline {
		t.Error("enter must not navigate behind an open help screen")
	}
	if _, cmd := m.Update(key("q")); cmd == nil {
		t.Error("q must still quit from the help screen")
	}
}

func TestMoreEventsAreOfferedOnlyWhenThePageIsFull(t *testing.T) {
	full := sampleSession()
	full.Limit = len(full.Events)
	m := newModel(full)
	if !m.hasMore() {
		t.Fatal("a full page must offer more events")
	}
	for _, want := range []string{"more available", "m: more"} {
		if !strings.Contains(m.View(), want) {
			t.Errorf("timeline is missing %q\n---\n%s", want, m.View())
		}
	}

	partial := sampleSession()
	partial.Limit = len(partial.Events) + 1
	if newModel(partial).hasMore() {
		t.Error("a partial page means everything is loaded")
	}
	if newModel(sampleSession()).hasMore() {
		t.Error("an unlimited session can never have more to load")
	}
	if strings.Contains(newModel(partial).View(), "m: more") {
		t.Error("the load-more hint must not appear when everything is loaded")
	}
}

func TestLoadMoreKeyRequestsTheNextPageOnlyWhenThereIsOne(t *testing.T) {
	full := sampleSession()
	full.Limit = len(full.Events)

	next, cmd := newModel(full).Update(key("m"))
	if cmd == nil {
		t.Fatal("m did not request more events")
	}
	if !next.(model).loading {
		t.Error("the model should be loading while more events are fetched")
	}

	if _, cmd := newModel(sampleSession()).Update(key("m")); cmd != nil {
		t.Error("m must do nothing when everything is already loaded")
	}
}

func TestEventsMsgReplacesTheLoadedPage(t *testing.T) {
	full := sampleSession()
	full.Limit = len(full.Events)
	m := newModel(full)
	m.loading = true

	grown := sampleSession().Events
	m = update(m, eventsMsg{events: grown, limit: 2 * len(grown)})

	if m.loading {
		t.Error("loading should be cleared once the page arrives")
	}
	if m.session.Limit != 2*len(grown) {
		t.Errorf("Limit = %d, want %d", m.session.Limit, 2*len(grown))
	}
	if m.hasMore() {
		t.Error("a page that came back short means everything is loaded")
	}
}

func TestDetectAndApplyOnBrokenRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	built := buildScenario(t, "reset-hard")
	git := gitexec.New(built.Dir)
	events, err := timeline.Load(context.Background(), git, 0)
	if err != nil {
		t.Fatalf("timeline.Load: %v", err)
	}
	session := Session{Git: git, Events: events}

	detected, ok := detectCmd(session)().(rescuesMsg)
	if !ok {
		t.Fatal("detectCmd did not return a rescuesMsg")
	}
	if detected.err != nil {
		t.Fatalf("detect: %v", detected.err)
	}
	if len(detected.rescues) == 0 {
		t.Fatal("no rescue detected for the reset-hard scenario")
	}
	if got := detected.rescues[0].recipe.Name(); got != "recover-after-reset-hard" {
		t.Fatalf("first rescue = %q, want recover-after-reset-hard", got)
	}
	if detected.dirty {
		t.Error("the freshly built scenario should have a clean working tree")
	}

	applied, ok := applyCmd(session, *detected.rescues[0].plan)().(appliedMsg)
	if !ok {
		t.Fatal("applyCmd did not return an appliedMsg")
	}
	if applied.err != nil {
		t.Fatalf("apply: %v", applied.err)
	}
	if applied.result.BackupBranch == "" {
		t.Error("no backup branch was created")
	}
	if head := revParse(t, git, "HEAD"); head != built.Anchors["lost"] {
		t.Errorf("HEAD = %s after the rescue, want the recovered commit %s", head, built.Anchors["lost"])
	}
	if revParse(t, git, applied.result.BackupBranch) == "" {
		t.Error("the backup branch does not resolve")
	}
}

func buildScenario(t *testing.T, name string) scenario.Built {
	t.Helper()
	for _, s := range scenario.All() {
		if s.Name != name {
			continue
		}
		built, err := s.Build(t.TempDir())
		if err != nil {
			t.Fatalf("build %s: %v", name, err)
		}
		return built
	}
	t.Fatalf("scenario %q not found", name)
	return scenario.Built{}
}

func revParse(t *testing.T, git *gitexec.Runner, ref string) string {
	t.Helper()
	out, err := git.Run(context.Background(), "rev-parse", ref)
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(out)
}
