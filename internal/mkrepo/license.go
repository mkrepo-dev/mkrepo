package mkrepo

import (
	"io"
	"io/fs"
	"log/slog"
	"regexp"
	"strings"
	"text/template"
)

type License struct {
	Title    string
	Filename string
	With     []string
	Vars     []string
	Template *template.Template
}

type LicenseContext struct {
	Year    *int
	Owner   *string
	Project *string
}

type Licenses map[string]License

var (
	reFindHeader = regexp.MustCompile(`{{-\s*/\*\s*(.+?):\s*(.+?)\s*\*/\s*-}}`)
	reFindVars   = regexp.MustCompile(`{{\.(\w+)}}`)
)

// TODO: Take multiple filesystems so user can merge directory with buildin licenses
func PrepareLicenses(licensesFS fs.FS) (Licenses, error) {
	licenses := make(Licenses)
	entries, err := fs.ReadDir(licensesFS, ".")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		license := License{Filename: "LICENSE"}
		key := strings.TrimSuffix(entry.Name(), ".tmpl")

		f, err := licensesFS.Open(entry.Name())
		if err != nil {
			slog.Error("Error opening license file", "file", entry.Name(), "error", err)
			continue
		}
		defer f.Close()
		content, err := io.ReadAll(f)
		if err != nil {
			slog.Error("Error reading license file", "file", entry.Name(), "error", err)
			continue
		}

		for line := range strings.Lines(string(content)) {
			matches := reFindHeader.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 2 {
					switch match[1] {
					case "title":
						license.Title = match[2]
					case "filename":
						license.Filename = match[2]
					case "with":
						license.With = append(license.With, match[2])
					}
				}
			}
			matches = reFindVars.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 1 {
					license.Vars = append(license.Vars, match[1])
				}
			}
		}

		license.Template, err = template.ParseFS(licensesFS, entry.Name())
		if err != nil {
			slog.Error("Error parsing license template", "file", entry.Name(), "error", err)
			continue
		}

		licenses[key] = license
		slog.Debug("License prepared", "name", license.Title)
	}

	slog.Info("Licenses prepared", slog.Int("count", len(licenses)))

	return licenses, nil
}
