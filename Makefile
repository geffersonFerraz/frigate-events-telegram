docker-publish:
	docker build -t frigate-events-telegram .
	docker tag frigate-events-telegram ghcr.io/geffersonferraz/frigate-events-telegram:latest
	docker push ghcr.io/geffersonferraz/frigate-events-telegram:latest


