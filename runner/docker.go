package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

var imageMap = map[string]string{
	"ubuntu-latest": "catthehacker/ubuntu:act-latest",
	"ubuntu-24.04":  "catthehacker/ubuntu:act-24.04",
	"ubuntu-22.04":  "catthehacker/ubuntu:act-22.04",
	"ubuntu-20.04":  "catthehacker/ubuntu:act-20.04",
}

type Container struct {
	cli *client.Client
	id  string
	ctx context.Context
}

func startContainer(ctx context.Context, job Job) (*Container, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("connecting to Docker: %w\nIs Docker running?", err)
	}

	// Explicit image (e.g. from GitLab CI image: field) takes priority.
	image := job.Image
	if image == "" {
		var ok bool
		image, ok = imageMap[job.RunsOn]
		if !ok {
			fmt.Printf("  ⚠  No image mapping for %q — falling back to ubuntu:22.04\n", job.RunsOn)
			image = "ubuntu:22.04"
		}
	}

	fmt.Printf("  Pulling image %s (this may take a minute the first time)...\n", image)
	reader, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return nil, fmt.Errorf("pulling image %s: %w", image, err)
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	workdir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:      image,
			Cmd:        []string{"tail", "-f", "/dev/null"},
			WorkingDir: "/workspace",
		},
		&container.HostConfig{
			Binds: []string{workdir + ":/workspace"},
		},
		nil, nil, "",
	)
	if err != nil {
		return nil, fmt.Errorf("creating container: %w", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("starting container: %w", err)
	}

	fmt.Printf("  Container started: %s\n", resp.ID[:12])
	return &Container{cli: cli, id: resp.ID, ctx: ctx}, nil
}

func (c *Container) exec(command string, env map[string]string, workingDir string, timeout time.Duration) (int, string, error) {
	var envSlice []string
	for k, v := range env {
		envSlice = append(envSlice, k+"="+v)
	}

	wd := "/workspace"
	if workingDir != "" {
		if strings.HasPrefix(workingDir, "/") {
			wd = workingDir
		} else {
			wd = "/workspace/" + workingDir
		}
	}

	execResp, err := c.cli.ContainerExecCreate(c.ctx, c.id, types.ExecConfig{
		Cmd:          []string{"/bin/bash", "-e", "-c", command},
		Env:          envSlice,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   wd,
	})
	if err != nil {
		return -1, "", fmt.Errorf("creating exec: %w", err)
	}

	attach, err := c.cli.ContainerExecAttach(c.ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return -1, "", fmt.Errorf("attaching to exec: %w", err)
	}
	defer attach.Close()

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(io.MultiWriter(os.Stdout, &buf), io.MultiWriter(os.Stderr, &buf), attach.Reader)
		done <- err
	}()

	if timeout > 0 {
		select {
		case <-done:
			// finished normally
		case <-time.After(timeout):
			attach.Close()
			return -1, buf.String(), fmt.Errorf("step timed out after %v", timeout)
		}
	} else {
		<-done
	}

	inspect, err := c.cli.ContainerExecInspect(c.ctx, execResp.ID)
	if err != nil {
		return -1, "", err
	}
	return inspect.ExitCode, buf.String(), nil
}

func (c *Container) dropShell() error {
	fmt.Println("\n  Dropping into container shell. Your project is at /workspace.")
	fmt.Println("  Type 'exit' to return to cidb.")

	cmd := exec.Command("docker", "exec", "-it", c.id, "/bin/bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return fmt.Errorf("shell: %w", err)
		}
	}
	return nil
}

func (c *Container) stop() {
	c.cli.ContainerStop(c.ctx, c.id, container.StopOptions{})
	c.cli.ContainerRemove(c.ctx, c.id, types.ContainerRemoveOptions{Force: true})
}
