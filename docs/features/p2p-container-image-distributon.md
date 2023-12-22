# P2P Container Image Distribution

This feature allows a user to set up OpenRegistry in P2P mode via IPFS, enabling multiple deployment & usage scenarios 
for them. One such example is OpenRegistry backed by a private IPFS cluster within their network. This allows for 
faster, and secure image distribution where the underlying storage is completely managed by the user.

## GitHub Pull Requests:

- Backend - [PR #490](https://github.com/containerish/OpenRegistry/pull/490)

## How to use this feature

This is a backend-only feature. This means that when you deploy OpenRegistry, you can configure OpenRegistry to run in
P2P mode. This allows you to control the DFS usage of OpenRegistry, increase/decrease the disk sizes, and run OpenRegistry
in Self-deployed mode while being backed by the reliable distributed storage system provided by IPFS.
To enable the P2P mode in OpenRegistry, please add the following snippet into your `config.yaml` file for OpenRegistry:

> [!WARNING]
> When OpenRegistry runs in P2P mode, there's user controls for Push or Pull operations. 
> Anyone can push to any repository which means you must configure network rules to allow only specified users.


```yaml
dfs:
  ipfs:
    enabled: true
    type: "p2p"
    local: true
    pinning: false
    gateway_endpoint: <ipfs-gateway-address>
```

Then you can push any container image to IPFS via P2P mode. Here's how to do that:
1. Tag your container image with the username of `ipfs`. This is a static username and OpenRegistry manages this user internally.
```bash
docker tag ubuntu:latest <openregistry-endpoint>/ipfs/ubuntu:latest
```

2. Push your container image:
```bash
docker push <openregistry-endpoint>/ipfs/ubuntu:latest
```

3. Then if your network allows, you can pull the container image on another machine using the following command:
```bash
docker pull <openregistry-endpoint>/ipfs/ubuntu:latest
```
