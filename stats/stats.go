package main

import (
	"html/template"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/ricochet2200/go-disk-usage/du"

	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
)

var tmpl = template.Must(template.New("").Parse(`
Hello from {{.Hostname}} running WAMC!

{{.Uptime}}

{{.Reboot}}

DISK SPACE REPORT

DISK SPACE WARNING

{{if .SonarrIssues -}}
Sonarr issues: {{range .SonarrIssues}}
* {{.}}
{{end}}
{{else -}}
Sonarr is healthy.
{{end}}
`))

func main() {
	loadConfig()

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	tmpl.Execute(os.Stdout, struct {
		Hostname     string
		Reboot       string
		SonarrIssues []string
		Uptime       string
	}{
		Hostname:     hostname,
		Reboot:       fmtRebootRequired(),
		SonarrIssues: fmtSonarrHealth(),
		Uptime:       fmtUptime(),
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

func availability(path string) string {
	usage := du.NewDiskUsage(path)
	avail := (humanize.Bytes(usage.Available()))
	return fmt.Sprintf("%s: %s Available (%%%.2f)",
		path,
		avail,
		100*(1-usage.Usage()))
}

type secretConfig struct {
	Sonarr struct {
		ApiKey string
	}
}

var config struct {
	secret secretConfig
}

func loadConfig() {
	file, err := os.Open("secret.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config.secret)
	if err != nil {
		panic(err)
	}
}

func fmtUptime() string {
	out, err := exec.Command("uptime").Output()
	if err != nil {
		return fmt.Sprintf("ERROR: Failed to run uptime: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func fmtSonarrHealth() []string {
	issues, err := getSonarrHealth()
	if err != nil {
		return []string{fmt.Sprintf("Error getting Sonarr health: %v", err)}
	}

	var messages []string

	for _, issue := range issues {
		messages = append(messages, issue.Message)
	}

	return messages
}

type sonarrHealthIssue struct {
	Type    string `json:type`
	Message string `json:message`
	WikiURL string `json:wikiUrl`
}

func getSonarrHealth() ([]sonarrHealthIssue, error) {
	url, err := url.Parse("http://localhost/sonarr/api/health")
	if err != nil {
		panic(err)
	}
	q := url.Query()
	q.Set("apikey", config.secret.Sonarr.ApiKey)
	url.RawQuery = q.Encode()

	resp, err := http.Get(url.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch %v: %v", url, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("Fetching %v: %s", url, resp.Status)
	}

	defer resp.Body.Close()

	var healthItems []sonarrHealthIssue
	json.NewDecoder(resp.Body).Decode(&healthItems)

	return healthItems, nil
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
