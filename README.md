# aptify

Probably the quickest, and easiest, way to create a Debian apt repository from
a list of deb files.

Kind of a lightweight/minimal alternative to [reprepro](https://salsa.debian.org/brlink/reprepro).

## Usage

### Create GPG Key For Signing Releases

You'll need a GPG key to sign your repository. If you don't have one, you can
create one using the `init-keys` command:

```shell
aptify init-keys
```

The resulting keys will be written to your $XDG_STATE_HOME/aptify directory.

### Create Repository

You'll need a simple YAML file describing the repository you want to create.

A demonstration file is provided in the examples directory. Schema for the
repository configuration is defined in the 
[./internal/config/v1alpha1/types.go](./internal/config/v1alpha1/types.go) file.

```shell
aptify build -c examples/demo.yaml -o ./my-awesome-repo
```

This will create a directory called `my-awesome-repo` containing the repository.

### Serve The Repository

You can serve the repository using any web server you like. For example, using
Python's built-in HTTP server:

```shell
cd my-awesome-repo
python3 -m http.server
```

For security, in a production environment you should serve the signing key over
HTTPS.

Then you can add the repository to your sources.list file:

```shell
curl -fsL  http://localhost:8000/signing_key.asc | sudo tee /etc/apt/keyrings/my-awesome-repo-keyring.asc > /dev/null
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/my-awesome-repo-keyring.asc] http://localhost:8000/ bookworm stable" | sudo tee /etc/apt/sources.list.d/my-awesome-repo.list > /dev/null
```

You can then update your package list and install packages from the repository:

```shell
sudo apt update
sudo apt install hello-world
```
