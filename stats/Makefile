all: stats

stats: stats.go
	docker run -it --rm -v $(PWD):/wamc golang:1.11 \
		bash -c "cd /wamc; go build"
