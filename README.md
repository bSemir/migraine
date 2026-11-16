# migraine

Detects and fixes colliding SQL migration numbers after git merges and rebases.

![migraine check and fix in a terminal](demo.gif)

## The problem

You add `000002_add_orders`. A teammate adds `000002_add_users`. Their branch
is merged, you rebase, and git is perfectly happy because the filenames differ:

```console
$ ls migrations
000001_init.down.sql        000002_add_orders.down.sql  000002_add_users.down.sql
000001_init.up.sql          000002_add_orders.up.sql    000002_add_users.up.sql
```

Git doesn't see it, CI doesn't see it, production does.

## Install

```bash
go install github.com/bSemir/migraine/cmd/migraine@latest
```

Or download a binary for macOS, Linux or Windows from the
[releases page](https://github.com/bSemir/migraine/releases). It is a single
static binary with no configuration file.

## Usage

`migraine check` reports problems and exits 1 if there are any:

```console
$ migraine check
✗ 000002 collides: add_orders (yours) vs add_users (main)
  fix: 000002_add_orders.{up,down}.sql -> 000003_add_orders.{up,down}.sql
run `migraine fix` to renumber
```

`migraine fix` renumbers only your branch's migrations so they follow the
base branch. Add `--dry-run` to see the plan without touching any file.

```console
$ migraine fix
✗ 000002 collides: add_orders (yours) vs add_users (main)
  renamed: 000002_add_orders.{up,down}.sql -> 000003_add_orders.{up,down}.sql
✓ renumbered 1 migration
```

| Flag         | Default                         | Meaning                                    |
| ------------ | ------------------------------- | ------------------------------------------ |
| `--dir`      | `migrations`                    | Migrations directory                       |
| `--base`     | `main`, falling back to `master` | Ref to compare against, e.g. `origin/main` |
| `--dry-run`  | off                             | `fix` only: print the plan, rename nothing |
| `--no-color` | off                             | Plain output (automatic when not a TTY)    |

Exit codes: `0` clean or fixed, `1` problems found, `2` usage or runtime error.

## CI and hooks

GitHub Actions. The full history is needed so that `origin/main` exists:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
- uses: actions/setup-go@v5
  with:
    go-version: stable
- run: go install github.com/bSemir/migraine/cmd/migraine@latest
- run: migraine check --base origin/main
```

Pre-push hook:

```bash
printf '#!/bin/sh\nexec migraine check\n' > .git/hooks/pre-push && chmod +x .git/hooks/pre-push
```

## How it works

- **Base**: the migrations on the base ref, read with `git ls-tree`. Nothing
  is checked out or fetched.
- **Yours**: every migration in the working tree that the base does not have,
  including uncommitted and untracked files.
- **Problems**: any of your migrations numbered at or below the base's highest
  version, and any two of yours sharing a number.
- **Fix**: your migrations are renumbered to follow the base's highest
  version, keeping their order, their up/down pairs and their zero-padding.
  Renames go through temporary names, so chains like 2→3, 3→4 cannot clobber
  each other, and a failure part-way rolls everything back.

Base-branch migrations are never renamed, and numbering gaps are left alone.
The renames are plain filesystem renames; `git add` them and git records
them as renames.

## Supported formats

| Format                                                  | Status                     |
| ------------------------------------------------------- | -------------------------- |
| golang-migrate, sequential (`000042_name.{up,down}.sql`) | Supported                  |
| Timestamp versions (`20240101120000_name.sql`)          | Refused with a clear error |
| goose, sql-migrate, Drizzle                             | Roadmap                    |

A `.up.sql` without a `.down.sql` is fine. Files that don't match the scheme
are ignored with a warning.

## Why not `goose fix`?

`goose fix` requires goose and its hybrid workflow, where migrations are
written with timestamps and converted to sequential numbers later. migraine
works with plain sequential numbering and doesn't care which tool applies the
migrations.

## License

[MIT](LICENSE)
