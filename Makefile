docker-publish-gcr:
	docker build -t frigate-events-telegram .
	docker tag frigate-events-telegram ghcr.io/geffersonferraz/frigate-events-telegram:latest
	docker push ghcr.io/geffersonferraz/frigate-events-telegram:latest


VERSION=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "latest")

docker-publish-dockerhub:
	docker build -t frigate-events-telegram .

	docker tag frigate-events-telegram geffws/frigate-events-telegram:$(VERSION)
	docker push geffws/frigate-events-telegram:$(VERSION)



