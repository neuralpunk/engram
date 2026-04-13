List, search, and manage all stored corrections in engram.

## Instructions

The user wants to see or manage their stored corrections. Determine what they need:

### List all corrections
```bash
engram list
```

### List by scope
```bash
engram list --scope global
engram list --scope project:myproject
engram list --scope domain:go
```

### List by tag
```bash
engram list --tag config
```

### Search by topic
```bash
engram search "<query>"
```

### Show statistics
```bash
engram stats
```

### Export corrections
```bash
engram export                        # JSON to stdout
engram export --format toml          # TOML to stdout
engram export -o corrections.json    # JSON to file
```

### Import corrections
```bash
engram import corrections.json
engram import corrections.toml
```

### Edit a correction
```bash
engram edit <id>
```
Opens the correction in $EDITOR as JSON with editable fields (fact, scope, tags).

## Output format

When displaying corrections, format them readably:
- Show the ID, scope, fact, and date
- Group by scope if there are many
- Mention total count
