all: stats

SOURCES=sonarr.go stats.go
MODCONFIG=go.mod go.sum

stats: $(SOURCES) $(MODCONFIG)
	docker run -it --rm -v $(PWD):/wamc golang:1.11 \
		bash -c "cd /wamc; go build"
