// Package examples runs examples/<id>/main.go as an integrator would: the key comes only from
// YONA_API_KEY. Used by scripts/run_examples. Not shipped. The key is passed
// through the child's environment and never printed.
//
// The examples are the code the guides show: they pass only the API key, so they talk to the default
// host. To run them against another gateway without changing a line of them, Run links the example
// with `-X github.com/elyonar/einvoice-go.defaultBaseURLOverride=<YONA_BASE_URL>`: the SDK itself
// never reads the environment.
package examples

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Order is the example ids in the order the guides list them.
var Order = []string{"first-invoice", "webhooks", "errors-and-retries", "sandbox-and-live", "received-invoices"}

// Root finds the repository root (the directory holding go.mod) from the working directory.
func Root() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found: run from the repository")
		}
		dir = parent
	}
}

// IDs lists the examples (directories of examples/ holding a main.go), Order first, the rest sorted.
func IDs(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "examples"))
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(root, "examples", e.Name(), "main.go")); err == nil {
				present[e.Name()] = true
			}
		}
	}
	var out, rest []string
	for _, id := range Order {
		if present[id] {
			out = append(out, id)
			delete(present, id)
		}
	}
	for id := range present {
		rest = append(rest, id)
	}
	sort.Strings(rest)
	return append(out, rest...), nil
}

// Result is the outcome of one example run.
type Result struct {
	ID     string
	OK     bool
	Millis int64
	Output string
}

// Run builds and runs one example; it never panics or returns an error, the Result says what happened.
func Run(root, id, apiKey, baseURL string, timeout time.Duration) Result {
	started := time.Now()
	result := func(ok bool, output string) Result {
		if apiKey != "" {
			output = strings.ReplaceAll(output, apiKey, "sk_***") // defence in depth: an example never prints the key
		}
		return Result{ID: id, OK: ok, Millis: time.Since(started).Milliseconds(), Output: output}
	}
	binDir, err := os.MkdirTemp("", "einvoice-go-example-")
	if err != nil {
		return result(false, err.Error())
	}
	defer os.RemoveAll(binDir)
	bin := filepath.Join(binDir, id)
	args := []string{"build", "-o", bin}
	if baseURL != "" {
		args = append(args, "-ldflags", "-X github.com/elyonar/einvoice-go.defaultBaseURLOverride="+strings.TrimRight(baseURL, "/"))
	}
	args = append(args, "./examples/"+id)
	build := exec.Command("go", args...)
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		return result(false, "build failed: "+err.Error()+"\n"+string(out))
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Dir = root
	env := []string{}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "YONA_API_KEY=") && !strings.HasPrefix(kv, "YONA_BASE_URL=") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "YONA_API_KEY="+apiKey)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	if ctx.Err() != nil {
		return result(false, output.String()+"\nkilled after "+timeout.String())
	}
	return result(err == nil, output.String())
}
