package main

import (
	"os"
	"path/filepath"
	"runtime"
)

type Kind int

const (
	KindCache Kind = iota
	KindVersion
)

// Place is one entry of the spike's built-in table. The real product reads
// these from a registry; here they are code.
type Place struct {
	Group string
	Label string
	Kind  Kind
	// Exes: the place counts as "tool present" if any of these is on PATH.
	Exes []string
	// PathCmds: candidate commands that print the directory; the first whose
	// executable exists is run.
	PathCmds [][]string
	// CmdOptional: the command only works inside a project directory, so a
	// failure says nothing and resolution continues with env / default.
	CmdOptional bool
	Env         string
	Default     func(goos string) string
	Sub         string // appended to the resolved base directory

	Special string // "docker", "rustup", "hang": measured by dedicated code
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// userCache is the per-OS location for tool caches that follow platform
// conventions.
func userCache(goos string) string {
	h := home()
	if h == "" {
		return ""
	}
	if goos == "darwin" {
		return filepath.Join(h, "Library", "Caches")
	}
	return envOr("XDG_CACHE_HOME", filepath.Join(h, ".cache"))
}

func under(rel ...string) func(string) string {
	return func(goos string) string {
		c := userCache(goos)
		if c == "" {
			return ""
		}
		return filepath.Join(append([]string{c}, rel...)...)
	}
}

func underHome(rel ...string) func(string) string {
	return func(string) string {
		h := home()
		if h == "" {
			return ""
		}
		return filepath.Join(append([]string{h}, rel...)...)
	}
}

func dataHome(goos string, rel ...string) string {
	h := home()
	if h == "" {
		return ""
	}
	if goos == "darwin" {
		return filepath.Join(append([]string{h, "Library"}, rel...)...)
	}
	return filepath.Join(append([]string{envOr("XDG_DATA_HOME", filepath.Join(h, ".local", "share"))}, rel...)...)
}

func builtinPlaces(demoHang bool) []Place {
	ps := []Place{
		{Group: "Go", Label: "build cache", Exes: []string{"go"}, PathCmds: [][]string{{"go", "env", "GOCACHE"}}, Env: "GOCACHE", Default: under("go-build")},
		{Group: "Go", Label: "module cache", Exes: []string{"go"}, PathCmds: [][]string{{"go", "env", "GOMODCACHE"}}, Env: "GOMODCACHE",
			Default: func(string) string {
				if gp := os.Getenv("GOPATH"); gp != "" {
					return filepath.Join(filepath.SplitList(gp)[0], "pkg", "mod")
				}
				return underHome("go", "pkg", "mod")("")
			}},
		{Group: "npm", Label: "_cacache", Exes: []string{"npm"}, PathCmds: [][]string{{"npm", "config", "get", "cache"}}, Env: "npm_config_cache", Default: underHome(".npm"), Sub: "_cacache"},
		{Group: "npm", Label: "_npx", Exes: []string{"npm"}, PathCmds: [][]string{{"npm", "config", "get", "cache"}}, Env: "npm_config_cache", Default: underHome(".npm"), Sub: "_npx"},
		{Group: "pnpm", Label: "store", Exes: []string{"pnpm"}, PathCmds: [][]string{{"pnpm", "store", "path"}},
			Default: func(goos string) string { return dataHome(goos, "pnpm", "store") }},
		{Group: "bun", Label: "install cache", Exes: []string{"bun"}, PathCmds: [][]string{{"bun", "pm", "cache"}}, CmdOptional: true, Env: "BUN_INSTALL_CACHE_DIR",
			Default: func(string) string {
				if b := os.Getenv("BUN_INSTALL"); b != "" {
					return filepath.Join(b, "install", "cache")
				}
				return underHome(".bun", "install", "cache")("")
			}},
		{Group: "pip", Label: "cache", Exes: []string{"pip3", "pip"}, PathCmds: [][]string{{"pip3", "cache", "dir"}, {"pip", "cache", "dir"}}, Env: "PIP_CACHE_DIR", Default: under("pip")},
		{Group: "uv", Label: "cache", Exes: []string{"uv"}, PathCmds: [][]string{{"uv", "cache", "dir"}}, Env: "UV_CACHE_DIR", Default: underHome(".cache", "uv")},
		{Group: "Homebrew", Label: "downloads", Exes: []string{"brew"}, PathCmds: [][]string{{"brew", "--cache"}}, Env: "HOMEBREW_CACHE", Default: under("Homebrew")},
		{Group: "Rust", Label: "cargo registry", Exes: []string{"cargo"}, Env: "CARGO_HOME", Default: underHome(".cargo"), Sub: "registry"},
		{Group: "Rust", Label: "sccache", Exes: []string{"sccache"}, Env: "SCCACHE_DIR",
			Default: func(goos string) string {
				c := userCache(goos)
				if c == "" {
					return ""
				}
				if goos == "darwin" {
					return filepath.Join(c, "Mozilla.sccache")
				}
				return filepath.Join(c, "sccache")
			}},
		{Group: "Rust", Label: "toolchains", Kind: KindVersion, Exes: []string{"rustup"}, Special: "rustup"},
		{Group: "Docker", Label: "images", Exes: []string{"docker"}, Special: "docker"},
		{Group: "Docker", Label: "build cache", Exes: []string{"docker"}, Special: "docker"},
	}
	if demoHang {
		ps = append(ps, Place{Group: "Demo", Label: "hung tool", Exes: []string{"sh"}, PathCmds: [][]string{{"sh", "-c", "sleep 600"}}})
	}
	return ps
}

var hostOS = runtime.GOOS
