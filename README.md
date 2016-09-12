# nomad-docker-wrapper

This is a hack to allow Docker bind mounts in Nomad. Inspired by how
[Weave](https://www.weave.works/) interacts with the Docker daemon by wrapping
the Docker socket and augment certain API calls with extra network
configuration, this tool allow us to specify Docker bind mounts as environment
variables.

## Requirements

 * The Docker socket must listen on unix:///var/run/docker.sock

## Building

```
make
```

## Usage

Run `sudo ./nomad-docker-wrapper`. This will create a new unix socket at
`/tmp/nomad-docker-wrapper.sock` that we can tell Nomad to use by setting
[docker.endpoint](https://www.nomadproject.io/docs/drivers/docker.html#docker_endpoint)
to `unix:///tmp/nomad-docker-wrapper.sock`.

Now we can create Nomad tasks like this:
```
task "webservice" {
    driver = "docker"
    config = {
        image = "redis"
    }
    env = {
        DOCKER_BIND_MOUNT = "/some-path:/some-path:ro"
    }
}
```

When Nomad tell Docker to create this container, nomad-docker-wrapper will
modify the API request body by removing the `DOCKER_BIND_MOUNT` environemnt
variable, and add it as an bind mount instead! Kind of ugly, but it seems to
work :-) If you want multiple binds you can just suffix the key with something,
for example numbers:
```
env = {
	DOCKER_BIND_MOUNT_1 = "/some-path:/some-path:ro"
	DOCKER_BIND_MOUNT_2 = "/some-other-path:/some-other-path:rw"
	DOCKER_BIND_MOUNT_3 = "/you/get/the/point:/meh:rw"
}
```

