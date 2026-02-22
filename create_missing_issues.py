import os
import re
import requests
import time
import sys

# Read Token
token = ""
with open(".env", "r") as f:
    for line in f:
        if line.startswith("GITHUB_TOKEN"):
            token = line.split("=")[1].strip()

if not token:
    print("Error: GITHUB_TOKEN not found in .env")
    sys.exit(1)

# Read Markdown
with open("lishus.md", "r") as f:
    lines = f.readlines()

issues = []
current_title = None
current_body = []
current_num = None

missing_numbers = {"109", "110", "111", "113", "114", "116", "118", "119"}

for line in lines:
    # Match any title line: e.g. "1. ..." or "109. docs: ..."
    m = re.match(r'^(\d+)\.\s+(.*)$', line)
    # Ensure it's a title (no indentation)
    if m and not line.startswith(" ") and not line.startswith("\t"):
        num = m.group(1)
        if current_title and current_num in missing_numbers:
            issues.append({"title": current_title, "body": "".join(current_body).strip()})
        current_title = m.group(2).strip()
        current_num = num
        current_body = []
    else:
        if current_title is not None:
            current_body.append(line)

# Add the last one
if current_title and current_num in missing_numbers:
    issues.append({"title": current_title, "body": "".join(current_body).strip()})

print(f"Found {len(issues)} missing issues to create.")
if not issues:
    print("No missing issues found. Exiting.")
    sys.exit(0)

repo = "dotandev/hintents"
url = f"https://api.github.com/repos/{repo}/issues"
headers = {
    "Authorization": f"Bearer {token}",
    "Accept": "application/vnd.github.v3+json",
}

print(f"Starting to push to {repo}...")
created_count = 0

for i, issue in enumerate(issues):
    print(f"({i+1}/{len(issues)}) Creating: {issue['title']}")
    try:
        resp = requests.post(url, headers=headers, json=issue)
        if resp.status_code == 201:
            issue_url = resp.json().get('html_url')
            print(f"  -> Created: {issue_url}")
            created_count += 1
        else:
            print(f"  -> Failed: {resp.status_code} {resp.text}")
            print("Stopping to prevent further errors.")
            break
        time.sleep(1.5)
    except Exception as e:
        print(f" Exception occurred: {e}")
        break

print(f"Successfully created {created_count} missing issues.")
