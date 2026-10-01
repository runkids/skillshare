package hooks

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGitCollisionSourcesAndSymlinkInclude(t *testing.T) {
	for _, scope := range []string{"system", "included", "global in project", "worktree"} {
		t.Run(scope, func(t *testing.T) {
			e := gitEnv(t)
			foreign := "[hook \"check\"]\n command = echo foreign\n event = pre-commit\n"
			s := e.service
			switch scope {
			case "system":
				write(t, os.Getenv("GIT_CONFIG_SYSTEM"), foreign)
			case "included":
				included := filepath.Join(e.home, "other.gitconfig")
				write(t, included, foreign)
				write(t, filepath.Join(e.home, ".gitconfig"), includeLines(included))
			default:
				root := t.TempDir()
				gitSetup(t, root, "init", "--quiet")
				s = e.project(root)
				if scope == "global in project" {
					write(t, filepath.Join(e.home, ".gitconfig"), foreign)
				} else {
					write(t, filepath.Join(root, ".git", "config"), "[core]\n repositoryformatversion = 0\n[extensions]\n worktreeConfig = true\n")
					write(t, filepath.Join(root, ".git", "config.worktree"), foreign)
				}
			}
			p, err := s.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, gitEntry), Replace: true})
			must(t, err)
			if !p.Blocked {
				t.Fatalf("cannot replace %s collision: %+v", scope, p.Changes)
			}
		})
	}
	t.Run("symlink", func(t *testing.T) {
		e := gitEnv(t)
		referent := filepath.Join(e.home, "dotfiles", "config")
		write(t, referent, "[user]\n name = private\n")
		must(t, os.Symlink(referent, filepath.Join(e.home, ".gitconfig")))
		p, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, gitEntry)})
		must(t, err)
		if p.Blocked || !strings.Contains(actions(p), "inactive") {
			t.Fatalf("symlink: %+v", p.Changes)
		}
		for _, f := range p.files {
			if f.kind == kindGitInclude && f.changed() {
				t.Fatal("must not write through symlink")
			}
		}
		if read(t, referent) != "[user]\n name = private\n" {
			t.Fatal("preview wrote user config")
		}
	})
}

func TestGitSectionEditorOracle(t *testing.T) {
	e := gitEnv(t)
	config := "[user]\n name = private\n[hook \"check\"]\n command = echo one\n unknown\n unknown = two\n[remote \"origin\"]\n url = private\n[hook \"check\"]\n event = pre-commit\n"
	after, fragments, err := e.service.removeGitSection([]byte(config), "check")
	must(t, err)
	if len(fragments) != 2 || string(after) != "[user]\n name = private\n[remote \"origin\"]\n url = private\n" {
		t.Fatalf("narrow section removal: %s / %+v", after, fragments)
	}
	for _, f := range fragments {
		if strings.Contains(f.Text, "private") {
			t.Fatal("private setting in backup fragment")
		}
	}
	if _, _, err := e.service.removeGitSection([]byte("[hook \"check\"] command = echo one\n"), "check"); err == nil {
		t.Fatal("unsupported syntax must fail closed")
	}
}

func TestGitSectionRestoreRepeatedOrder(t *testing.T) {
	e := gitEnv(t)
	for _, prefix := range []string{"", "[user]\n name = test\n"} {
		original := prefix + "[hook \"check\"]\n command = echo one\n[hook \"check\"]\n command = echo two\n"
		after, fragments, err := e.service.removeGitSection([]byte(original), "check")
		must(t, err)
		restored, err := e.service.restoreGitSection(after, gitSectionOp{Name: "check", Fragments: fragments, Restore: true})
		must(t, err)
		if string(restored) != original {
			t.Fatalf("repeated fragment order changed:\n%s", restored)
		}
	}
}

func TestGitSectionDottedSibling(t *testing.T) {
	e := gitEnv(t)
	original := "[hook \"check\"]\n command = echo check\n[hook \"check.lint\"]\n command = echo lint\n"
	after, fragments, err := e.service.removeGitSection([]byte(original), "check")
	must(t, err)
	if string(after) != "[hook \"check.lint\"]\n command = echo lint\n" {
		t.Fatal("dotted sibling was removed")
	}
	restored, err := e.service.restoreGitSection(after, gitSectionOp{Name: "check", Fragments: fragments, Restore: true})
	must(t, err)
	if string(restored) != original {
		t.Fatalf("dotted sibling restore: %s", restored)
	}
	write(t, filepath.Join(e.home, ".gitconfig"), original)
	save(t, e.service, Mutation{Name: "guard", Entry: entry(t, gitEntry), Replace: true})
	if !strings.Contains(read(t, filepath.Join(e.home, ".gitconfig")), "echo lint") {
		t.Fatal("apply changed dotted sibling")
	}
}

