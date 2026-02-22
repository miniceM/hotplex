#!/bin/bash
set -e

SVG_FILE="docs/images/logo.svg"
OUTPUT_DIR=".github/assets"

mkdir -p $OUTPUT_DIR

echo "0/3: 同步源 SVG 到资产目录..."
cp $SVG_FILE $OUTPUT_DIR/hotplex-logo.svg

echo "1/3: 渲染基础高清 PNG..."
# 针对暗色背景提取白色文字版本的 SVG
TMP_SVG="$OUTPUT_DIR/logo-dark-temp.svg"
sed -e 's/class="hot-stop-1"/stop-color="#FFFFFF"/' \
    -e 's/class="hot-stop-2"/stop-color="#E6EDF3"/' \
    $SVG_FILE > $TMP_SVG

# 强制输出 1024x1024 高清底图，保留透明度
cairosvg $TMP_SVG -W 1024 -H 1024 -o $OUTPUT_DIR/logo-base.png
rm $TMP_SVG

echo "2/3: 生成 Open Graph 社交预览图 (1200x630)..."
# 居中放置 Logo，填充 GitHub 暗色背景 (#0D1117)，适配社交卡片规范
magick -size 1200x630 xc:"#0D1117" \
  \( $OUTPUT_DIR/logo-base.png -resize 600x600 \) \
  -gravity center -composite \
  $OUTPUT_DIR/hotplex-og.png

echo "3/3: 生成多尺寸 favicon.ico..."
# 从高清底图生成包含 16, 32, 48, 64, 128, 256 尺寸的完美 ICO
magick -background none $OUTPUT_DIR/logo-base.png -define icon:auto-resize=256,128,64,48,32,16 $OUTPUT_DIR/favicon.ico

# 清理中间产物
rm $OUTPUT_DIR/logo-base.png

echo "完成！资产已存放在 $OUTPUT_DIR 目录下。"
