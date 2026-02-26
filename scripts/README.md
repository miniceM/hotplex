# HotPlex Scripts

This directory contains utility scripts and Git hooks used for development, documentation, and asset generation in the HotPlex project.

## Development Tools

### Git Hooks
These scripts ensure code quality and consistent commit messages.
- `setup_hooks.sh`: Links the following hooks from `scripts/` to `.git/hooks/`.
- `pre-commit`: Runs `go fmt` and dependency checks before each commit.
- `commit-msg`: Validates that commit messages follow the Conventional Commits specification.
- `pre-push`: Performs final checks (e.g., full test suite) before pushing to remote.

### Documentation Management
- `sync_docs.sh`: Synchronizes documentation from the `docs/` and `sdks/` directories to the VitePress site (`docs-site/`). It includes advanced regex-based link transformation (e.g., converting relative code links to GitHub URLs).
- `check_links.py`: Audits internal documentation links to prevent dead links and ensure integrity.

### Asset Generation
- `generate_assets.sh`: Generates project assets like `favicon.ico`, Open Graph (OG) social preview images, and high-resolution PNGs from original SVG sources.
- `svg2png.sh`: A versatile CLI utility to convert SVG files in `docs/images` to high-resolution PNGs with customizable zoom and background colors.

## Usage

To set up the development environment hooks, run:
```bash
bash scripts/setup_hooks.sh
```

To synchronize documentation for local preview:
```bash
make docs  # This triggers scripts/sync_docs.sh
```

To verify documentation links:
```bash
python3 scripts/check_links.py
```
