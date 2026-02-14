# Diane Icon Assets

## Source Files

- `diane-icon.svg` - App icon (512x512) - Dark theme with recorder
- `diane-icon-v2.svg` - App icon (512x512) - Cleaner, bolder design  
- `menubar-icon.svg` - Menu bar icon (22x22) - Monochrome template

## Design Concept

The icon is inspired by Agent Cooper's Olympus Pearlcorder microcassette recorder from Twin Peaks. Cooper would hold up the device and dictate: *"Diane, 11:30 AM, February 24th..."*

### Elements:
- **Microcassette recorder** - The main shape
- **Tape reels** - Visible through the window
- **Recording indicator (red dot)** - Shows "Diane is listening"
- **Forest green background** - Douglas Fir / Pacific Northwest
- **Chevron pattern** - Subtle Red Room floor reference

## Converting to App Icon

### Using rsvg-convert (recommended):
```bash
# Install rsvg-convert
brew install librsvg

# Generate all icon sizes
for size in 16 32 64 128 256 512 1024; do
    rsvg-convert -w $size -h $size diane-icon-v2.svg -o icon_${size}x${size}.png
done

# For @2x versions
rsvg-convert -w 32 -h 32 diane-icon-v2.svg -o icon_16x16@2x.png
rsvg-convert -w 64 -h 64 diane-icon-v2.svg -o icon_32x32@2x.png
rsvg-convert -w 256 -h 256 diane-icon-v2.svg -o icon_128x128@2x.png
rsvg-convert -w 512 -h 512 diane-icon-v2.svg -o icon_256x256@2x.png
rsvg-convert -w 1024 -h 1024 diane-icon-v2.svg -o icon_512x512@2x.png
```

### Using Inkscape:
```bash
for size in 16 32 64 128 256 512 1024; do
    inkscape -w $size -h $size diane-icon-v2.svg -o icon_${size}x${size}.png
done
```

### Menu Bar Icon (PDF for vector support):
```bash
# Convert to PDF for Xcode asset catalog
rsvg-convert -f pdf menubar-icon.svg -o ../Diane/Resources/Assets.xcassets/MenuBarIcon.imageset/menubar-icon.pdf
```

## Xcode Asset Catalog Setup

After generating the PNG files, copy them to:
```
Diane/Resources/Assets.xcassets/AppIcon.appiconset/
```

Update the `Contents.json` to reference the files:
```json
{
  "images": [
    { "filename": "icon_16x16.png", "idiom": "mac", "scale": "1x", "size": "16x16" },
    { "filename": "icon_16x16@2x.png", "idiom": "mac", "scale": "2x", "size": "16x16" },
    ...
  ]
}
```

## Color Palette

| Element | Hex | Description |
|---------|-----|-------------|
| Background | #1B3D2F | Deep forest green |
| Recorder body | #E8E4DF | Warm off-white plastic |
| Dark accents | #2D2D2D | Speaker grille, buttons |
| Recording dot | #C41E3A | Twin Peaks red |
| Chevron | #8B0000 | Dark red (25% opacity) |
