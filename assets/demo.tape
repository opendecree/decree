# OpenDecree quickstart demo: schema → config → watch
#
# Renders to assets/demo.gif. Requires a running decree service at
# localhost:9090 and the decree CLI on PATH. See assets/README.md for
# how to render (`make demo-gif`).

Output assets/demo.gif

Require decree

Set FontSize 14
Set Width 1200
Set Height 720
Set Padding 20
Set Theme "GitHub Dark"
Set TypingSpeed 40ms

# Default role + subject for metadata-header auth (no JWT required).
Env DECREE_SUBJECT "demo"

# 1. One command bootstraps a schema, tenant, and initial config from YAML.
Type "decree seed examples/seed.yaml --auto-publish"
Enter
Sleep 2s

# 2. Look at the seeded tenant.
Type "decree tenant list"
Enter
Sleep 2s

# 3. Grab the tenant ID for subsequent commands.
Type "TENANT=$(decree tenant list | awk '/acme-corp/ { print $1 }')"
Enter
Sleep 500ms

# 4. Read a config value.
Type "decree config get $TENANT app.name"
Enter
Sleep 2s

# 5. Start a live watcher in the background — events print inline.
Type "decree watch $TENANT &"
Enter
Sleep 2s

# 6. Update the value — the watcher prints the change event.
Type 'decree config set $TENANT app.name "Acme v2"'
Enter
Sleep 3s

# 7. Read it back — now shows the new value.
Type "decree config get $TENANT app.name"
Enter
Sleep 2s
