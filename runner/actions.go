package runner

import (
	"fmt"
	"strings"
)

// runAction handles known `uses:` steps instead of skipping them.
// Returns (handled, error) — if handled is false, the caller should skip.
func runAction(ctr *Container, step Step) (handled bool, err error) {
	action := strings.Split(step.Uses, "@")[0]

	switch {
	case action == "actions/checkout", action == "circleci/checkout":
		// Workspace is already mounted at /workspace — nothing to do.
		fmt.Println("  (checkout — workspace already mounted at /workspace)")
		return true, nil

	// CircleCI built-ins we silently skip (caching, artifacts, etc.)
	case strings.HasPrefix(action, "circleci/"):
		fmt.Printf("  (circleci built-in %q — skipped)\n", action)
		return true, nil

	case action == "actions/setup-go":
		return true, setupGo(ctr, step.With)

	case action == "actions/setup-python":
		return true, setupPython(ctr, step.With)

	case action == "actions/setup-node":
		return true, setupNode(ctr, step.With)

	case action == "actions/setup-java":
		return true, setupJava(ctr, step.With)

	default:
		return false, nil
	}
}

func setupPython(ctr *Container, with map[string]string) error {
	version := with["python-version"]
	if version == "" {
		version = "3"
	}
	version = strings.TrimLeft(version, "~^>=")

	fmt.Printf("  (actions/setup-python — installing Python %s in container)\n", version)

	script := `
set -e
apt-get update -qq
apt-get install -y -qq python3 python3-pip > /dev/null 2>&1
ln -sf /usr/bin/python3 /usr/local/bin/python
python --version
pip3 --version
`

	exitCode, _, err := ctr.exec(script, nil)
	if err != nil {
		return fmt.Errorf("setup-python: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("setup-python exited with code %d", exitCode)
	}
	return nil
}

func setupJava(ctr *Container, with map[string]string) error {
	version := with["java-version"]
	if version == "" {
		version = "17"
	}
	version = strings.TrimLeft(version, "~^>=")

	fmt.Printf("  (actions/setup-java — installing Java %s in container)\n", version)

	script := fmt.Sprintf(`
set -e
apt-get update -qq
apt-get install -y -qq wget apt-transport-https > /dev/null 2>&1
apt-get install -y -qq openjdk-%s-jdk > /dev/null 2>&1
apt-get install -y -qq maven > /dev/null 2>&1
java -version
mvn -version
`, version)

	exitCode, _, err := ctr.exec(script, nil)
	if err != nil {
		return fmt.Errorf("setup-java: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("setup-java exited with code %d", exitCode)
	}
	return nil
}

func setupNode(ctr *Container, with map[string]string) error {
	version := with["node-version"]
	if version == "" {
		version = "20"
	}
	version = strings.TrimLeft(version, "~^>=v")
	// Only keep major version for nvm install
	major := strings.Split(version, ".")[0]

	fmt.Printf("  (actions/setup-node — installing Node.js %s in container)\n", major)

	script := fmt.Sprintf(`
set -e
apt-get update -qq
apt-get install -y -qq curl > /dev/null 2>&1
curl -fsSL https://deb.nodesource.com/setup_%s.x | bash - > /dev/null 2>&1
apt-get install -y -qq nodejs > /dev/null 2>&1
node --version
npm --version
`, major)

	exitCode, _, err := ctr.exec(script, nil)
	if err != nil {
		return fmt.Errorf("setup-node: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("setup-node exited with code %d", exitCode)
	}
	return nil
}

func setupGo(ctr *Container, with map[string]string) error {
	version := with["go-version"]
	if version == "" {
		version = "1.22.0"
	}
	// Strip leading ~, ^, or >= if present
	version = strings.TrimLeft(version, "~^>=")
	// If it's only major.minor (e.g. "1.22"), append .0
	if strings.Count(version, ".") == 1 {
		version += ".0"
	}

	fmt.Printf("  (actions/setup-go — installing Go %s in container)\n", version)

	script := fmt.Sprintf(`
set -e
apt-get update -qq
apt-get install -y -qq curl tar > /dev/null 2>&1
curl -fsSL "https://go.dev/dl/go%s.linux-amd64.tar.gz" -o /tmp/go.tar.gz
tar -C /usr/local -xzf /tmp/go.tar.gz
rm /tmp/go.tar.gz
ln -sf /usr/local/go/bin/go /usr/local/bin/go
ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
go version
`, version)

	exitCode, _, err := ctr.exec(script, nil)
	if err != nil {
		return fmt.Errorf("setup-go: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("setup-go exited with code %d", exitCode)
	}
	return nil
}
