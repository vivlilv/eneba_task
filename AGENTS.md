## Working agreements

- Do not create commits unless Viv explicitly asks for a commit in that prompt.
- Work in the stages Viv requests; do not continue implementing beyond the requested slice.
- Keep code easy to read and learn from: use clear names, short focused functions, cohesive modules, and single responsibility. Extract helpers instead of stuffing behavior into large functions or files, especially in JavaScript/TypeScript.
- Do not assume unclear requirements. Ask Viv one focused question or raise a decision issue before choosing; distinguish confirmed facts from provisional choices.
- Add focused tests with behavior changes and report clearly when runtime or integration tests could not be run.

## Agent skills

### Issue tracker

Issues live in GitHub Issues; use the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context layout. See `docs/agents/domain.md`.
