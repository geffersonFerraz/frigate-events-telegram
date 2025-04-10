docker-publish-gcr:
	docker build -t frigate-events-telegram .
	docker tag frigate-events-telegram ghcr.io/geffersonferraz/frigate-events-telegram:latest
	docker push ghcr.io/geffersonferraz/frigate-events-telegram:latest


docker-publish-dockerhub:
	docker build -t frigate-events-telegram .
	docker tag frigate-events-telegram geffersonferraz/frigate-events-telegram:latest
	docker push geffersonferraz/frigate-events-telegram:latest

	docker tag frigate-events-telegram geffersonferraz/frigate-events-telegram:$(shell git rev-parse --short HEAD)
	docker push geffersonferraz/frigate-events-telegram:$(shell git rev-parse --short HEAD)
	


