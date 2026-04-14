---
title: decree completion powershell
---

## decree completion powershell

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	decree completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
decree completion powershell [flags]
```

### Options

```
  -h, --help              help for powershell
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

