# git-rewind

**Undo any Git mistake — with an automatic backup, and the exact commands shown before anything runs.**

[![CI](https://github.com/neomikhe/git-rewind/actions/workflows/ci.yml/badge.svg)](https://github.com/neomikhe/git-rewind/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/neomikhe/git-rewind.svg)](https://pkg.go.dev/github.com/neomikhe/git-rewind)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

You ran `git reset --hard`. Or you amended the wrong commit, deleted a branch that still had
work on it, or finished a rebase and watched an afternoon disappear.

Your work is almost certainly still there. Git keeps it — in the reflog, in unreachable
objects — but getting it back means reading `HEAD@{3}`, decoding `git fsck --lost-found`,
and typing another `reset --hard` while already rattled. That is a bad moment to be
improvising with a destructive command.

`git-rewind` reads that state for you, explains it in plain language, and offers the rescue
as a preview you approve.

---

## The ten-second rescue

```console
$ git reset --hard HEAD~1        # ...and there goes the whole afternoon

$ git rewind last
Rescue: Recover commits discarded by reset --hard

Will run:
  git reset --hard 934d4e82924dd15261bc6d74c5bff45024a0dc6d

  Moves your branch back to the recovered commit, replacing the current state (which is
  saved to the backup branch first). Any uncommitted changes are discarded.

Dry run. Re-run with --yes to apply it; a backup branch is always created first.

$ git rewind last --yes
Rescue: Recover commits discarded by reset --hard

Will run:
  git reset --hard 934d4e82924dd15261bc6d74c5bff45024a0dc6d

Done. The previous state is saved on branch backup/rewind-20260817-164646.
```

Nothing ran until you asked for it, you saw the exact command first, and the state you were
in before the rescue is still on a branch in case the rescue itself was the mistake.

## The interactive timeline

Run `git rewind` with no arguments and you get a navigable history of what actually happened
to the repository, with a risk level on every event:

```
git-rewind timeline (3 events)

> HEAD@{0}      2m  [red]    945a801  Reset the branch to HEAD~1 (1 commit left unreachable, recoverable)
  HEAD@{1}      5m  [green]  e205c73  Committed "second commit"
  HEAD@{2}      6m  [green]  945a801  Made the first commit "first commit"

up/down, j/k: move  |  enter: details  |  q, esc: quit  |  ?: help
```

Red, yellow and green are colours in your terminal too — but the label is always there, so
the risk is readable without them.

Press `enter` on an event to see what it did and what it left behind:

```
Event HEAD@{0}

  When     2026-08-17 16:44:00 UTC (2m ago)
  Kind     reset
  Risk     red
  Commit   945a801
  Who      Ada Lovelace
  What     Reset the branch to HEAD~1 (1 commit left unreachable, recoverable)
  Reflog   reset: moving to HEAD~1

  Commits left unreachable but still recoverable
    e205c73

enter: rescues  |  esc: back  |  q: quit  |  ?: help
```

Press `enter` again for the rescues that apply, and once more for the confirmation panel —
the real commands, one explanation per line, and every warning the rescue carries:

```
Rescue: Recover commits discarded by reset --hard

Will run:
  git reset --hard e205c733abdb3eac0c074db65389c8717ff69f45
      move the branch back onto the recovered commit e205c73

  Your current state is saved to a backup branch before anything runs.

  Moves your branch back to the recovered commit, replacing the current state (which is
  saved to the backup branch first). Any uncommitted changes are discarded.

y: apply  |  esc: back  |  q: quit  |  ?: help
```

`?` opens a help screen for whichever screen you are on, listing exactly the keys that work
there. On a very large repository the timeline loads the most recent events first and offers
`m` to load older ones.

## What state am I actually in?

When you are not sure anything is wrong, `git rewind explain` says so in four lines:

```console
$ git rewind explain
Repository state

  HEAD          on branch main at 9665e14
  Working tree  1 uncommitted change — a backup branch does not preserve these
  Last event    just now  Reset the branch to HEAD~1 (1 commit left unreachable, recoverable)
  Unreachable   1 commit no branch or tag reaches, still recoverable

Something can be undone: Recover commits discarded by reset --hard
  "git rewind" reviews it interactively; "git rewind last" prints the exact commands.
  "git rewind find <text>" searches the unreachable commits for lost work.
```

## Finding work you cannot even name

Sometimes you do not know *which* commit you lost — only that it had a function in it. `git
rewind find` searches every commit no branch or tag reaches, through both commit messages and
the file contents at that commit:

```console
$ git rewind find "parseInvoiceTotal"
Found 1 commit matching "parseInvoiceTotal", out of 1 commit no branch or tag reaches.

  3afd6f3  2026-08-14 16:02  Ada Lovelace  "add the invoice parser I spent all afternoon on"
      billing.go:3  func parseInvoiceTotal(raw string) (int, error) {
      keep it with: git branch rescued/3afd6f3 3afd6f35362bccb63bd89457fdd133230e72f0f9

That command only adds a branch pointing at the commit; nothing else changes.
```

The query is matched literally, so `parseInvoiceTotal()` searches for that text rather than
being read as a regular expression. `--messages` restricts the search to commit messages,
which is faster on very large repositories.

## Install

```bash
go install github.com/neomikhe/git-rewind/cmd/git-rewind@latest
```

Or build it yourself:

```bash
git clone https://github.com/neomikhe/git-rewind
cd git-rewind
go build -o git-rewind ./cmd/git-rewind
```

Put the binary anywhere on your `PATH`. Because Git dispatches `git <name>` to an executable
called `git-<name>`, **`git rewind` then works as a native subcommand** — no aliases, no
configuration.

The first tagged release will also publish archives for Linux, macOS and Windows on amd64
and arm64, with a `checksums.txt` (SHA-256) to verify them against, plus Homebrew and Scoop
packages. Until then, the two commands above are the way in.

**Requirements:** `git` on your `PATH` at runtime, and Go 1.26+ if you build from source.
Linux, macOS and Windows are all supported and all tested in CI.

## Usage

| Command | What it does |
|---|---|
| `git rewind` | Interactive timeline: browse events, open a rescue, approve it. |
| `git rewind last` | Print the rescue for the most recent mistake. Changes nothing. |
| `git rewind last --yes` | Apply that rescue. A backup branch is created first. |
| `git rewind last --yes --force` | Also allow it when the rescue would discard uncommitted changes. |
| `git rewind find "<text>"` | Search unreachable commits by message and file contents. Read-only. |
| `git rewind explain` | Diagnose the repository right now: HEAD, working tree, last event, what is recoverable. Read-only. |

`last`, `find` and `explain` all accept `--json` for scripting. The document carries a
`schema` field so a consumer can tell when the shape changes, errors still go to stderr with
a non-zero exit, and `--json` never changes what a command *does* — a dry run stays a dry
run. Flags may come before or after the search text.

```console
$ git rewind explain --json | jq '.rescue.name, .unreachableCommits'
"recover-after-reset-hard"
1
```
| `git rewind version` | Print the version, commit and platform — worth including in bug reports. |

## What it can rescue

| Rescue | When it applies |
|---|---|
| Recover commits discarded by `reset --hard` | A reset stranded a commit that still exists |
| Undo the last amend | An amend replaced a commit that is still recoverable |
| Undo a rebase | A rebase rewrote the branch and the old tip survives |
| Undo a merge | `HEAD` is the merge commit you just made |
| Restore a deleted branch | A branch you left no longer exists, but its tip does |
| Restore a dropped stash | A `git stash drop` left a stash that git has not collected yet |
| Undo the last commit, keeping the changes | `reset --soft`, changes stay staged |
| Undo the last commit, discarding the changes | `reset --hard`, with the warning it deserves |

Each one ships with an integration test that builds a genuinely broken repository, runs the
rescue, and verifies the repository came back.

## Is it safe?

This is a tool that runs destructive Git commands on your repository, so the honest answer
matters more than a reassuring one. Four guarantees, each enforced in code rather than by
convention:

- **Dry-run by default.** Detecting a rescue never changes anything. The commands are
  printed before a single one can run, and applying them takes an explicit `--yes` or a
  keypress in the confirmation panel.
- **A backup branch, always.** Before any destructive operation, the current state is saved
  to `backup/rewind-<timestamp>`. `--yes` skips the prompt; there is no flag, anywhere, that
  skips the backup. If a rescue turns out to be the wrong call, your previous state is a
  branch away.
- **Uncommitted changes are never touched silently.** A backup branch preserves commits, not
  your working tree — so any rescue that would overwrite uncommitted work is *refused* on a
  dirty tree until you explicitly force it.
- **Nothing here reimplements Git.** Every operation shells out to your own `git` binary
  using plumbing commands and stable `--format` output. There is no bespoke object-database
  code that could corrupt a repository.

Backing that up: **8 rescues, 7 reproducible broken-repository fixtures**, and a test suite
that builds real repositories in real breakage — `reset --hard`, amend, deleted branch,
rewriting rebase, merge, detached HEAD, dropped stash — and asserts each rescue actually
fixes them. CI runs the whole suite on Linux, macOS and Windows on every push, with the race
detector on Linux and macOS.

If you find a case where a rescue does the wrong thing, that is the most valuable issue you
can open.

## How it works

```
git reflog ──┐
             ├─▶ classified timeline ─▶ a rescue recipe ─▶ a plan ─▶ preview ─▶ backup ─▶ apply
git fsck ────┘   (what, when, risk)      (detect only)     (commands
                                                            + warnings)
```

Reflog entries are parsed from a fixed `--format` string, classified by operation and by how
dangerous they are to committed work, and cross-referenced with unreachable objects so each
history-rewriting event carries the commit it stranded. A *recipe* inspects that state and
returns a plan; recipes never execute anything. A separate safety layer creates the backup
and runs the plan. The interactive and non-interactive front ends share all of it, so they
cannot drift apart in how careful they are.

## Compared with doing it by hand

`git-rewind` runs the same commands you would. The difference is everything around them:

| | By hand | With `git-rewind` |
|---|---|---|
| Working out what happened | Read `git reflog` and decode `HEAD@{3}` | Plain-language timeline with risk levels |
| Finding lost commits | `git fsck --lost-found`, then inspect hashes | Shown attached to the event that stranded them |
| A safety net | You remember to branch first — or you don't | A backup branch, created automatically, every time |
| Knowing what will run | You type it, under pressure | Shown and explained before anything runs |
| Uncommitted changes | `reset --hard` takes them with it | Refused unless you explicitly force it |
| Learning Git | Copy an answer, forget it by next month | Every rescue shows the real commands and why |

**Prior art:** `git-rewind` is not the first tool for undoing Git operations —
[`ugit`](https://github.com/Bhupesh-V/ugit) is well worth knowing about. The angle here is
the contextual timeline (what happened, how risky it was, what it left recoverable), the
mandatory backup branch, and showing you the underlying Git so you need the tool less over
time.

## Contributing

The `Recipe` interface is deliberately small, and new rescues are the most useful
contribution:

```go
type Recipe interface {
    Name() string
    Title() string
    Detect(ctx context.Context, repo *Repo) (*safety.Plan, error)
}
```

To add one: write a fixture in `internal/scenario` that reproduces the mistake, implement
`Detect` to return a plan (or a nil plan when it does not apply), register it in `All()`, and
add an integration test that builds the broken repository, applies the rescue and verifies
the result. A rescue without that test will not be merged — the test suite is the reason
anyone should trust this tool with their repository.

```bash
go test ./...            # the full suite
golangci-lint run ./...  # lint (gosec included)
```

## Status

Pre-release and under active development. The engine, the rescues and both front ends work
and are tested; the first tagged release, packaged binaries and colour output are next.

## License

[MIT](LICENSE).

---

## Español

**Deshaz cualquier error de Git — con copia de seguridad automática y los comandos exactos a la vista antes de ejecutar nada.**

Has hecho `git reset --hard`, o un `amend` sobre el commit equivocado, o has borrado una rama
que todavía tenía trabajo. Tu trabajo casi con toda seguridad sigue ahí, pero recuperarlo
implica leer el `reflog`, descifrar `git fsck` y escribir otro comando destructivo justo en
el peor momento para improvisar.

`git-rewind` lee ese estado por ti, lo explica en lenguaje llano y te ofrece el rescate como
una vista previa que tú apruebas.

**Instalación**

```bash
go install github.com/neomikhe/git-rewind/cmd/git-rewind@latest
```

Con el binario en el `PATH`, Git lo invoca como subcomando nativo: `git rewind`.

**Uso**

- `git rewind` — línea de tiempo interactiva: navega los eventos, abre un rescate, apruébalo.
- `git rewind last` — muestra el rescate del último error. No cambia nada.
- `git rewind last --yes` — lo aplica, creando antes una rama de backup.

**¿Es seguro?**

- **Simulación por defecto.** Detectar un rescate nunca modifica nada; los comandos se
  imprimen antes de que pueda ejecutarse ninguno.
- **Siempre hay copia de seguridad.** Antes de cualquier operación destructiva se guarda el
  estado actual en `backup/rewind-<timestamp>`. No existe ninguna opción que se salte ese
  backup.
- **Nunca se tocan los cambios sin commitear en silencio.** Un rescate que sobrescribiría el
  working tree se rechaza si hay cambios pendientes, salvo que lo fuerces explícitamente.
- **No se reimplementa Git.** Todo se delega en tu propio binario `git`.

Respaldándolo: **7 rescates y 7 repositorios rotos reproducibles**, con tests de integración
que rompen un repositorio de verdad y comprueban que el rescate lo arregla. CI en Linux,
macOS y Windows.

La interfaz de la herramienta está en inglés por ahora; la traducción al español está
planificada para la v1.0.
