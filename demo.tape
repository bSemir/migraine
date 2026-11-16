# Records demo.gif: the collision repro, then migraine check and fix.
# Run with `make demo` (builds ./migraine first).

Output demo.gif

Require git

Set Shell bash
Set FontSize 16
Set Width 1280
Set Height 720
Set Padding 24
Set TypingSpeed 30ms

# Off-camera: put the fresh build on PATH and start a repository with
# one migration in a throwaway directory.
Hide
Type `export PATH="$PWD:$PATH" PS1='$ ' GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example.com GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example.com`
Enter
Type `cd "$(mktemp -d)" && git init -q -b main && mkdir migrations && touch migrations/000001_init.{up,down}.sql && git add . && git commit -qm init && clear`
Enter
Sleep 1s
Show

Type "# A teammate adds migration 000002..."
Enter
Type "git checkout -q -b teammate && touch migrations/000002_add_users.{up,down}.sql && git add . && git commit -qm users"
Enter
Sleep 1s

Type "# ...and so do you, on your own branch."
Enter
Type "git checkout -q main && git checkout -q -b feature"
Enter
Sleep 1s
Type "touch migrations/000002_add_orders.{up,down}.sql && git add . && git commit -qm orders"
Enter
Sleep 1s

Type "# Their branch is merged, you rebase. Git reports no conflict:"
Enter
Type "git checkout -q main && git merge -q teammate && git checkout -q feature && git rebase -q main"
Enter
Sleep 1s
Type "ls migrations"
Enter
Sleep 3s

Type "migraine check"
Enter
Sleep 3s

Type "migraine fix"
Enter
Sleep 2s
Type "ls migrations"
Enter
Sleep 4s
