package main

import (
	"flag"
	"html/template"
	"log"
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

var (
	heartbeatFile = flag.String("heartbeat_file", "/tmp/wamcstats", "Heartbeat file to indicate last run")
)

func main() {
	flag.Parse()

	os.Chdir(filepath.Dir(os.Args[0]))
	loadConfig()

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	minimalDuration := time.Duration(config.HeartbeatHours) * time.Hour

	if fmtRebootRequired() != "" || len(nearlyFullPartitions()) > 0 {
		minimalDuration = time.Duration(config.AlertHours) * time.Hour
		log.Printf("Alert condition, reducing notification duration")
	}

	log.Printf("Notifying if timestamp older than %s", minimalDuration)

	shouldNotify := false

	if timestampOlderThan(minimalDuration) {
		shouldNotify = true
	}

	log.Print(shouldNotify)

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
}

func timestampOlderThan(d time.Duration) bool {
	stat, err := os.Stat(*heartbeatFile)
	if os.IsNotExist(err) {
		f, err := os.Create(*heartbeatFile)
		if err != nil {
			panic(err)
		}
		f.Close()
		return true
	}

	if time.Now().Sub(stat.ModTime()) < d {
		return false
	}

	if err := os.Chtimes(*heartbeatFile, time.Now(), time.Now()); err != nil {
		panic(err)
	}
	return true
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
	HeartbeatHours           uint32
	AlertHours               uint32

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
