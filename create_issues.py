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

for line in lines:
    # Match the title line format exactly: e.g., "1. [UI] Add split-pane view..."
    # It must start with a number, a period, space, and a bracket.
    m = re.match(r'^(\d+)\.\s+(\[.*?].*)$', line)
    if m:
        if current_title:
            issues.append({"title": current_title, "body": "".join(current_body).strip()})
        current_title = m.group(2).strip()
        current_body = []
    else:
        if current_title is not None:
            current_body.append(line)

if current_title:
    issues.append({"title": current_title, "body": "".join(current_body).strip()})

print(f"Found {len(issues)} issues to create.")
if not issues:
    print("No issues parsed. Exiting.")
    sys.exit(0)

print(f"Example First Title: {issues[0]['title']}")

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
        # Sleep to be nice to GitHub's rate limits
        time.sleep(1.5)
    except Exception as e:
        print(f" Exception occurred: {e}")
        break

print(f"Successfully created {created_count} issues.")
