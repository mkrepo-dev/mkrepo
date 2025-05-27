# mkrepo

mkrepo is tools used for templating new git repositories hosted on one of popular VCS such as GitHub.

## Run

```sh
task run
```

### Config

Generate base config

```sh
task config:default
```

Validate config.yaml against schema defined using KCL

```sh
task config:validate
```

## Misc

Generate safe secrets: `openssl rand -hex 32`
