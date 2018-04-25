.PHONY: all
all: build

.PHONY: build
build:
	docker build -t tag-monger:latest .

.PHONY: run
run:
	 docker run -ti \
		 -e AWS_DEFAULT_REGION=$(AWS_DEFAULT_REGION) \
		 -e AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) \
		 -e AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) \
		 tag-monger
