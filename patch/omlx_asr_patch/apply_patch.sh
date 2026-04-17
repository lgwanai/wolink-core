#!/bin/bash

# Target directory for the installed macOS oMLX App
OMLX_DIR="/Applications/oMLX.app/Contents/Resources/omlx"

if [ ! -d "$OMLX_DIR" ]; then
    echo "❌ Error: oMLX directory not found at $OMLX_DIR"
    echo "Please ensure oMLX is installed in the macOS Applications folder."
    exit 1
fi

echo "🚀 Starting to apply oMLX ASR patch (char_level_info support)..."

# Get the directory where the script is located
PATCH_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Define the files to patch
FILES=(
    "api/audio_models.py"
    "api/audio_routes.py"
    "engine/stt.py"
)

for FILE in "${FILES[@]}"; do
    SRC_FILE="$PATCH_DIR/$FILE"
    DEST_FILE="$OMLX_DIR/$FILE"

    if [ ! -f "$SRC_FILE" ]; then
        echo "⚠️ Warning: Patch file $SRC_FILE not found, skipping."
        continue
    fi

    # Backup the original file if a backup does not already exist
    if [ ! -f "$DEST_FILE.bak" ] && [ -f "$DEST_FILE" ]; then
        cp "$DEST_FILE" "$DEST_FILE.bak"
        echo "📦 Backed up original file to: $DEST_FILE.bak"
    fi

    # Copy the patched file to the destination
    cp "$SRC_FILE" "$DEST_FILE"
    echo "✅ Successfully patched: $DEST_FILE"
done

echo ""
echo "🎉 Patch applied successfully!"
echo "⚠️ IMPORTANT: Please restart your oMLX application or the omlx.server process for the changes to take effect."