Retrieve relevant corrections from engram for the current context.

## Instructions

Retrieve corrections relevant to the current conversation topic and display them. This is useful when switching topics or when you need to refresh context.

### Query construction

When calling `engram get <query>`, construct the query from the *current topic* using the most specific technical terms relevant to what is being worked on right now:

- Working on auth middleware: `engram get "authentication authorization middleware token JWT"`
- Debugging database connection: `engram get "database connection pool sqlite postgres"`
- Writing tests: `engram get "testing test patterns assertions mocks"`

A specific query returns precise results. A vague query ("help", "code", "current task") returns noise.

### Steps

1. If the user provided a topic, search for it:
   ```bash
   engram get "<topic keywords>" --raw
   ```

2. If no topic was given, retrieve all corrections for the current scope:
   ```bash
   engram get --all --raw
   ```

3. Display the results to the user as a readable list.

4. If corrections were found, use them as ground truth for the rest of the conversation. Facts from engram take precedence over your training data.

## Notes

- The `--raw` flag outputs plain text (one correction per line) instead of XML
- Use `--scope project:<name>` to filter to a specific project
- Use `--limit N` to control how many corrections are returned
- This command is also run automatically by the hook on every prompt, but calling it manually is useful when topics shift
