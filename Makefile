docker-publish-gcr:
	docker build -t frigate-events-telegram .
	docker tag frigate-events-telegram ghcr.io/geffersonferraz/frigate-events-telegram:latest
	docker push ghcr.io/geffersonferraz/frigate-events-telegram:latest


docker-publish-dockerhub:
	go build -o frigate-events-telegram -ldflags="-s -w"

	docker build -t frigate-events-telegram .
	docker tag frigate-events-telegram geffws/frigate-events-telegram:latest
	docker push geffws/frigate-events-telegram:latest

	docker tag frigate-events-telegram geffws/frigate-events-telegram:$(shell git rev-parse --short HEAD)
	docker push geffws/frigate-events-telegram:$(shell git rev-parse --short HEAD)



