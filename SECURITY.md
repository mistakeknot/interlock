# Security

Report vulnerabilities privately through GitHub's "Report a vulnerability" button on this repository, or by email to the address on the maintainer's GitHub profile. You will get an acknowledgement within a week.

## Threat model

interlock runs as an MCP server inside an agent session and talks to a local intermute. Its hooks are advisory except the git pre-commit hook, which blocks commits to files another agent has reserved. Hooks fail open: if intermute is unreachable they do nothing. interlock never reads file contents; it exchanges file patterns and short messages between agents that already share the machine.
