# Security

Report vulnerabilities privately through GitHub's "Report a vulnerability" button on this repository, or by email to the address on the maintainer's GitHub profile. You will get an acknowledgement within a week.

## Threat model

interlock runs as an MCP server inside an agent session and talks to a local intermute. Its pre-edit hook blocks Edit and Write tool calls on files another agent holds exclusively (shell writes are not inspected), and the git pre-commit hook blocks commits that touch them. Both fail open: if intermute is unreachable they warn once and let the action through. interlock never reads file contents; it exchanges file patterns and short messages between agents that already share the machine.
