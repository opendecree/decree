---
title: decree completion fish
---

## decree completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	decree completion fish | source

To load completions for every new session, execute once:

<<<<<<< HEAD:docs/cli/decree_completion_fish.md
	decree completion fish > ~/.config/fish/completions/decree.fish
=======
	decree completion fish > ~/.config/fish/completions/ccs.fish
>>>>>>> origin/main:docs/cli/ccs_completion_fish.md

You will need to start a new shell for this setup to take effect.


```
decree completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --insecure           skip TLS verification (default true)
  -o, --output string      output format: table, json, yaml (default "table")
      --role string        actor role (x-role header) (default "superadmin")
      --server string      gRPC server address (default "localhost:9090")
      --subject string     actor identity (x-subject header)
      --tenant-id string   auth tenant ID (x-tenant-id header)
      --token string       JWT bearer token
```

### SEE ALSO

* [decree completion](decree_completion.md)	 - Generate the autocompletion script for the specified shell

