# Examples

---

## Positional

```sh
mycmd aaa bbb
```

| Declared Arg | Value |
|--------------|-------|
| `arg1: str`  | `aaa` |
| `arg2: str`  | `bbb` |

---

## Flags

```sh
mycmd aaa --arg2 bbb -c ddd -f
```

| Declared Arg            | Value  |
|-------------------------|--------|
| `arg1: str`             | `aaa`  |
| `arg2: str`             | `bbb`  |
| `arg3: str`, short `c`  | `ddd`  |
| `arg4: bool`, short `f` | `true` |

---

## Positional slots assigned left to right

```sh
mycmd --arg1=aaa bbb  # assigns 'bbb' to 'arg2' because 'arg1' already assigned
```

| Declared Arg | Value |
|--------------|-------|
| `arg1: str`  | `aaa` |
| `arg2: str`  | `bbb` |

```sh
mycmd aaa --arg1=bbb  # 'aaa' first gets assigned positionally to 'arg1', and then overridden by 'bbb'.
```

| Declared Arg | Value                                                          |
|--------------|----------------------------------------------------------------|
| `arg1: str`  | `bbb`                                                          |
| `arg2: str`  | `undefined`, error because both inputs go to assigning `arg1`. |

---

## Short clusters (bools)

```sh
mycmd -bcd aaa  # since flags are bools, 'aaa' gets interpreted as the first positional arg
```

| Declared Arg            | Value   |
|-------------------------|---------|
| `arg1: str`             | `aaa`   |
| `arg2: bool`, short `b` | `true`  |
| `arg3: bool`, short `c` | `true`  |
| `arg4: bool`, short `d` | `true`  |
| `arg5: bool`, short `e` | `false` |

---

## Short clusters can end with non-bool

```sh
mycmd -abc ddd  # last flag 'c' is a non-bool and so will read 'ddd'
```

| Declared Arg            | Value  |
|-------------------------|--------|
| `arg1: bool`, short `a` | `true` |
| `arg2: bool`, short `b` | `true` |
| `arg3: str`, short `c`  | `ddd`  |

---

## Incrementing int shorts

Int and Int64 flags support clustered short flag counting:
- When the same short flag is repeated (e.g., `-vvv`), the value is set to the count of repetitions
- Explicit values override counting (e.g., `-vvv=10` sets value to 10, not 3)
- Later occurrences override earlier ones (e.g., `-vv -v 5` sets value to 5)

**Basic counting:**
```sh
mycmd -aaa
```

| Declared Arg                        | Value |
|-------------------------------------|-------|
| `arg1: int`, short `a`, default `0` | `3`   |

**Single flag with explicit value:**
```sh
mycmd -a 3
```

| Declared Arg                        | Value |
|-------------------------------------|-------|
| `arg1: int`, short `a`, default `0` | `3`   |

**Counting with two repetitions:**
```sh
mycmd -aa
```

| Declared Arg                        | Value |
|-------------------------------------|-------|
| `arg1: int`, short `a`, default `0` | `2`   |

**Explicit value overrides counting:**
```sh
mycmd -aaa=10
```

| Declared Arg                        | Value |
|-------------------------------------|-------|
| `arg1: int`, short `a`, default `0` | `10`  |

**Int64 flags work the same way:**
```sh
mycmd -nnn
```

| Declared Arg                          | Value |
|---------------------------------------|-------|
| `arg1: int64`, short `n`, default `0` | `3`   |

**Error case - non-integer value:**
```sh
mycmd -a bbb
```

| Declared Arg                        | Value |
|-------------------------------------|-------|
| `arg1: int`, short `a`, default `0` | ?     |
| `arg2: str`                         | ?     |

---

## Negative numbers

```sh
mycmd -1 --arg2 -2 -3.4
```

| Declared Arg  | Value  |
|---------------|--------|
| `arg1: int`   | `-1`   |
| `arg2: int`   | `-2`   |
| `arg3: float` | `-3.4` |

---

## Values that look like flags

A `-`-prefixed token is read as a flag, never as the value of the flag before it.

```sh
mycmd --arg1 --arg2
```

This is invalid: `--arg1` has no value. Reach for `=` to pass the token literally.

```sh
mycmd --arg1=--arg2
```

| Declared Arg | Value    |
|--------------|----------|
| `arg1: str`  | `--arg2` |

Two shapes are still values. A bare `-` is the stdin/stdout convention, and a
number-typed arg takes a negative number.

```sh
mycmd --arg1 - --arg2 -5
```

| Declared Arg | Value |
|--------------|-------|
| `arg1: str`  | `-`   |
| `arg2: int`  | `-5`  |

The negative-number exception is typed: `--arg2 -5` works because `arg2` is an
int. Against a `str` arg, `-5` is a flag like any other, and `--arg1=-5` is how
you say you meant it.

---

## Number shorts enables 'number shorts mode'

If any arg defines an int short, we enter 'number shorts mode'.
This means that any standalone int flags are *always* interpreted as flags, not negative ints.
In this mode, you can only pass negative ints as values to arguments by using `=`.

```sh
mycmd --arg1=-2 -2 aaa -a bbb ccc
```

