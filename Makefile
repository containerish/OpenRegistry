push: rm
	docker pull postgres:11
	docker tag postgres:11 0.0.0.0:5000/postgres:11
	docker push 0.0.0.0:5000/postgres:11
push-only:
	docker push 0.0.0.0:5000/postgres:11
pull: rm
	docker pull 0.0.0.0:5000/postgres:11

rm:
	docker images -q | xargs -t docker rmi -f 
