---
title: decree completion zsh
---

## decree completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(decree completion zsh)

To load completions for every new session, execute once:

#### Linux:

	decree completion zsh > "${fpath[1]}/_decree"

#### macOS:

	decree completion zsh > $(brew --prefix)/share/zsh/site-functions/_decree

You will need to start a new shell for this setup to take effect.


```
decree completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
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

