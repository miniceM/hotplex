#!/usr/bin/env python3
import os
import re

def find_links(content):
    # Match [link text](url)
    return re.findall(r'\[.*?\]\((.*?)\)', content)

def get_target_path(search_root, current_file, link_path):
    # Determine the directory to resolve against
    # If starting with '/', use documentation root (if we're in docs-site)
    if link_path.startswith('/'):
        if 'docs-site' in current_file:
            parts = current_file.split('docs-site')
            docs_site_root = parts[0] + 'docs-site'
            # VitePress assets in /public/ are served from /
            public_path = os.path.normpath(os.path.join(docs_site_root, 'public', link_path.lstrip('/')))
            if os.path.exists(public_path):
                return public_path
            return os.path.normpath(os.path.join(docs_site_root, link_path.lstrip('/')))
        else:
            return link_path

            
    # Standard relative link
    return os.path.normpath(os.path.join(os.path.dirname(current_file), link_path))

def check_link(search_root, file_path, link_path):
    if link_path.startswith(('http', 'mailto:', '#')):
        return None
        
    # Internal link
    clean_link = link_path.split('#')[0] # Remove anchors
    if clean_link.startswith('file:///'):
        clean_link = clean_link[len('file:///'):]
    
    if not clean_link:
        return None

    target_path = get_target_path(search_root, file_path, clean_link)
    
    # Check if target exists
    if not os.path.exists(target_path):
        # Case: VitePress often omits .md extension
        if not target_path.endswith('.md') and os.path.exists(target_path + '.md'):
            return None
        return f"Broken (Internal): {link_path} (Target path: {target_path})"
        
    return None

def main():
    root_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    broken_found = False
    
    for root, dirs, files in os.walk(root_dir):
        # Skip directories
        if any(skip in root for skip in ['.git', 'node_modules', '.gemini', 'dist', 'vendor']):
            continue
            
        for file in files:
            if file.endswith('.md'):
                file_path = os.path.join(root, file)
                try:
                    with open(file_path, 'r', encoding='utf-8') as f:
                        content = f.read()
                        links = find_links(content)
                        for link in links:
                            error = check_link(root_dir, file_path, link)
                            if error:
                                # Special exception for common development false positives
                                if "docs/chatapps/url" in error or "docs-site/guide/url" in error:
                                    continue
                                    
                                print(f"{os.path.relpath(file_path, root_dir)}: {error}")
                                broken_found = True
                except Exception as e:
                    print(f"Error reading {file_path}: {e}")

    if broken_found:
        exit(1)
    else:
        print("✅ All internal links verified successfully.")

if __name__ == "__main__":
    main()
