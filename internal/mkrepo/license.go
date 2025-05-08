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
	Year     *int
	Fullname *string
	Project  *string
}

type Licenses map[string]License

var (
	reFindHeader = regexp.MustCompile(`{{-\s*/\*\s*(.+?):\s*(.+?)\s*\*/\s*-}}`)
	reFindVars   = regexp.MustCompile(`{{\.(\w+)}}`)
)

// TODO: Take multiple filesystems so user can merge directory with buildin licenses
func PrepareLicenses(licensesFS fs.FS) (Licenses, error) {
	licenses := make(Licenses)
	err := fs.WalkDir(licensesFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		license := License{Filename: "LICENSE"}
		key := strings.TrimSuffix(d.Name(), ".txt.tmpl")

		f, err := licensesFS.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		content, err := io.ReadAll(f)
		if err != nil {
			return err
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

		license.Template, err = template.ParseFS(licensesFS, path)
		if err != nil {
			return err
		}

		licenses[key] = license
		slog.Debug("License prepared", "name", license.Title)
		return nil
	})
	if err != nil {
		return nil, err
	}

	slog.Info("Licenses prepared", slog.Int("count", len(licenses)))

	return licenses, nil
}
