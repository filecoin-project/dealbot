package webutil

import (
	"errors"
	"path"

	"github.com/evanw/esbuild/pkg/api"
)

// Compile executes esbuild to bundle client-side app logic
func Compile(rootPath string, minify bool) (string, error) {
	opts := api.BuildOptions{
		EntryPoints: []string{path.Join(rootPath, "index.js")},
		Outfile:     "script.js",
		Bundle:      true,
		Write:       false,
		LogLevel:    api.LogLevelInfo,
	}
	if minify {
		opts.MinifyWhitespace = true
		opts.MinifyIdentifiers = false
		opts.MinifySyntax = true
	}
	res := api.Build(opts)
	if len(res.Errors) > 0 {
		return "", errors.New("failed to compile web js")
	}
	return string(res.OutputFiles[0].Contents), nil
}
