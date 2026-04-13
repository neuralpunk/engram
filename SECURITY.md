# Security Policy

## Scope

engram stores user corrections in a local SQLite database. It never makes network connections, transmits data, or runs as a service. The attack surface is limited to local file access.

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.2.x   | Yes       |

## Reporting a vulnerability

If you find a security issue, please open a GitHub issue with the **security** label rather than disclosing details publicly.

Include:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment

We aim to release a fix within 7 days for confirmed vulnerabilities.

## Security measures

- Database files are created with `0600` permissions (owner read/write only)
- Config directories use `0750` permissions
- All SQL queries use parameterized statements (no string interpolation)
- No network access, no telemetry, no cloud sync
- No shell expansion or command injection vectors in stored data
