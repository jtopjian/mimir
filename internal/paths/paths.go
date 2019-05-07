package paths

import "path/filepath"

func DockerCompose(rootPath string) string {
	return filepath.Join(rootPath, "docker-compose.yml")
}

func Dockerfile(rootPath string) string {
	return filepath.Join(rootPath, "Dockerfile")
}

func Scripts(rootPath string) string {
	return filepath.Join(rootPath, "src/scripts")
}
