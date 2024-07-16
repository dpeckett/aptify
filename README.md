# aptify

Probably the quickest, and easiest, way to create a Debian apt repository from
a list of deb files.

## Usage

### Initialize Keys

You'll need a GPG key to sign your repository. If you don't have one, you can
create one using the `init-keys` command:

```shell
aptify init-keys
```

The resulting keys will be written to your `$XDG_CONFIG_HOME/aptify` directory.

### Create Repository

You'll need a simple YAML file describing the repository you want to create.

A demonstration file is provided in the examples directory. Schema for the
repository configuration is defined in the 
[v1alpha1/types.go](./internal/config/v1alpha1/types.go) file.

```shell
aptify build -c examples/demo.yaml -o ./demo-repo
```

This will create a directory called `demo-repo` containing the repository.

### Serve Repository

You can serve the repository using any web server you like. However for 
convenience, aptify includes an embedded web server that you can use to serve 
the repository.

To start a server listening on `http://localhost:8080`:

```shell
aptify serve -d ./demo-repo
```

You can enable HTTPS support by passing the `--tls` flag and providing a 
domain/email for Let's Encrypt certificate issuance.

### Use Repository

To use the repository, you'll need to add a new apt source to your system. You
can do this by downloading the signing key and adding the repository to your
`/etc/apt/sources.list.d` directory.

In a production setting the signing key should be downloaded over HTTPS.

```shell
curl -fsL http://localhost:8080/signing_key.asc | sudo tee /etc/apt/keyrings/demo-repo-keyring.asc > /dev/null
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/demo-repo-keyring.asc] http://localhost:8080/ $(. /etc/os-release && echo $VERSION_CODENAME) stable" | sudo tee /etc/apt/sources.list.d/demo-repo.list > /dev/null
```

Packages can now be installed from the repository.

```shell
sudo apt update
sudo apt install hello-world
```
