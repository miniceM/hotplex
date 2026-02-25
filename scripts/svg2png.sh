#!/bin/bash
#
# SVG to PNG Converter for HotPlex
# Converts all SVG files in docs/images to high-resolution PNG
#
# Usage:
#   ./scripts/svg2png.sh [options]
#
# Options:
#   -s, --src-dir      Source directory containing SVG files (default: docs/images)
#   -o, --output-dir   Output directory for PNG files (default: docs/images/png)
#   -z, --zoom         Zoom factor for resolution (default: 4)
#   -b, --background   Background color in hex (default: transparent)
#   -h, --help         Show this help message
#
# Examples:
#   ./scripts/svg2png.sh                           # Convert all with defaults
#   ./scripts/svg2png.sh -z 8                      # 8x resolution (8K+)
#   ./scripts/svg2png.sh -b "#C0C0C0"              # Silver gray background
#   ./scripts/svg2png.sh -o assets -z 4 -b "#FFF"  # Custom output, 4K, white bg
#

set -e

# Default values
SRC_DIR="docs/images"
OUTPUT_DIR="docs/images/png"
ZOOM=4
BACKGROUND=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -s|--src-dir)
            SRC_DIR="$2"
            shift 2
            ;;
        -o|--output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -z|--zoom)
            ZOOM="$2"
            shift 2
            ;;
        -b|--background)
            BACKGROUND="$2"
            shift 2
            ;;
        -h|--help)
            head -30 "$0" | tail -25
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Check dependencies
check_dependencies() {
    local missing=()
    
    if ! command -v rsvg-convert &> /dev/null; then
        missing+=("librsvg (brew install librsvg)")
    fi
    
    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${RED}Missing dependencies:${NC}"
        printf "  - %s\n" "${missing[@]}"
        exit 1
    fi
}

# Convert SVG to PNG
convert_svg() {
    local svg_file="$1"
    local filename=$(basename "$svg_file" .svg)
    local output_file="${OUTPUT_DIR}/${filename}.png"
    
    # Calculate dimensions for display
    local dims=$(rsvg-convert -d 96 -p 96 "$svg_file" 2>/dev/null | file - | grep -oE '[0-9]+ x [0-9]+' || echo "unknown")
    local orig_w=$(echo "$dims" | awk '{print $1}')
    local orig_h=$(echo "$dims" | awk '{print $3}')
    local final_w=$((orig_w * ZOOM))
    local final_h=$((orig_h * ZOOM))
    
    # Build command
    local cmd="rsvg-convert -z $ZOOM"
    
    if [ -n "$BACKGROUND" ]; then
        cmd="$cmd --background-color=\"$BACKGROUND\""
    fi
    
    cmd="$cmd -o \"$output_file\" \"$svg_file\""
    
    echo -e "  ${BLUE}→${NC} $filename.svg → ${final_w}×${final_h} PNG"
    
    eval $cmd
}

# Main
main() {
    check_dependencies
    
    # Verify source directory exists
    if [ ! -d "$SRC_DIR" ]; then
        echo -e "${RED}Source directory not found: $SRC_DIR${NC}"
        exit 1
    fi
    
    # Count SVG files
    local svg_count=$(find "$SRC_DIR" -maxdepth 1 -name "*.svg" -type f | wc -l | tr -d ' ')
    
    if [ "$svg_count" -eq 0 ]; then
        echo -e "${RED}No SVG files found in $SRC_DIR${NC}"
        exit 1
    fi
    
    # Create output directory
    mkdir -p "$OUTPUT_DIR"
    
    # Also handle HotPlex logo and Author avatar if they exist in standard paths
    # (Optional: these could be merged into the loop if we pass multiple directories)
    
    # Print header
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  SVG to PNG Converter${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "  Source:      ${BLUE}$SRC_DIR${NC}"
    echo -e "  Output:      ${BLUE}$OUTPUT_DIR${NC}"
    echo -e "  Zoom:        ${BLUE}${ZOOM}x${NC}"
    if [ -n "$BACKGROUND" ]; then
        echo -e "  Background:  ${BLUE}$BACKGROUND${NC}"
    else
        echo -e "  Background:  ${BLUE}transparent${NC}"
    fi
    echo -e "  Files:       ${BLUE}$svg_count${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    
    # Convert each SVG in SRC_DIR
    for svg_file in "$SRC_DIR"/*.svg; do
        if [ -f "$svg_file" ]; then
            convert_svg "$svg_file"
        fi
    done

    # Specifically convert project-specific assets if found
    local EXTRA_ASSETS=(
        "docs-site/public/author-avatar.svg"
        "docs-site/public/logo.svg"
        ".github/assets/hotplex-logo.svg"
    )

    for asset in "${EXTRA_ASSETS[@]}"; do
        if [ -f "$asset" ]; then
            convert_svg "$asset"
        fi
    done
    
    echo ""
    echo -e "${GREEN}✓ Done! $svg_count files converted to $OUTPUT_DIR${NC}"
    echo ""
}

main
