Retrieve relevant corrections from engram for the current context.

## Instructions

Retrieve corrections relevant to the current conversation topic and display them. This is useful when switching topics or when you need to refresh context.

1. If the user provided a topic, search for it:
   ```bash
   engram get "<topic>" --raw
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
