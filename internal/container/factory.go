package container

// Only the Docker workspace backend is supported. Other backends
// (containerd, kubernetes, apple) were removed when the server moved to a
// host-process deployment model.
const BackendDocker = "docker"

func NormalizeBackend(backend string) string {
	return backend
}
