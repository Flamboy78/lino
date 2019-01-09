FROM golang:1.10

RUN apt-get update && \
        apt-get -y install wget curl sudo make && \
        apt-get install -y git && \
        apt-get install -y jq && \
        apt install -y python3 \
        python3-pip \
        python3-setuptools && \
        pip3 install --upgrade pip && \
        apt-get clean

# get lino core code
RUN pip install awscli --upgrade --user
RUN mkdir -p src/github.com/lino-network
WORKDIR src/github.com/lino-network
RUN git clone https://github.com/lino-network/lino.git
WORKDIR lino
RUN git checkout origin/zl/recorder
COPY vendor vendor

# replace customize file
WORKDIR cmd/lino
RUN go build

COPY recorder/watch_dog.sh watch_dog.sh
COPY recorder/genesis.json genesis.json
COPY recorder/config.toml config.toml

EXPOSE 26656
EXPOSE 26657


ENTRYPOINT ["./lino", "start", "--log_level=error"]