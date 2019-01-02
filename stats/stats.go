package main

import (
	"bytes"
	"flag"
	"html/template"
	"log"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/ricochet2200/go-disk-usage/du"
	sendgrid "github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"

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

	if !timestampOlderThan(minimalDuration) {
		log.Printf("Timestamp is not old enough, exiting")
		return
	}

	var msgBuf bytes.Buffer
	if err := tmpl.Execute(&msgBuf, struct {
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
	}); err != nil {
		panic(err)
	}

	log.Printf("Notifying via sendgrid")
	if err := notifySendgrid(msgBuf.String()); err != nil {
		panic(err)
	}
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

var config struct {
	MonitoredPartitions      []string
	MinimalFreeSpaceFraction float32
	HeartbeatHours           uint32
	AlertHours               uint32

	secret struct {
		Sonarr struct {
			ApiKey string
		}
		SendGrid struct {
			ApiKey string
			To     struct {
				Name  string
				Email string
			}
			From struct {
				Name  string
				Email string
			}
		}
	}
}

func notifySendgrid(msg string) error {
	from := mail.NewEmail(
		config.secret.SendGrid.From.Name,
		config.secret.SendGrid.From.Email,
	)
	to := mail.NewEmail(
		config.secret.SendGrid.To.Name,
		config.secret.SendGrid.To.Email,
	)
	subject := "Update from wamc/stats"
	plainTextContent := msg
	htmlContent := "<pre>" + plainTextContent + "</pre>"
	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
	client := sendgrid.NewSendClient(config.secret.SendGrid.ApiKey)
	response, err := client.Send(message)
	if err != nil {
		log.Printf("Response details from sendgrid:\n%s\n%s\n%s",
			response.StatusCode,
			response.Body,
			response.Headers)
		return err
	}
	return nil
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
