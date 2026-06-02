package service

import (
	"fmt"
	"io/fs"
	"text/template"

	"github.com/go-git/go-billy/v6"
	"github.com/kaptinlin/jsonschema"
	"gopkg.in/yaml.v3"
)

type LicenseKey string

type Licenses map[LicenseKey]License

type License struct {
	Name       string             `json:"name" yaml:"name"`
	File       string             `json:"file" yaml:"file"`
	TargetFile string             `json:"targetFile" yaml:"targetFile"`
	With       []LicenseKey       `json:"with,omitempty" yaml:"with,omitempty"`
	Schema     *jsonschema.Schema `json:"schema,omitempty" yaml:"schema,omitempty"`

	template *template.Template `json:"-" yaml:"-"`
}

type LicensesConfig struct {
	Licenses Licenses `yaml:"licenses"`
}

// AddLicense adds a license and all it's first class dependecies to filesystem.
func AddLicense(fs billy.Filesystem, licenseKey LicenseKey, licenses Licenses, values templateContext) error {
	license, ok := licenses[licenseKey]
	if !ok {
		return fmt.Errorf("license %s not found", licenseKey)
	}
	err := addLicense(fs, license, values)
	if err != nil {
		return err
	}

	for _, licenseKey := range license.With {
		license, ok := licenses[licenseKey]
		if !ok {
			return fmt.Errorf("license %s not found", licenseKey)
		}
		err := addLicense(fs, license, values)
		if err != nil {
			return err
		}
	}
	return nil
}

func addLicense(fs billy.Filesystem, license License, values any) error {
	return templateFile(fs, license.TargetFile, 0644, license.template, values)
}

func ParseLicensesFromBytes(configBytes []byte, licensesFS fs.FS) (Licenses, error) {
	var config LicensesConfig
	err := yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}
	return ParseLicenses(config, licensesFS)
}

func ParseLicenses(config LicensesConfig, licensesFS fs.FS) (Licenses, error) {
	licenses := make(Licenses)

	for key, licenseConfig := range config.Licenses {
		tmpl, err := template.ParseFS(licensesFS, licenseConfig.File)
		if err != nil {
			return nil, err
		}
		license := licenseConfig
		license.template = tmpl
		licenses[key] = license
	}

	return licenses, nil
}
