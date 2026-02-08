#!/bin/bash
# Generate app icons from SVG source
# Requires: rsvg-convert (brew install librsvg) or inkscape

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_SVG="$SCRIPT_DIR/diane-icon-v2.svg"
OUTPUT_DIR="$SCRIPT_DIR/../DianeMenu/Resources/Assets.xcassets/AppIcon.appiconset"
MENUBAR_SVG="$SCRIPT_DIR/menubar-icon.svg"
MENUBAR_OUTPUT="$SCRIPT_DIR/../DianeMenu/Resources/Assets.xcassets/MenuBarIcon.imageset"

# Check for converter
if command -v rsvg-convert &> /dev/null; then
    CONVERTER="rsvg-convert"
elif command -v inkscape &> /dev/null; then
    CONVERTER="inkscape"
else
    echo "Error: No SVG converter found. Install with:"
    echo "  brew install librsvg"
    echo "  or"
    echo "  brew install inkscape"
    exit 1
fi

echo "Using converter: $CONVERTER"
echo "Source: $SOURCE_SVG"
echo "Output: $OUTPUT_DIR"

# Create output directory if needed
mkdir -p "$OUTPUT_DIR"
mkdir -p "$MENUBAR_OUTPUT"

# Icon sizes for macOS app
declare -a SIZES=(16 32 128 256 512)

for size in "${SIZES[@]}"; do
    size2x=$((size * 2))
    
    echo "Generating ${size}x${size}..."
    
    if [ "$CONVERTER" = "rsvg-convert" ]; then
        rsvg-convert -w $size -h $size "$SOURCE_SVG" -o "$OUTPUT_DIR/icon_${size}x${size}.png"
        rsvg-convert -w $size2x -h $size2x "$SOURCE_SVG" -o "$OUTPUT_DIR/icon_${size}x${size}@2x.png"
    else
        inkscape -w $size -h $size "$SOURCE_SVG" -o "$OUTPUT_DIR/icon_${size}x${size}.png"
        inkscape -w $size2x -h $size2x "$SOURCE_SVG" -o "$OUTPUT_DIR/icon_${size}x${size}@2x.png"
    fi
done

# Generate menu bar icon (PDF for vector support)
echo "Generating menu bar icon..."
if [ "$CONVERTER" = "rsvg-convert" ]; then
    rsvg-convert -f pdf "$MENUBAR_SVG" -o "$MENUBAR_OUTPUT/menubar-icon.pdf"
else
    inkscape "$MENUBAR_SVG" -o "$MENUBAR_OUTPUT/menubar-icon.pdf"
fi

# Update Contents.json for AppIcon
cat > "$OUTPUT_DIR/Contents.json" << 'EOF'
{
  "images" : [
    {
      "filename" : "icon_16x16.png",
      "idiom" : "mac",
      "scale" : "1x",
      "size" : "16x16"
    },
    {
      "filename" : "icon_16x16@2x.png",
      "idiom" : "mac",
      "scale" : "2x",
      "size" : "16x16"
    },
    {
      "filename" : "icon_32x32.png",
      "idiom" : "mac",
      "scale" : "1x",
      "size" : "32x32"
    },
    {
      "filename" : "icon_32x32@2x.png",
      "idiom" : "mac",
      "scale" : "2x",
      "size" : "32x32"
    },
    {
      "filename" : "icon_128x128.png",
      "idiom" : "mac",
      "scale" : "1x",
      "size" : "128x128"
    },
    {
      "filename" : "icon_128x128@2x.png",
      "idiom" : "mac",
      "scale" : "2x",
      "size" : "128x128"
    },
    {
      "filename" : "icon_256x256.png",
      "idiom" : "mac",
      "scale" : "1x",
      "size" : "256x256"
    },
    {
      "filename" : "icon_256x256@2x.png",
      "idiom" : "mac",
      "scale" : "2x",
      "size" : "256x256"
    },
    {
      "filename" : "icon_512x512.png",
      "idiom" : "mac",
      "scale" : "1x",
      "size" : "512x512"
    },
    {
      "filename" : "icon_512x512@2x.png",
      "idiom" : "mac",
      "scale" : "2x",
      "size" : "512x512"
    }
  ],
  "info" : {
    "author" : "xcode",
    "version" : 1
  }
}
EOF

echo "Done! Icons generated at: $OUTPUT_DIR"
echo "Menu bar icon at: $MENUBAR_OUTPUT"
