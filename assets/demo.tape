# OpenDecree quickstart demo: schema → config → watch
#
# Renders to assets/demo.gif. Requires a running decree service at
# localhost:9090 and the decree CLI on PATH. See assets/README.md for
# how to render (`make demo-gif`).

Output assets/demo.gif

Require decree

Set FontSize 14
Set Width 1200
Set Height 400
Set Padding 20
Set Theme "GitHub Dark"
Set TypingSpeed 50ms

# Default role + subject for metadata-header auth (no JWT required).
Env DECREE_SUBJECT "demo"

# 1. Peek at the seed file — schema + tenant + initial config in one doc.
Type "head -20 examples/demo.seed.yaml"
Sleep 1000ms
Enter
Sleep 3200ms

# 2. One command bootstraps the schema, tenant, and config from that YAML.
Type "decree seed examples/demo.seed.yaml --auto-publish"
Sleep 200ms
Enter
Sleep 2400ms

# 3. Look at the seeded tenant.
Type "decree tenant list"
Sleep 200ms
Enter
Sleep 2400ms

# 4. Read a config value.
Type "decree config get acme-corp app.name"
Sleep 100ms
Enter
Sleep 2400ms

# 5. Update the value.
Type 'decree config set acme-corp app.name "Acme v2"'
Sleep 100ms
Enter
Sleep 2400ms

# 6. Read it back — now shows the new value.
Type "decree config get acme-corp app.name"
Sleep 100ms
Enter
Sleep 2400ms
