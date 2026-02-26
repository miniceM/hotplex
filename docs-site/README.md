# HotPlex Documentation Site (Generated)

> [!CAUTION]
> **DO NOT EDIT THIS DIRECTORY MANUALLY.**

This directory contains the source for the [HotPlex Documentation Site](https://hrygo.github.io/hotplex/). 

### Source of Truth (SSOT)
All documentation in this directory is automatically generated from the **Single Source of Truth (SSOT)** located in the root `docs/` folder and other repository locations (e.g., SDK READMEs).

### Synchronization
To update the documentation site, edit the files in the root `docs/` directory and run the synchronization script:
```bash
bash scripts/sync_docs.sh
```

### Note for AI Agents
If you are an AI agent, please perform all documentation maintenance in the root `docs/` folder. Any changes made directly in `docs-site/` will be **OVERWRITTEN** during the next synchronization.
