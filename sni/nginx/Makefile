all:

TAG = 1.0
PREFIX = bprashanth/nginxrouter

container:
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	rm *.key
	rm *.crt
	rm *.json
