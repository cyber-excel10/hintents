import os
import glob

replacements = {
    "✅": "[OK]",
    "❌": "[FAIL]",
    "[WARN]": "[WARN]",
    "✓": "[OK]",
    "✗": "[FAIL]",
    "⚡": "*",
    "🚀": "",
    "🔍": "[SEARCH]",
    "➡️": "->",
    "⬅️": "<-",
    "🎯": "[TARGET]",
    "📍": "[LOC]",
    "🔧": "[TOOL]",
    "📊": "[STATS]",
    "📋": "[LIST]",
    "▶️": "[PLAY]",
    "📖": "[DOC]",
    "👋": "[HELLO]",
    "📡": "[NET]",
}

root_dir = "/Users/khabibthekillys./Documents/change/lib/stellar/erst/sdk"

for root, dirs, files in os.walk(root_dir):
    for file in files:
        if file.endswith(".md"):
            filepath = os.path.join(root, file)
            try:
                with open(filepath, 'r', encoding='utf-8') as f:
                    content = f.read()
                
                new_content = content
                for emoji, replacement in replacements.items():
                    new_content = new_content.replace(emoji, replacement)
                
                if content != new_content:
                    with open(filepath, 'w', encoding='utf-8') as f:
                        f.write(new_content)
                    print(f"Updated {filepath}")
            except Exception as e:
                print(f"Error processing {filepath}: {e}")
