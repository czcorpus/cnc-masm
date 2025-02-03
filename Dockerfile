FROM czcorpus/kontext-manatee:2.225.8-noble

RUN apt-get update && apt-get install wget tar python3-dev python3-pip curl git bison libpcre2-dev -y \
    && wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz \
    && pip install pulp numpy --break-system-packages

WORKDIR /opt
RUN git clone https://github.com/czcorpus/manabuild \
    && cd manabuild \
    && export PATH=$PATH:/usr/local/go/bin \
    && make build && make install

WORKDIR /opt/masm
COPY . .
RUN git config --global --add safe.directory /opt/masm \
    && PATH=$PATH:/usr/local/go/bin:/root/go/bin \
    && ./configure --with-pcre2 \
    && make build

EXPOSE 8088
CMD ["./masm3", "start", "conf-docker.json"]