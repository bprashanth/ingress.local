all:

TAG = 1.0
PREFIX = bprashanth/nginxrouter
KEY = nginx.key
CERT = nginx.crt
HOST = nginxsvc

keys:
	# The CName used here is specific to the service specified in nginx-app.yaml.
	openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout $(KEY) -out $(CERT) -subj "/CN=$(HOST)/O=$(HOST)"

secret:
	CGO_ENABLED=0 GOOS=linux go run -a -installsuffix cgo -ldflags '-w' make_secret.go -crt $(CERT) -key $(KEY) -name $(HOST) > $(HOST).json

container:
	docker build -t $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	rm *.key
	rm *.crt
	rm *.json
