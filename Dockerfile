FROM czcorpus/kontext-manatee:2.208-jammy

RUN apt-get update && apt-get install wget tar python3-dev curl git -y \
    && wget https://go.dev/dl/go1.18.3.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.18.3.linux-amd64.tar.gz

COPY . /opt/masm
WORKDIR /opt/masm

RUN git config --global --add safe.directory /opt/masm \
    && export PATH=$PATH:/usr/local/go/bin \
    && python3 build3 2.208

EXPOSE 8088
CMD ["./masm3", "start", "conf-docker.json"]