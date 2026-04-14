---
title: decree completion bash
---

## decree completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(decree completion bash)

To load completions for every new session, execute once:

#### Linux:

	decree completion bash > /etc/bash_completion.d/decree

#### macOS:

	decree completion bash > $(brew --prefix)/etc/bash_completion.d/decree

You will need to start a new shell for this setup to take effect.


```
decree completion bash
```

### Options

```
  -h, --help              help for bash
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

