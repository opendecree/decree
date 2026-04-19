# OpenDecree quickstart demo: schema → config → watch
#
# Renders to assets/demo.gif. Requires a running decree service at
# localhost:9090 and the decree CLI on PATH. See assets/README.md for
# how to render (`make demo-gif`).

Output assets/demo.gif

Require decree

Set FontSize 14
Set Width 1200
Set Height 360
Set Padding 20
Set Theme "GitHub Dark"
Set TypingSpeed 40ms

# Default role + subject for metadata-header auth (no JWT required).
Env DECREE_SUBJECT "demo"

# 1. One command bootstraps a schema, tenant, and initial config from YAML.
Type "decree seed examples/seed.yaml --auto-publish"
Enter
Sleep 2200ms

# 2. Look at the seeded tenant.
Type "decree tenant list"
Enter
Sleep 2200ms

# 3. Read a config value.
Type "decree config get acme-corp app.name"
Enter
Sleep 2s

# 4. Update the value — the watcher prints the change event.
Type 'decree config set acme-corp app.name "Acme v2"'
Enter
Sleep 2200ms

# 5. Read it back — now shows the new value.
Type "decree config get acme-corp app.name"
Enter
Sleep 2200ms
