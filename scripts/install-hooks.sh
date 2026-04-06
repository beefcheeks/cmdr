#!/bin/bash
# cmdr: Install Claude Code hooks for refactor flow.
# Merges PostToolUse hook into ~/.claude/settings.json (preserving existing settings).
# Installs hook script to ~/.cmdr/hooks/.

set -e

HOOKS_DIR="$HOME/.cmdr/hooks"
SETTINGS="$HOME/.claude/settings.json"
HOOK_SCRIPT="$HOOKS_DIR/on-pr-create.sh"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Install hook script
mkdir -p "$HOOKS_DIR"
cp "$SCRIPT_DIR/on-pr-create.sh" "$HOOK_SCRIPT"
chmod +x "$HOOK_SCRIPT"

# Ensure settings file exists
mkdir -p "$HOME/.claude"
[ -f "$SETTINGS" ] || echo '{}' > "$SETTINGS"

# Check if our hook is already registered
if grep -q "on-pr-create.sh" "$SETTINGS" 2>/dev/null; then
    exit 0
fi

# Merge hook into settings using python3 (available on macOS)
python3 -c "
import json, sys

path = '$SETTINGS'
with open(path) as f:
    settings = json.load(f)

hooks = settings.setdefault('hooks', {})
post_hooks = hooks.setdefault('PostToolUse', [])

# Add our hook if not already present
cmdr_hook = {
    'matcher': 'Bash',
    'hooks': [{
        'type': 'command',
        'command': '$HOOK_SCRIPT'
    }]
}

# Check if already exists
for h in post_hooks:
    if h.get('matcher') == 'Bash':
        for sub in h.get('hooks', []):
            if 'on-pr-create.sh' in sub.get('command', ''):
                sys.exit(0)

post_hooks.append(cmdr_hook)

with open(path, 'w') as f:
    json.dump(settings, f, indent=2)
    f.write('\n')
"

echo "cmdr: claude hooks installed"
