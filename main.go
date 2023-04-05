/*
This package is meant to be invoked by go generate and acts as a build
script for v8. Follow the official v8 instructions to install depot_tools
and whatever v8's build dependencies are before running this package.

https://v8.dev/docs/build
*/
package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

//go:generate go run .

// gnArgs are basic arguments for the gn build system that we
// always want to use.
const gnArgs = `
is_debug=false
symbol_level=0
strip_debug_info=0
clang_use_chrome_plugins=false
use_custom_libcxx=false
use_sysroot=false
is_component_build=false
v8_monolithic=true
v8_use_external_startup_data=false
treat_warnings_as_errors=false
v8_embedder_string="-v8go"
v8_enable_gdbjit=false
v8_enable_test_features=false
exclude_unwind_tables=true
`

// gnArgsForArch returns the gn config contents to build for the given arch.
func gnArgsForArch(arch string) string {
	outArgs := gnArgs
	switch arch {
	case "arm64":
		outArgs += "target_cpu=\"arm64\"\n"
		outArgs += "v8_target_cpu=\"arm64\"\n"
	case "amd64":
		outArgs += "target_cpu=\"x64\"\n"
		outArgs += "v8_target_cpu=\"x64\"\n"
	}
	return outArgs
}

// sh executes a subprocess. Unlike a proper shell, no argument interpolation
// or expansion is performed.
func sh(args ...string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("[go] sh: %q", args)
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

// cd changes the current working directory.
func cd(dir string) {
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	log.Printf("[go] cd: %q", dir)
}

// isDir returns whether the provided path exists as a directory.
func isDir(dir string) bool {
	stats, err := os.Lstat(dir)
	if err != nil {
		return false
	}
	return stats.IsDir()
}

// mkdir creates a directory hierarchy if it doesn't already exist.
func mkdir(dir string) {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		panic(err)
	}
	log.Printf("[go] mkdir: %q", dir)
}

// writeFile creates a new file with the given name and contents,
// overwriting any existing file with the same name.
func writeFile(file string, data string) {
	err := os.WriteFile(file, []byte(data), 0o644)
	if err != nil {
		panic(err)
	}
	log.Printf("[go] write: %q", file)
}

// cp copies the file at src to dest.
func cp(src, dest string) {
	srcFile, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dest)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := dstFile.Close(); err != nil {
			panic(err)
		}
	}()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		panic(err)
	}
	log.Printf("[go] cp %q %q", src, dest)
}

// pwd returns the current working directory.
func pwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	log.Printf("[go] pwd: %q", cwd)
	return cwd
}

func main() {
	startDir := pwd()
	// Update v8 repo.
	if !isDir("v8") {
		sh("fetch", "v8")
	}
	cd("v8")
	sh("gclient", "sync")
	// Build v8.
	arches := []string{"amd64"}
	if runtime.GOOS == "darwin" {
		// Build for both major macOS architectures.
		arches = append(arches, "arm64")
	}
	outDirs := []string{}

	const (
		initialArchiveName = "libv8_monolith.a"
		finalArchiveName   = "libv8.a"
	)

	// Build for each architecture.
	for _, arch := range arches {
		outDir := filepath.Join(startDir, runtime.GOOS+"_"+arch)
		outDirs = append(outDirs, outDir)
		mkdir(outDir)
		dir := filepath.Join("out", "release."+arch)
		mkdir(dir)
		writeFile(filepath.Join(dir, "args.gn"), gnArgsForArch(arch))
		sh("gn", "gen", dir)
		sh("ninja", "-C", dir, "v8_monolith")
		cp(
			filepath.Join(dir, "obj", initialArchiveName),
			filepath.Join(outDir, finalArchiveName),
		)
	}
}
