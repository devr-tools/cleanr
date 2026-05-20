package devtools

import "io"

type Runner struct {
	WorkDir string
	Stdout  io.Writer
	Stderr  io.Writer
}

type ReleaseOptions struct {
	Version string
	Output  string
}

type Platform struct {
	GOOS   string
	GOARCH string
}

func NewRunner(workDir string, stdout, stderr io.Writer) Runner {
	return Runner{
		WorkDir: workDir,
		Stdout:  stdout,
		Stderr:  stderr,
	}
}
