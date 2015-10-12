all:

TAG = 0.0
PREFIX = bprashanth/nginxhttps

container:
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	if [ -f *.key ]; then rm *.key; fi
	if [ -f *.crt ]; then rm *.crt; fi
	if [ -f *.json ]; then rm *.json; fi
