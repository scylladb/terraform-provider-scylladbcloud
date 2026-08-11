# Code Comments

Avoid adding unnecessary code comments, in particular those that describe the code one-to-one.

When comments should be added, be concise and precise.

# Git Commits

Only create commits when explicitly asked. Do not commit
automatically after completing a task.

Follow standard git commit message formatting rules.

**Subject line (first line)**
- 50 characters or fewer
- Imperative mood: "add", "fix", "rename" — not "added" or "fixes"
- No trailing period
- Use conventional commits format: `type(scope): subject`

**Blank line**
- Always separate subject from body with a blank line

**Body lines**
- Hard wrap at 72 characters — no line may exceed 72 chars including the trailing space
- Explain *what* and *why*, not *how*

These limits ensure correct rendering in `git log`, `git format-patch`,
GitHub, and 80-column terminals.
