# ccc — surfaces

`ccc` has one surface: a command-line program. What it must do is in
[00-requirements.md](00-requirements.md).

| Invocation | What it is | Decided in |
|---|---|---|
| `ccc check`, `ccc` | The interactive screen: rows with sizes, marking, cleaning marked rows or everything, confirmation, the result after cleaning | Requirements, "See what takes space" and "Clean a selection" |
| `ccc --clean`, `ccc -c` | Cleans every cache place and every folder without a screen and without confirmation, then reports what was freed | Requirements, "Clean everything" |
| `ccc add` | A form that adds a tool through the three steps of the contract | Requirements, "Add a tool" |
| `ccc add <path>` | Adds a folder by path | Requirements, "Add a folder" |
| output that is not a terminal | What `ccc` prints into a pipe or a file | Open point of the requirements |

The registry files are the second thing a user touches directly: TOML files, one per tool, in
the user's registry directory. Their schema is in
[05-architecture.md](05-architecture.md#the-entry-file).

What each of them shows is in [04-screens.md](04-screens.md).
