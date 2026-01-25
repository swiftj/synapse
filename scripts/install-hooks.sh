#!/bin/bash
# Install Git hooks for Synapse project
#
# This script copies hooks from scripts/hooks/ to .git/hooks/
# Run this after cloning the repository.
#
# Usage: ./scripts/install-hooks.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
HOOKS_SOURCE="$SCRIPT_DIR/hooks"
HOOKS_TARGET="$PROJECT_ROOT/.git/hooks"

echo "Installing Git hooks for Synapse..."

# Check if .git directory exists
if [[ ! -d "$PROJECT_ROOT/.git" ]]; then
    echo "Error: Not a git repository. Run 'git init' first."
    exit 1
fi

# Check if hooks source directory exists
if [[ ! -d "$HOOKS_SOURCE" ]]; then
    echo "Error: Hooks source directory not found at $HOOKS_SOURCE"
    exit 1
fi

# Install each hook
for hook in "$HOOKS_SOURCE"/*; do
    if [[ -f "$hook" ]]; then
        hook_name=$(basename "$hook")
        target="$HOOKS_TARGET/$hook_name"

        # Copy and make executable
        cp "$hook" "$target"
        chmod +x "$target"
        echo "  Installed: $hook_name"
    fi
done

echo ""
echo "Git hooks installed successfully!"
echo ""
echo "Hook behavior:"
echo ""
echo "  Version Bumping (pre-commit):"
echo "    - Commits with Go files: Auto-bump patch version (0.3.2 -> 0.3.3)"
echo "    - [minor] in commit msg: Bump minor version (0.3.2 -> 0.4.0)"
echo "    - [major] in commit msg: Bump major version (0.3.2 -> 1.0.0)"
echo "    - [skip-version] in commit msg: Skip version bump"
echo ""
echo "  Tagging & Releases (post-commit, main branch only):"
echo "    - Always creates git tag when version bumps"
echo "    - [minor] or [major]: Auto-creates GitHub release"
echo "    - [release] in commit msg: Force GitHub release on patch"
echo "    - [skip-release] in commit msg: Skip GitHub release"
echo ""
echo "  Requirements for releases:"
echo "    - gh CLI installed and authenticated"
echo "    - Push access to repository"