func TestGitConfigOperationsHaveVisibleChanges(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	write(t, d.includeTarget, "[hook \"check\"]\n command = echo foreign\n event = pre-commit\n[credential]\n helper = SECRET\n")
	p, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, gitEntry), Replace: true})
	must(t, err)
	for _, diff := range p.Files() {
		if diff.Path != d.includeTarget {
			continue
		}
		found := false
		for _, change := range p.Changes {
			found = found || change.Path == diff.Path && change.Target == "git"
		}
		if !found {
			t.Fatalf("config operation hidden from changes: %+v", diff)
		}
		if strings.Contains(diff.Before+diff.After, "SECRET") {
			t.Fatal("private config exposed")
		}
	}
}

func TestGitNestedConditionalIncludeDoesNotWidenScope(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	group := filepath.Join(e.home, "group.gitconfig")
	write(t, group, includeLines(d.includeValue))
	write(t, d.includeTarget, "[includeIf \"gitdir:/never/\"]\n path = "+gitQuote(group)+"\n")
	p, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	must(t, err)
	for _, f := range p.files {
		if f.kind == kindGitInclude && f.changed() {
			t.Fatal("nested conditional include would be widened to all repositories")
		}
	}
}

func TestGitInactiveIncludeGraphPreservesLexicalSymlinkDirectory(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	real := filepath.Join(e.home, "elsewhere", "group.gitconfig")
	alias := filepath.Join(e.home, "group.gitconfig")
	write(t, real, "[include]\n path = nested.gitconfig\n")
	must(t, os.Symlink(real, alias))
	write(t, filepath.Join(e.home, "nested.gitconfig"), includeLines(d.includeValue))
	write(t, d.includeTarget, "[includeIf \"gitdir:/never/\"]\n path = "+gitQuote(alias)+"\n")
	p, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	must(t, err)
	for _, f := range p.files {
		if f.kind == kindGitInclude && f.changed() {
			t.Fatal("lexical include directory lost, widening a conditional include")
		}
	}
}

func TestGitConditionalParentChangeInvalidatesPreview(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	group := filepath.Join(e.home, "group.gitconfig")
	write(t, group, includeLines(d.includeValue))
	write(t, d.includeTarget, "[includeIf \"gitdir:/never/\"]\n path = "+gitQuote(group)+"\n")
	p := gitDraft(t, e, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	write(t, group, "[user]\n name = unrelated\n")
	q, err := e.service.Preview()
	must(t, err)
	if p.Revision == q.Revision || p.Fingerprint == q.Fingerprint {
		t.Fatal("nested include edit absent from public preview tokens")
	}
	_, err = e.service.applyScoped(p, nil)
	if !errors.Is(err, ErrStaleRevision) || pathExists(d.hooksFile) {
		t.Fatalf("conditional parent edit did not stop writes: %v", err)
	}
}

func TestGitCrossScopeGuardRetainsUnrelatedObservations(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	snapshot, err := e.service.gitSnapshot(d)
	must(t, err)
	guard := &gitGuard{service: e.service, destination: d, expected: snapshot}
	write(t, d.includeTarget, includeLines(d.includeValue)+"[hook \"other\"]\n command = echo changed\n")
	f := gitIncludePlan(e.service, d, nil, d.includeValue, true)
	if err := guard.completed(f); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("own include write masked a foreign hook edit: %v", err)
	}
}

func TestGitNamedHomeIncludeKeepsConditionalScope(t *testing.T) {
	e := gitEnv(t)
	d, err := e.service.gitDestination("")
	must(t, err)
	current, err := user.Current()
	must(t, err)
	relative, err := filepath.Rel(current.HomeDir, d.hooksFile)
	must(t, err)
	value := "~" + current.Username + "/" + filepath.ToSlash(relative)
	write(t, d.includeTarget, "[includeIf \"gitdir:/never/\"]\n path = "+gitQuote(value)+"\n")
	resolved := gitSetup(t, e.home, "config", "--file", d.includeTarget, "--path", "--get-regexp", `^includeif\..*\.path$`)
	_, resolved, ok := strings.Cut(resolved, " ")
	if !ok || !sameGitPath(resolved, d.hooksFile) {
		t.Fatal("native Git did not resolve named-user include to the isolated output")
	}
	p, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	must(t, err)
	for _, f := range p.files {
		if f.kind == kindGitInclude && f.changed() {
			t.Fatal("named-user Git include widened to all repositories")
		}
	}
}

type gitPathReadMutation struct {
	GitRunner
	before func()
}

func (g *gitPathReadMutation) Run(dir string, stdin []byte, args ...string) ([]byte, error) {
	if g.before != nil && slices.Contains(args, "--path") {
		before := g.before
		g.before = nil
		before()
	}
	return g.GitRunner.Run(dir, stdin, args...)
}

func TestGitIncludeExpansionRejectsInterveningValueChange(t *testing.T) {
	e := gitEnv(t)
	path := filepath.Join(e.home, ".gitconfig")
	write(t, path, includeLines("~/before.gitconfig"))
	e.service.Git = &gitPathReadMutation{GitRunner: e.service.gitRunner(), before: func() {
		write(t, path, includeLines("~/after.gitconfig"))
	}}
	_, err := e.service.gitFileIncludes(path)
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("mixed raw and expanded include observations were accepted: %v", err)
	}
}
