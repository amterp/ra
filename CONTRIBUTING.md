# Contributing to Ra

Issues and PRs welcome! This library prioritizes:

- **Deterministic behavior** – same input always produces same output
- **Clear error messages** – users should understand what went wrong  
- **Type safety** – catch mistakes at compile time when possible
- **Intuitive API** – common tasks should be simple

## Development

See existing tests for examples and patterns. Run the validation suite:

```bash
./dev --validate
```

The `dev` script requires [Rad](https://github.com/amterp/rad).

## Commit Messages

To help with automated release notes, consider using [conventional commit](https://www.conventionalcommits.org/) prefixes:

- `feat:` for new features
- `fix:` for bug fixes  
- `docs:` for documentation changes
- `refactor:` for code refactoring
- `test:` for adding tests

**Good Examples:**

```
feat: add dual-nature argument support

Allow flags to work both positionally and as named flags, giving users 
flexibility in how they invoke commands. Updates parser logic to handle
both styles seamlessly.
```

```
fix: handle negative numbers in number shorts mode

When int flags define short names, negative numbers like -5 were being
interpreted as short flags. Now requires --flag=-5 syntax to avoid
ambiguity, matching the documented behavior.
```

```
docs: update README with constraint examples
```

**Commit message guidelines:**
- **First line**: Brief summary (72 chars max, present tense)
- **Body**: Explain the "why" not just the "what"
- **Include context**: What problem does this solve?
- **Reference issues**: "Fixes #123" or "Closes #456"

This helps categorize changes in release notes, but regular descriptive commit messages work fine too!
