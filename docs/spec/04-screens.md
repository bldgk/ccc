# ccc — screens

What `ccc` shows in the terminal. The commands are in [02-surfaces.md](02-surfaces.md); all
data on every screen comes from the engine described in
[05-architecture.md](05-architecture.md).

The look — palette, gradient title, group headers, size bars, spinner, rounded panel — is the
look of the prototype in `spikes/checkscreen/`, which the owner looked at and accepted. The
list screen below is that prototype's screen. The statements marked **Assumed** describe
what the prototype did not have and the owner has not seen.

## The list — `ccc check`, `ccc`

- A title line with the name of the program.
- Rows grouped under the name of their entry. A row shows: a mark box, the name of the place
  (and the version name for a row of installed versions; the path for a folder), a bar
  proportional to the largest measured row, and the size.
- A row is in one of these states: being measured (spinner); a size; "not measured" with the
  reason; "could not be determined" with the reason. Rows appear and fill in as the scan
  delivers them. A place that is not present has no row.
- A footer with the keys and with the count and the total size of the marked rows.
- Keys: arrows or `j`/`k` move, `g`/`G` jump to the first and last row, space marks a row,
  `a` marks or unmarks everything that "clean everything" covers — never a row of installed
  versions — Enter asks to clean the marked rows, `q` or Ctrl-C quits.
- A row that could not be determined cannot be marked.
- An entry file of the user that failed to load is shown under the list with its reason.
  **Assumed.**
- When nothing was found, the screen says so in one line. **Assumed.**
- When the list is taller than the window, it scrolls with the cursor, and a line above and a
  line below the rows say how many rows are out of view (`↑ 3 more`, `↓ 9 more`). A group
  name is never shown at the bottom without one of its rows. **Assumed.**

## The confirmation

Enter with at least one marked row opens a panel: the marked rows with their sizes, their
total, and the two keys — `y` cleans, any other key returns to the list with the marks kept.
Enter with nothing marked does nothing. **Assumed:** the keys.

## Cleaning and the result

**Assumed**, all of it:

- The panel stays and each row gets a state: waiting, being cleaned (spinner), cleaned with the
  size freed, failed with the message of the tool.
- When the last row is done the panel shows the total freed and the number of failed rows.
  Any key starts a new scan and returns to the list.
- Ctrl-C during cleaning stops before the next row; the row in progress finishes.

## `ccc --clean`, `ccc -c`

**Assumed**, all of it:

- No full-screen interface. `ccc` scans, then cleans, printing one line per row as it
  finishes: the entry, the place, and either the size freed or `failed` with the message.
- The last line is the total freed and the number of failed rows.
- The exit status is 0 when no row failed and 1 otherwise.

## Output that is not a terminal

**Assumed**, all of it:

- `ccc check` into a pipe or a file prints the rows as aligned text after the scan has
  finished — entry, place, version, size or state with the reason — and removes nothing.
- `ccc -c` prints the same lines as on a terminal, without colour.
- `NO_COLOR` turns colour off everywhere.

## Adding a folder — `ccc add <path>`

**Assumed**, all of it:

- No screen. The path is made absolute, and a path inside the home directory is stored with
  `~`. The name of the entry is the last element of the path; `--name` gives another one.
- `ccc` prints the file it wrote and the row the folder gives: its size, or that the directory
  does not exist yet.
- The command refuses, with the reason and without writing anything, when an entry of the
  user with that name exists already, or when the path is one the removal guards of the
  architecture would refuse.
- A name taken from the path that equals the name of a shipped entry is refused too: the
  folder would replace that entry by accident. With `--name` it is allowed, and `ccc` prints
  that the new entry replaces the shipped one.

## Adding a tool — `ccc add`

**Assumed**, all of it. A form in the style of the list screen, one question at a time:

1. The name of the entry and the name of the tool's executable.
2. A place: its name, and whether it holds a cache or installed versions.
3. Detection: a command of the tool that prints the path, an environment variable, a default
   path — any of the three, at least one. For installed versions also the command that lists
   them and the pattern that takes the version name from a line. `ccc` then runs detection
   and shows the path it found and, for versions, the names it listed.
4. Space check: disk usage of the path, or a command with the pattern that takes the size
   from its output. `ccc` runs it and shows the size.
5. Clean: the tool's own command, or removal of the directory. Nothing is run.
6. Whether to add another place; then the entry is shown as the text of its file and saved on
   confirmation. `ccc` prints the file it wrote.

- A step whose check fails shows the reason and lets the user change the answers or keep
  them.
- The form refuses a name an entry of the user already has. A name of a shipped entry is
  allowed and the form says that the new entry replaces the shipped one.
- Esc or Ctrl-C leaves the form without writing anything.
- When the input is not a terminal, `ccc add` without a path prints that the form needs a
  terminal and that an entry file can be written by hand, and exits with a non-zero status.

## Sizes

Sizes are shown in decimal units (`kB`, `MB`, `GB`), as the tools themselves print them.
**Assumed.**
