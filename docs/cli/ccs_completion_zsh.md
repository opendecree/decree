---
title: ccs completion zsh
---

## ccs completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(ccs completion zsh)

To load completions for every new session, execute once:

#### Linux:

	ccs completion zsh > "${fpath[1]}/_ccs"

#### macOS:

	ccs completion zsh > $(brew --prefix)/share/zsh/site-functions/_ccs

You will need to start a new shell for this setup to take effect.


```
ccs completion zsh [flags]
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

* [ccs completion](ccs_completion.md)	 - Generate the autocompletion script for the specified shell

