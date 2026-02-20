# AGENTS.md

## Development Process

- Use strict TDD.
- Before feature work, prime context with `kanban primer` if that context is not already present.
- If the `kanban` command cannot talk to the server, stop immediately and tell the user to start the server.
- Ask extensive clarifying questions about the feature before planning implementation.
- Create cards in the kanban board for the agreed work.
- If working in a worktree/branch, record the active worktree/branch on the card.
- Once feature context and scope are aligned, move the card to `doing`.
- Start implementation only after the card is in `doing`.
- When implementation is done, move the card to `review` and notify the user.
- Continue working until all relevant tests pass.
- Exception: if blocked and unable to proceed without user input, stop and ask concise clarifying questions.
- Preferred development flow is: `e2e/black-box tests -> unit tests -> implementation`.
- For new components (especially CLI), start by writing black-box/e2e tests that execute real binaries/processes and validate behavior end-to-end.
- After e2e tests are in place, add focused unit tests for command/config/domain logic.
- Only then implement code changes to satisfy tests.
- For every feature/change, follow this loop:
  1. Write a failing test first.
  2. Implement the smallest possible change to make the test pass.
  3. Refactor while keeping tests green.
- Do not skip the failing-test-first step.
- Prefer small, incremental commits that reflect the TDD cycle.

## Testing Expectations

- Run relevant tests after each change.
- Run full test suites before considering work complete.
- If a test fails unexpectedly, investigate root cause before adding new behavior.

## Git Operations

- You are allowed to run git operations as needed (status, add, restore, branch, commit, rebase, merge, etc.).
- You are explicitly allowed to create and switch branches.
- You are explicitly allowed to create commits.
- You must NOT run `git push`.
- If publishing changes remotely is required, stop and ask the user to push.
