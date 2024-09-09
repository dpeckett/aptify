# aptify

Probably the quickest, and easiest, way to create a Debian APT repository from
a list of deb files.

## Installation

### From APT

Add my [apt repository](https://github.com/dpeckett/apt.dpeckett.dev?tab=readme-ov-file#usage) to your system.

Then install aptify:

*Currently packages are only published for Debian 12 (Bookworm).*

```shell
sudo apt update
sudo apt install aptify
```

### GitHub Releases

Download statically linked binaries from the GitHub releases page: 

[Latest Release](https://github.com/dpeckett/aptify/releases/latest)

## Usage

### Initialize Keys

You'll need a GPG key to sign your repository. If you don't have one, you can
create one using the `init-keys` command:

```shell
aptify init-keys
```

The resulting key will be written to your `$XDG_CONFIG_HOME/aptify/` directory. You should back this up somewhere safe.

### Create Repository

You'll need a simple YAML file describing the repository you want to create.

A demonstration file is provided in the examples directory. Schema for the
repository configuration is defined in the 
[v1alpha1/types.go](./internal/config/v1alpha1/types.go) file.

```shell
aptify build -c examples/demo.yaml -d ./demo-repo
```

This will create a directory called `demo-repo` containing the repository.

### Serve Repository

The recommended way to serve the repository is to use [caddy](https://caddyserver.com).

An example Caddyfile is provided below, replace `apt.example.com` with your domain:

```
https://apt.example.com {
  root * /var/lib/aptify/repo
  file_server {
    browse 
  }
}

http://apt.example.com {
  root * /var/lib/aptify/repo

  # Don't serve the signing key over insecure connections.
  handle_path "/signing_key.asc" {
    redir https://{host}{uri}
  }

  handle {
    root * /var/lib/aptify/repo
    file_server {
      browse
    }
  }
}
```

### Use Repository

To use the repository, you'll need to add a new apt source to your system. You
can do this by downloading the signing key and adding the repository to your
`/etc/apt/sources.list.d` directory.

```shell
curl -fsL https://apt.example.com/signing_key.asc | sudo tee /etc/apt/keyrings/demo-repo-keyring.asc > /dev/null
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/demo-repo-keyring.asc] http://apt.example.com/ $(. /etc/os-release && echo $VERSION_CODENAME) stable" | sudo tee /etc/apt/sources.list.d/demo-repo.list > /dev/null
```

Packages can now be installed from the repository.

```shell
sudo apt update
sudo apt install hello-world
```

## Telemetry

By default aptify gathers anonymous crash and usage statistics. This anonymized
data is processed on our servers within the EU and is not shared with third
parties. You can opt out of telemetry by setting the `DO_NOT_TRACK=1`
environment variable.