| Declared Arg           | Value |
|------------------------|-------|
| `arg1: int`            | `-2`  |
| `arg2: str`, short `2` | `aaa` |
| `arg3: int`            | `ccc` |
| `arg4: str`, short `a` | `bbb` |

Take the below alternative invocation.

```sh
mycmd --arg1 -2 -2 aaa -a bbb ccc
```

This is invalid. Because the `-2` is interpreted as a short flag in both instances, the initial `--arg1` doesn't
have a corresponding value, nor does the `-2` that follows.

In other words, number shorts mode is a more restrictive mode to avoid ambiguities and mistakes caused by the
conflicting use cases of having int flags and passing negative ints as args to a script.

When no int shorts are defined, number shorts mode remains disabled, meaning any "int flags" are interpreted as
negative integer values.

---

## List positionals

A list arg collects bare positional values greedily, the same as a variadic. The
two differ on the flag side: a variadic takes a run of values after its own
token, a list takes exactly one and is repeated instead.

With `arg1: str`, variadic and `arg2: str[]`, short `c`, default `[]`:

```sh
mycmd t1 t2 t3 -c c1 c2
```

| Declared Arg          | Value            |
|-----------------------|------------------|
| `arg1: str`, variadic | `[ t1, t2, t3 ]` |
| `arg2: str[]`         | `[ c1 ]`         |

`c2` is an error: `-c` took its one value, and neither collector resumes after a
flag. To pass several values to `arg2`, repeat the flag (`-c c1 -c c2`) or pass
them positionally.

```sh
mycmd x y -c c1
```

| Declared Arg  | Value           |
|---------------|-----------------|
| `arg2: str[]` | `[ x, y, c1 ]`  |

The bare values open the collection, and the later flag appends to the same
list. Reversed, it doesn't work:

```sh
mycmd -c c1 x y
```

`arg2` is `[ c1 ]`, and `x` and `y` are errors. Non-resumption is what lets
several collectors divide one command line into runs; see Multiple Variadics.

---

## Positional variadic

The last positional arg can be a variadic.
If nothing is read into it, it will be an empty slice.

```sh
mycmd aaa
```

| Declared Arg          | Value |
|-----------------------|-------|
| `arg1: str`           | `aaa` |
| `arg2: str`, variadic | `[ ]` |

```sh
mycmd aaa bbb
```

| Declared Arg          | Value     |
|-----------------------|-----------|
| `arg1: str`           | `aaa`     |
| `arg2: str`, variadic | `[ bbb ]` |

```sh
mycmd aaa bbb ccc
```

| Declared Arg          | Value          |
|-----------------------|----------------|
| `arg1: str`           | `aaa`          |
| `arg2: str`, variadic | `[ bbb, ccc ]` |

---

## Variadic flags

```sh
mycmd aaa --arg2  # Can be specified as a flag, even with no arguments
```

| Declared Arg          | Value |
|-----------------------|-------|
| `arg1: str`           | `aaa` |
| `arg2: str`, variadic | `[ ]` |

```sh
mycmd aaa --arg2 bbb ccc
```

| Declared Arg          | Value          |
|-----------------------|----------------|
| `arg1: str`           | `aaa`          |
| `arg2: str`, variadic | `[ bbb, ccc ]` |

```sh
mycmd --arg2 aaa bbb --arg1 ccc  # Vararg reads until the next flag 
```

| Declared Arg          | Value          |
|-----------------------|----------------|
| `arg1: str`           | `ccc`          |
| `arg2: str`, variadic | `[ aaa, bbb ]` |

---

## Multiple Variadics

Input args will be assigned to the first var arg declaration the parser encounters, or until it reaches
a new flag. A new flag could be another variadic arg, repeating the same logic again.

```sh
mycmd aaa bbb --arg2 ccc ddd -e fff
```

| Declared Arg            | Value          |
|-------------------------|----------------|
| `arg1: str`, variadic   | `[ aaa, bbb ]` |
| `arg2: str`, variadic   | `[ ccc, ddd]`  |
| `arg3: bool`, short `e` | `true`         |
| `arg4: str`             | `fff`          |

Here, we consume `ccc` and `ddd` and assign them to `arg2`, but its variadic is then ended by `-e`. We then read `fff`,
check to see our next unfilled positional arg (`arg4`), and assign to that.

## Default Subcommand

A tool with a primary action plus a few subcommands. `SetDefaultCmd` makes the
primary action reachable without naming it, while keeping it named for when a
value would otherwise be mistaken for a subcommand.

```go
root := ra.NewCmd("remind")

add := ra.NewCmd("add")
tokens, _ := ra.NewStringSlice("tokens").Register(add)
root.RegisterCmd(add)

list := ra.NewCmd("list")
root.RegisterCmd(list)

root.SetDefaultCmd("add")
```

```
remind 15m standup     -> add, tokens = ["15m", "standup"]
remind list            -> list
remind add list        -> add, tokens = ["list"]      # explicit escape
remind -- list         -> add, tokens = ["list"]      # conventional escape
```

The command list marks it, so the shorter spelling is discoverable:

```
Commands:
  add    Add a reminder. (default)
  list   Show upcoming reminders.
```
