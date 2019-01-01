package main

import (
	"html/template"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/ricochet2200/go-disk-usage/du"

	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var tmpl = template.Must(template.New("").Parse(`
Hello from {{.Hostname}} running WAMC!

{{.Uptime}}

{{.Reboot}}

{{range .DiskSpace -}}
* {{.}}
{{end -}}
{{- if .NearlyFullPartitions}}

The following partitions are nearly full:
{{- range .NearlyFullPartitions}}
* {{.}}
{{end -}}
{{end -}}
{{if .SonarrIssues -}}

Sonarr issues: {{range .SonarrIssues}}
* {{.}}
{{end}}
{{else}}
Sonarr is healthy.
{{end}}
`))

func main() {
	os.Chdir(filepath.Dir(os.Args[0]))
	loadConfig()

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	tmpl.Execute(os.Stdout, struct {
		Hostname             string
		DiskSpace            []string
		NearlyFullPartitions []string
		Reboot               string
		SonarrIssues         []string
		Uptime               string
	}{
		DiskSpace:            fmtDiskSpacePartitions(),
		NearlyFullPartitions: nearlyFullPartitions(),
		Hostname:             hostname,
		Reboot:               fmtRebootRequired(),
		SonarrIssues:         fmtSonarrHealth(),
		Uptime:               fmtUptime(),
	})

	/*
		uptime, err := getUptime()
		if err != nil {
			panic(err)
		}
		fmt.Println(uptime)

		fmt.Println(availability("/"))
		fmt.Println(availability("/wamc"))

		fmt.Println()

		fmt.Println(rebootMessage())
	*/
}

func nearlyFullPartitions() []string {
	var result []string

	for _, path := range config.MonitoredPartitions {
		usage := du.NewDiskUsage(path)
		availFrac := 1 - usage.Usage()
		if availFrac < config.MinimalFreeSpaceFraction {
			result = append(result, path)
		}
	}

	return result
}

func fmtDiskSpacePartitions() []string {
	fmtDiskSpace := func(path string) string {
		usage := du.NewDiskUsage(path)
		avail := (humanize.Bytes(usage.Available()))
		return fmt.Sprintf("%s: %s Available (%.2f%%)",
			path,
			avail,
			100*(1-usage.Usage()))
	}

	var result []string
	for _, path := range config.MonitoredPartitions {
		result = append(result, fmtDiskSpace(path))
	}
	return result
}

type secretConfig struct {
	Sonarr struct {
		ApiKey string
	}
}

var config struct {
	MonitoredPartitions      []string
	MinimalFreeSpaceFraction float32

	secret secretConfig
}

func loadConfig() {
	decodeInto := func(filename string, v interface{}) {
		file, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		err = decoder.Decode(&v)
		if err != nil {
			panic(err)
		}
	}

	decodeInto("config.json", &config)
	decodeInto("secret.json", &config.secret)
}

func fmtUptime() string {
	out, err := exec.Command("uptime").Output()
	if err != nil {
		return fmt.Sprintf("ERROR: Failed to run uptime: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func fmtRebootRequired() string {
	since, err := rebootRequiredSince()
	if err != nil {
		return fmt.Sprintf("WARNING: Couldn't reason about reboot: %v", err)
	}
	if since == nil {
		return ""
	}
	return fmt.Sprintf("Reboot required since %s", humanize.Time(*since))
}

func rebootRequiredSince() (*time.Time, error) {
	s, err := os.Stat("/var/run/reboot-required")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	modTime := s.ModTime()

	return &modTime, nil
}
