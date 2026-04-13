Find and delete a correction from engram's persistent memory.

## Instructions

The user wants to remove something engram has stored. They may describe it vaguely ("forget what you know about auth") or specifically ("delete correction #5").

1. If the user gave a specific ID, delete it directly:
   ```bash
   engram delete <id>
   ```

2. If the user described what to forget, search for it first:
   ```bash
   engram search "<description>"
   ```
   Show the matching corrections and their IDs to the user, then ask which to delete. If there's only one obvious match, delete it directly.

3. For bulk deletion by scope:
   ```bash
   engram list --scope <scope>
   ```
   Then delete each matching ID.

4. Confirm what was deleted: "Done, removed correction #N."

## Examples

User says: "Forget what you know about the old auth flow"
→ `engram search "auth flow"` → show results → `engram delete <id>`

User says: "That last correction was wrong, remove it"
→ `engram list --limit 1` → `engram delete <id>`

User says: "Delete correction 5"
→ `engram delete 5`
