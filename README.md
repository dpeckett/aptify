# aptify

Probably the quickest, and easiest, way to create a Debian apt repository from
a list of deb files.

Kind of a lightweight/minimal alternative to [reprepro](https://salsa.debian.org/brlink/reprepro).

## Usage

### Initialize GPG Keys

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
[./internal/config/v1alpha1/types.go](./internal/config/v1alpha1/types.go) file.

```shell
aptify build -c examples/demo.yaml -o ./my-awesome-repo
```

This will create a directory called `my-awesome-repo` containing the repository.

### Serve Repository

You can serve the repository using any web server you like. However for convenience,
aptify includes a simple embedded web server that you can use to serve the 
repository.

```shell
aptify serve -d ./my-awesome-repo
```

You can connect to the server by visiting [http://localhost:8080](http://localhost:8080) 
in your browser.


You can enable HTTPS support by passing the `--tls` flag and providing a public 
domain and your email for Let's Encrypt certificate issuance.

### Use Repository

You can then add the repository to your sources.list file.

```shell
curl -fsL http://localhost:8080/signing_key.asc | sudo tee /etc/apt/keyrings/my-awesome-repo-keyring.asc > /dev/null
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/my-awesome-repo-keyring.asc] http://localhost:8080/ bookworm stable" | sudo tee /etc/apt/sources.list.d/my-awesome-repo.list > /dev/null
sudo apt update
```

You can then install packages from the repository:

```shell
sudo apt install hello-world
```
